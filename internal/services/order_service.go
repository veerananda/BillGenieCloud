package services

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"restaurant-api/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type OrderService struct {
	db *gorm.DB
}

type CreateOrderRequest struct {
	RestaurantID string                   `json:"restaurant_id"` // Set by handler from JWT
	TableNumber  string                   `json:"table_number"`
	TableID      *string                  `json:"table_id"` // Link to RestaurantTable for dine-in orders
	CustomerName  string                   `json:"customer_name"`
	CustomerPhone string                   `json:"customer_phone"`
	OrderType     string                   `json:"order_type"`   // dine_in | counter
	ServiceMode  string                   `json:"service_mode"` // eat_here | takeaway (counter only)
	Items        []CreateOrderItemRequest `json:"items" validate:"required,min=1"`
	Notes        string                   `json:"notes"`
}

type CreateOrderItemRequest struct {
	MenuItemID string `json:"menu_item_id" validate:"required"`
	Quantity   int    `json:"quantity" validate:"required,min=1"`
	Notes      string `json:"notes"`
}

type OrderResponse struct {
	ID          string              `json:"id"`
	TableNumber string              `json:"table_number"`
	OrderNumber int                 `json:"order_number"`
	Status      string              `json:"status"`
	SubTotal    float64             `json:"sub_total"`
	TaxAmount   float64             `json:"tax_amount"`
	Total       float64             `json:"total"`
	Items       []OrderItemResponse `json:"items"`
	CreatedAt   string              `json:"created_at"`
}

type OrderItemResponse struct {
	ID       string  `json:"id"`
	MenuName string  `json:"menu_name"`
	Quantity int     `json:"quantity"`
	UnitRate float64 `json:"unit_rate"`
	Total    float64 `json:"total"`
	Status   string  `json:"status"`
}

// NewOrderService creates a new order service
func NewOrderService(db *gorm.DB) *OrderService {
	return &OrderService{db: db}
}

// GetDB returns the database instance (for testing purposes)
func (s *OrderService) GetDB() *gorm.DB {
	return s.db
}

// CreateOrder creates a new order and deducts inventory
func (s *OrderService) CreateOrder(restaurantID string, userID string, req CreateOrderRequest) (*models.Order, error) {
	// Validate items exist (but don't require inventory to be set up)
	for _, item := range req.Items {
		var menuItem models.MenuItem
		if err := s.db.Where("restaurant_id = ? AND id = ?", restaurantID, item.MenuItemID).
			First(&menuItem).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil, errors.New("menu item not found")
			}
			return nil, err
		}
	}

	// Start transaction
	log.Printf("🔵 [CreateOrder] Starting transaction for restaurant: %s", restaurantID)
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("❌ [CreateOrder] Transaction panic, rolling back: %v", r)
			tx.Rollback()
			panic(r)
		}
	}()

	// Determine order type and allocate numbers
	orderType := inferOrderType(req)
	todayStart := StartOfRestaurantDay(time.Now())

	var orderNumber int
	var ticketNumber int
	tableNumber := req.TableNumber
	var tableID *string = req.TableID

	if orderType == "counter" {
		maxTicket, err := getMaxCounterTicketToday(tx, restaurantID, todayStart)
		if err != nil {
			log.Printf("❌ [CreateOrder] Failed to allocate counter ticket: %v", err)
			tx.Rollback()
			return nil, err
		}
		ticketNumber = maxTicket + 1
		orderNumber = ticketNumber
		tableNumber = strconv.Itoa(ticketNumber)
		tableID = nil // counter orders are not tied to restaurant tables
	} else {
		if strings.TrimSpace(tableNumber) == "" {
			tx.Rollback()
			return nil, errors.New("table_number is required for dine-in orders")
		}
		var lastOrder models.Order
		tx.Where("restaurant_id = ?", restaurantID).
			Order("order_number DESC").
			Limit(1).
			First(&lastOrder)

		orderNumber = 1
		if lastOrder.ID != "" {
			orderNumber = lastOrder.OrderNumber + 1
		}
		ticketNumber = 0
	}

	customerName := strings.TrimSpace(req.CustomerName)
	if orderType == "counter" && customerName == "" {
		if req.ServiceMode == "takeaway" {
			customerName = "Takeaway"
		} else {
			customerName = "Counter"
		}
	}

	// Create order with explicit UUID generation (don't rely on BeforeCreate hook in transaction)
	orderID := uuid.New().String()
	order := &models.Order{
		ID:              orderID,
		RestaurantID:    restaurantID,
		TableNumber:     tableNumber,
		TableID:         tableID,
		CustomerName:    customerName,
		CustomerPhone:   strings.TrimSpace(req.CustomerPhone),
		OrderNumber:     orderNumber,
		OrderType:       orderType,
		TicketNumber:    ticketNumber,
		ServiceMode:     req.ServiceMode,
		Status:          "pending",
		SubTotal:        0,
		TaxAmount:       0,
		Total:           0,
		CreatedByUserID: userID,
		Notes:           req.Notes,
	}

	log.Printf("🔵 [CreateOrder] Generated UUID: %s", orderID)
	tableIDStr := ""
	if req.TableID != nil {
		tableIDStr = *req.TableID
	}
	log.Printf("🔵 [CreateOrder] Request TableID: %s (nil: %v)", tableIDStr, req.TableID == nil)
	log.Printf("🔵 [CreateOrder] Creating order for table %s with order_number: %d, restaurant_id: %s", req.TableNumber, orderNumber, restaurantID)
	log.Printf("🔵 [CreateOrder] Order object before Create: ID=%s, RestaurantID=%s, TableNumber=%s, TableID=%v", order.ID, order.RestaurantID, order.TableNumber, order.TableID)

	createResult := tx.Create(order)
	log.Printf("🔵 [CreateOrder] Create() returned. RowsAffected: %d, Error: %v", createResult.RowsAffected, createResult.Error)

	if createResult.Error != nil {
		log.Printf("❌ [CreateOrder] Failed to create order: %v", createResult.Error)
		tx.Rollback()
		return nil, createResult.Error
	}

	log.Printf("✅ [CreateOrder] Order created with ID: %s (RowsAffected: %d)", order.ID, createResult.RowsAffected)

	// Double-check the order was actually created in transaction
	var verifyOrder models.Order
	if err := tx.Where("id = ?", order.ID).First(&verifyOrder).Error; err != nil {
		log.Printf("⚠️  [CreateOrder] WARNING: Order not found in transaction immediately after Create! Error: %v", err)
		log.Printf("⚠️  [CreateOrder] This suggests Create() might not have actually saved the order")
		tx.Rollback()
		return nil, fmt.Errorf("order creation verification failed: %v", err)
	}
	log.Printf("✅ [CreateOrder] Order verified in transaction: ID=%s, Status=%s", verifyOrder.ID, verifyOrder.Status)

	// Create order items (inventory deduction is now optional)
	subTotal := 0.0
	log.Printf("🔵 [CreateOrder] Processing %d items for order #%d", len(req.Items), orderNumber)
	for i, itemReq := range req.Items {
		var menuItem models.MenuItem
		if err := tx.Where("id = ?", itemReq.MenuItemID).First(&menuItem).Error; err != nil {
			log.Printf("❌ [CreateOrder] Item %d: Menu item not found for ID: %s, error: %v", i+1, itemReq.MenuItemID, err)
			tx.Rollback()
			return nil, err
		}

		// Create order item with explicit UUID generation
		itemID := uuid.New().String()
		orderItem := &models.OrderItem{
			ID:       itemID,
			OrderID:  order.ID,
			MenuID:   menuItem.ID,
			Quantity: itemReq.Quantity,
			UnitRate: menuItem.Price,
			Total:    menuItem.Price * float64(itemReq.Quantity),
			Status:   "pending",
			Notes:    itemReq.Notes,
		}

		log.Printf("🔵 [CreateOrder] Item %d: Creating OrderItem - ID: %s, MenuID: %s, Qty: %d, Total: ₹%.2f", i+1, itemID, menuItem.ID, itemReq.Quantity, orderItem.Total)
		if err := tx.Create(orderItem).Error; err != nil {
			log.Printf("❌ [CreateOrder] Item %d: Failed to create order item: %v", i+1, err)
			tx.Rollback()
			return nil, err
		}
		log.Printf("✅ [CreateOrder] Item %d: Created with ID: %s", i+1, orderItem.ID)

		subTotal += orderItem.Total

		// Attempt to deduct inventory if it exists
		var inventory models.Inventory
		if err := tx.Where("restaurant_id = ? AND menu_item_id = ?", restaurantID, menuItem.ID).
			First(&inventory).Error; err == nil {
			// Inventory record exists, deduct it
			if inventory.Quantity >= float64(itemReq.Quantity) {
				log.Printf("🔵 [CreateOrder] Item %d: Deducting inventory - Current: %.0f, Deduct: %d", i+1, inventory.Quantity, itemReq.Quantity)
				if err := tx.Model(&models.Inventory{}).
					Where("restaurant_id = ? AND menu_item_id = ?", restaurantID, menuItem.ID).
					Update("quantity", gorm.Expr("quantity - ?", itemReq.Quantity)).Error; err != nil {
					log.Printf("❌ [CreateOrder] Item %d: Failed to deduct inventory: %v", i+1, err)
					tx.Rollback()
					return nil, err
				}
				log.Printf("✅ [CreateOrder] Inventory deducted: %s - %d units from stock", menuItem.Name, itemReq.Quantity)
			}
		} else {
			log.Printf("⚠️  [CreateOrder] Item %d: No inventory record found for %s, skipping deduction", i+1, menuItem.ID)
		}
		// If inventory doesn't exist or has insufficient quantity, we skip deduction (order still created)
	}

	// Calculate tax (assume 5% GST)
	taxAmount := subTotal * 0.05
	total := subTotal + taxAmount

	// Update order totals
	log.Printf("🔵 [CreateOrder] Updating order totals - SubTotal: ₹%.2f, Tax: ₹%.2f, Total: ₹%.2f", subTotal, taxAmount, total)
	if err := tx.Model(order).Updates(map[string]interface{}{
		"sub_total":  subTotal,
		"tax_amount": taxAmount,
		"total":      total,
	}).Error; err != nil {
		log.Printf("❌ [CreateOrder] Failed to update order totals: %v", err)
		tx.Rollback()
		return nil, err
	}

	// Commit transaction
	log.Printf("🔵 [CreateOrder] Committing transaction for order #%d", orderNumber)
	if err := tx.Commit().Error; err != nil {
		log.Printf("❌ [CreateOrder] Transaction commit failed: %v", err)
		return nil, err
	}
	log.Printf("✅ [CreateOrder] Transaction committed successfully for order #%d with ID: %s", orderNumber, order.ID)

	// Reload with items and totals for API response and WebSocket broadcast
	if err := s.db.Preload("Items").
		Preload("Items.MenuItem").
		Where("id = ? AND restaurant_id = ?", order.ID, restaurantID).
		First(order).Error; err != nil {
		log.Printf("❌ [CreateOrder] Failed to reload order after create: %v", err)
		return nil, err
	}

	log.Printf("✅ [CreateOrder] Order created successfully: Order #%d, ID: %s, Total: ₹%.2f, Items: %d",
		order.OrderNumber, order.ID, order.Total, len(order.Items))

	return order, nil
}

// UpdateOrder adds items to an existing order
func (s *OrderService) UpdateOrder(restaurantID string, orderID string, req CreateOrderRequest) (*models.Order, error) {
	// Validate items exist
	for _, item := range req.Items {
		var menuItem models.MenuItem
		if err := s.db.Where("restaurant_id = ? AND id = ?", restaurantID, item.MenuItemID).
			First(&menuItem).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil, errors.New("menu item not found")
			}
			return nil, err
		}
	}

	// Start transaction
	log.Printf("🔵 [UpdateOrder] Starting transaction for order: %s", orderID)
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("❌ [UpdateOrder] Transaction panic, rolling back: %v", r)
			tx.Rollback()
			panic(r)
		}
	}()

	// Get existing order
	var order models.Order
	if err := tx.Where("id = ? AND restaurant_id = ?", orderID, restaurantID).
		Preload("Items").
		Preload("Items.MenuItem").
		First(&order).Error; err != nil {
		tx.Rollback()
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("order not found")
		}
		return nil, err
	}

	log.Printf("🔵 [UpdateOrder] Found existing order #%d with %d items", order.OrderNumber, len(order.Items))

	// Add new items to the order
	var totalAdded float64 = 0
	for _, itemReq := range req.Items {
		var menuItem models.MenuItem
		if err := tx.Where("restaurant_id = ? AND id = ?", restaurantID, itemReq.MenuItemID).
			First(&menuItem).Error; err != nil {
			tx.Rollback()
			if err == gorm.ErrRecordNotFound {
				return nil, errors.New("menu item not found")
			}
			return nil, err
		}

		// Create order item
		itemID := uuid.New().String()
		orderItem := models.OrderItem{
			ID:       itemID,
			OrderID:  orderID,
			MenuID:   menuItem.ID,
			Quantity: itemReq.Quantity,
			UnitRate: menuItem.Price,
			Total:    menuItem.Price * float64(itemReq.Quantity),
			Status:   "pending",
			Notes:    itemReq.Notes,
		}

		if err := tx.Create(&orderItem).Error; err != nil {
			tx.Rollback()
			log.Printf("❌ [UpdateOrder] Failed to create order item: %v", err)
			return nil, err
		}

		totalAdded += orderItem.Total
		log.Printf("🔵 [UpdateOrder] Added item: %s (qty: %d, total: ₹%.2f)", menuItem.Name, itemReq.Quantity, orderItem.Total)

		// Deduct inventory
		if err := tx.Model(&models.Inventory{}).
			Where("restaurant_id = ? AND menu_item_id = ?", restaurantID, menuItem.ID).
			Update("quantity", gorm.Expr("quantity - ?", itemReq.Quantity)).Error; err != nil {
			tx.Rollback()
			log.Printf("❌ [UpdateOrder] Inventory deduction failed: %v", err)
			return nil, err
		}
	}

	// Update order totals
	order.SubTotal += totalAdded
	order.Total = order.SubTotal + order.TaxAmount - order.DiscountAmount

	if err := tx.Model(&order).Updates(map[string]interface{}{
		"sub_total": order.SubTotal,
		"total":     order.Total,
	}).Error; err != nil {
		tx.Rollback()
		log.Printf("❌ [UpdateOrder] Failed to update order totals: %v", err)
		return nil, err
	}

	log.Printf("🔵 [UpdateOrder] Committing transaction for order #%d", order.OrderNumber)
	if err := tx.Commit().Error; err != nil {
		log.Printf("❌ [UpdateOrder] Transaction commit failed: %v", err)
		return nil, err
	}

	// Reload order with new items
	if err := s.db.Where("id = ?", orderID).
		Preload("Items").
		Preload("Items.MenuItem").
		First(&order).Error; err != nil {
		log.Printf("❌ [UpdateOrder] Failed to reload order: %v", err)
		return nil, err
	}

	log.Printf("✅ [UpdateOrder] Order updated successfully: Order #%d, New Total: ₹%.2f", order.OrderNumber, order.Total)

	return &order, nil
}

func (s *OrderService) markAllOrderItemsServed(tx *gorm.DB, orderID string) error {
	return tx.Model(&models.OrderItem{}).
		Where("order_id = ? AND status <> ?", orderID, "served").
		Update("status", "served").Error
}

func (s *OrderService) reloadOrderWithItems(orderID, restaurantID string) (*models.Order, error) {
	var order models.Order
	if err := s.db.Preload("Items").
		Preload("Items.MenuItem").
		Where("id = ? AND restaurant_id = ?", orderID, restaurantID).
		First(&order).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("order not found")
		}
		return nil, err
	}
	return &order, nil
}

// CompleteOrder marks order as completed
func (s *OrderService) CompleteOrder(restaurantID string, orderID string) (*models.Order, error) {
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	var order models.Order
	if err := tx.Where("id = ? AND restaurant_id = ?", orderID, restaurantID).
		First(&order).Error; err != nil {
		tx.Rollback()
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("order not found")
		}
		return nil, err
	}

	if err := s.markAllOrderItemsServed(tx, orderID); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Model(&order).Update("status", "completed").Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	log.Printf("✅ Order completed: Order #%d (all items marked served)", order.OrderNumber)

	return s.reloadOrderWithItems(orderID, restaurantID)
}

// CompleteOrderWithPayment completes order with payment details
func (s *OrderService) CompleteOrderWithPayment(restaurantID string, orderID string, paymentMethod string, amountReceived float64, changeReturned float64) (*models.Order, error) {
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	var order models.Order
	if err := tx.Where("id = ? AND restaurant_id = ?", orderID, restaurantID).
		First(&order).Error; err != nil {
		tx.Rollback()
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("order not found")
		}
		return nil, err
	}

	log.Printf("🔵 [CompleteOrderWithPayment] BEFORE - Order #%d Status: %s, Total: %.2f", order.OrderNumber, order.Status, order.Total)

	isCounter := order.OrderType == "counter"

	if !isCounter {
		if err := s.markAllOrderItemsServed(tx, orderID); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	now := time.Now()
	updates := map[string]interface{}{
		"payment_method":  paymentMethod,
		"amount_received": amountReceived,
		"change_returned": changeReturned,
		"updated_at":      now,
	}

	if isCounter {
		// Paid at counter — keep pending so kitchen can prepare items
		updates["completed_at"] = now
		token, tokenErr := GenerateTrackingToken()
		if tokenErr != nil {
			tx.Rollback()
			return nil, tokenErr
		}
		expires := now.Add(trackingTTL)
		updates["tracking_token"] = token
		updates["tracking_expires_at"] = expires
	} else {
		updates["status"] = "completed"
		updates["completed_at"] = now
	}

	log.Printf("🔵 [CompleteOrderWithPayment] Updating order with: %+v", updates)

	if err := tx.Model(&order).Updates(updates).Error; err != nil {
		tx.Rollback()
		log.Printf("❌ [CompleteOrderWithPayment] Update failed: %v", err)
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		log.Printf("❌ [CompleteOrderWithPayment] Commit failed: %v", err)
		return nil, err
	}

	reloaded, err := s.reloadOrderWithItems(orderID, restaurantID)
	if err != nil {
		return nil, err
	}

	log.Printf("✅ [CompleteOrderWithPayment] AFTER - Order #%d Status: %s, PaymentMethod: %s, Received: %.2f, Change: %.2f",
		reloaded.OrderNumber, reloaded.Status, reloaded.PaymentMethod, reloaded.AmountReceived, reloaded.ChangeReturned)

	return reloaded, nil
}

// CancelOrder cancels order and restores inventory
func (s *OrderService) CancelOrder(restaurantID string, orderID string) error {
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	var order models.Order
	if err := tx.Where("id = ? AND restaurant_id = ?", orderID, restaurantID).
		First(&order).Error; err != nil {
		tx.Rollback()
		if err == gorm.ErrRecordNotFound {
			return errors.New("order not found")
		}
		return err
	}

	// Restore inventory for all items
	var items []models.OrderItem
	if err := tx.Where("order_id = ?", orderID).Find(&items).Error; err != nil {
		tx.Rollback()
		return err
	}

	for _, item := range items {
		if err := tx.Model(&models.Inventory{}).
			Where("restaurant_id = ? AND menu_item_id = ?", restaurantID, item.MenuID).
			Update("quantity", gorm.Expr("quantity + ?", item.Quantity)).Error; err != nil {
			tx.Rollback()
			return err
		}
		log.Printf("✅ Inventory restored: %s - %d units to stock", item.MenuID, item.Quantity)
	}

	// Update order status
	if err := tx.Model(&order).Update("status", "cancelled").Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	log.Printf("✅ Order cancelled: Order #%d, Inventory restored", order.OrderNumber)

	return nil
}

// GetOrderByID retrieves order with items
func (s *OrderService) GetOrderByID(restaurantID string, orderID string) (*models.Order, error) {
	var order models.Order
	if err := s.db.Preload("Items").
		Preload("Items.MenuItem").
		Where("id = ? AND restaurant_id = ?", orderID, restaurantID).
		First(&order).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("order not found")
		}
		return nil, err
	}

	return &order, nil
}

// ListOrders retrieves orders for a restaurant
func (s *OrderService) ListOrders(restaurantID string, status string, limit int, offset int) ([]models.Order, int64, error) {
	var orders []models.Order
	var count int64

	query := s.db.Where("restaurant_id = ?", restaurantID)

	if status != "" {
		// Support special status values
		if status == "active" {
			// Return pending and cooking orders (not completed)
			query = query.Where("status IN ?", []string{"pending", "cooking"})
			log.Printf("🔵 [ListOrders] Filtering for active orders (pending and cooking only)")
		} else {
			// Filter by exact status
			query = query.Where("status = ?", status)
			log.Printf("🔵 [ListOrders] Filtering by status: %s", status)
		}
	} else {
		log.Printf("🔵 [ListOrders] No status filter - getting ALL orders")
	}

	if err := query.Model(&models.Order{}).Count(&count).Error; err != nil {
		log.Printf("❌ [ListOrders] Count failed: %v", err)
		return nil, 0, err
	}

	log.Printf("📊 [ListOrders] Total orders found: %d (status=%s, limit=%d, offset=%d)", count, status, limit, offset)

	if err := query.Preload("Items").
		Preload("Items.MenuItem").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&orders).Error; err != nil {
		log.Printf("❌ [ListOrders] Query failed: %v", err)
		return nil, 0, err
	}

	log.Printf("✅ [ListOrders] Returned %d orders", len(orders))
	for i, order := range orders {
		log.Printf("   Order %d: ID=%s, #%d, Status=%s, Total=%.2f, PaymentMethod=%s",
			i+1, order.ID, order.OrderNumber, order.Status, order.Total, order.PaymentMethod)
	}

	return orders, count, nil
}

// OrderSummaryItem is a lightweight line item for list/summary views.
type OrderSummaryItem struct {
	ID       string  `json:"id"`
	MenuID   string  `json:"menu_id"`
	Quantity int     `json:"quantity"`
	UnitRate float64 `json:"unit_rate"`
	Status   string  `json:"status"`
	Name     string  `json:"name"`
	IsVeg    bool    `json:"is_vegetarian"`
}

// OrderSummary is a lightweight order payload without full menu metadata.
type OrderSummary struct {
	ID             string             `json:"id"`
	RestaurantID   string             `json:"restaurant_id"`
	TableNumber    string             `json:"table_number"`
	TableID        *string            `json:"table_id,omitempty"`
	CustomerName   string             `json:"customer_name"`
	OrderNumber    int                `json:"order_number"`
	OrderType      string             `json:"order_type,omitempty"`
	TicketNumber   int                `json:"ticket_number,omitempty"`
	ServiceMode    string             `json:"service_mode,omitempty"`
	Status         string             `json:"status"`
	SubTotal       float64            `json:"sub_total"`
	TaxAmount      float64            `json:"tax_amount"`
	DiscountAmount float64            `json:"discount_amount"`
	Total          float64            `json:"total"`
	CreatedAt      time.Time          `json:"created_at"`
	ItemCount      int                `json:"item_count"`
	ReadyCount     int                `json:"ready_count"`
	Items          []OrderSummaryItem `json:"items"`
}

// SalesSummary holds aggregated completed-order stats for a period.
type SalesSummary struct {
	TotalOrders       int64   `json:"total_orders"`
	TotalRevenue      float64 `json:"total_revenue"`
	AverageOrderValue float64 `json:"average_order_value"`
	Period            string  `json:"period"`
}

// ListOrdersSummary returns orders with minimal item fields (no heavy menu preload).
func (s *OrderService) ListOrdersSummary(restaurantID string, status string, limit int) ([]OrderSummary, int64, error) {
	var orders []models.Order
	var count int64

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	query := s.db.Where("restaurant_id = ?", restaurantID)
	query = applyDineInOnlyFilter(query)
	if status != "" {
		if status == "active" {
			query = query.Where("status IN ?", []string{"pending", "cooking"})
		} else {
			query = query.Where("status = ?", status)
		}
	}

	if err := query.Model(&models.Order{}).Count(&count).Error; err != nil {
		return nil, 0, err
	}

	if err := query.
		Preload("Items", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "order_id", "menu_id", "quantity", "unit_rate", "status", "total")
		}).
		Preload("Items.MenuItem", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "name", "is_veg")
		}).
		Order("created_at DESC").
		Limit(limit).
		Find(&orders).Error; err != nil {
		return nil, 0, err
	}

	summaries := make([]OrderSummary, 0, len(orders))
	for _, order := range orders {
		itemCount := 0
		readyCount := 0
		items := make([]OrderSummaryItem, 0, len(order.Items))
		for _, item := range order.Items {
			itemCount += item.Quantity
			if item.Status == "ready" {
				readyCount += item.Quantity
			}
			name := "Unknown Item"
			isVeg := false
			if item.MenuItem != nil {
				name = item.MenuItem.Name
				isVeg = item.MenuItem.IsVeg
			}
			items = append(items, OrderSummaryItem{
				ID:       item.ID,
				MenuID:   item.MenuID,
				Quantity: item.Quantity,
				UnitRate: item.UnitRate,
				Status:   item.Status,
				Name:     name,
				IsVeg:    isVeg,
			})
		}

		summaries = append(summaries, OrderSummary{
			ID:             order.ID,
			RestaurantID:   order.RestaurantID,
			TableNumber:    order.TableNumber,
			TableID:        order.TableID,
			CustomerName:   order.CustomerName,
			OrderNumber:    order.OrderNumber,
			OrderType:      order.OrderType,
			TicketNumber:   order.TicketNumber,
			ServiceMode:    order.ServiceMode,
			Status:         order.Status,
			SubTotal:       order.SubTotal,
			TaxAmount:      order.TaxAmount,
			DiscountAmount: order.DiscountAmount,
			Total:          order.Total,
			CreatedAt:      order.CreatedAt,
			ItemCount:      itemCount,
			ReadyCount:     readyCount,
			Items:          items,
		})
	}

	return summaries, count, nil
}

// GetSalesSummary aggregates completed orders for today or this month.
func (s *OrderService) GetSalesSummary(restaurantID string, period string) (*SalesSummary, error) {
	now := time.Now().In(RestaurantLocation())
	var start time.Time
	label := "today"
	switch period {
	case "month":
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		label = "month"
	default:
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	}

	var result struct {
		TotalOrders  int64
		TotalRevenue float64
	}

	err := s.db.Model(&models.Order{}).
		Where("restaurant_id = ?", restaurantID).
		Where("(status = ? OR (order_type = ? AND payment_method <> ''))", "completed", "counter").
		Where(historyActivityAtSQL+" >= ?", start).
		Select("COUNT(*) AS total_orders, COALESCE(SUM(total), 0) AS total_revenue").
		Scan(&result).Error
	if err != nil {
		return nil, err
	}

	avg := float64(0)
	if result.TotalOrders > 0 {
		avg = result.TotalRevenue / float64(result.TotalOrders)
	}

	return &SalesSummary{
		TotalOrders:       result.TotalOrders,
		TotalRevenue:      result.TotalRevenue,
		AverageOrderValue: avg,
		Period:            label,
	}, nil
}

// ListOrderHistory returns completed/paid orders within a date range for order history screens.
func (s *OrderService) ListOrderHistory(restaurantID string, from, toEnd time.Time, orderType string, limit, offset int) ([]models.Order, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	query := s.db.Model(&models.Order{}).
		Where("restaurant_id = ?", restaurantID).
		Where("(status = ? OR (order_type = ? AND payment_method <> ''))", "completed", "counter").
		Where(historyActivityAtSQL+" >= ? AND "+historyActivityAtSQL+" < ?", from, toEnd)

	switch orderType {
	case "counter":
		query = query.Where(isLegacyCounterOrderClause())
	case "dine_in":
		query = query.Where("NOT (" + isLegacyCounterOrderClause() + ")")
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	var orders []models.Order
	err := query.Preload("Items").
		Preload("Items.MenuItem").
		Order("COALESCE(completed_at, created_at) DESC").
		Limit(limit).
		Offset(offset).
		Find(&orders).Error
	if err != nil {
		return nil, 0, err
	}

	return orders, count, nil
}

// TryCompleteCounterOrderAfterKitchen marks a paid counter order completed once every item is ready/served.
func (s *OrderService) TryCompleteCounterOrderAfterKitchen(restaurantID, orderID string) (*models.Order, bool, error) {
	var order models.Order
	if err := s.db.Preload("Items").
		Where("id = ? AND restaurant_id = ?", orderID, restaurantID).
		First(&order).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, false, errors.New("order not found")
		}
		return nil, false, err
	}

	if order.OrderType != "counter" || order.Status == "completed" || order.PaymentMethod == "" {
		return &order, false, nil
	}

	for _, item := range order.Items {
		if item.Status != "ready" && item.Status != "served" {
			return &order, false, nil
		}
	}

	now := time.Now()
	if err := s.db.Model(&order).Updates(map[string]interface{}{
		"status":       "completed",
		"completed_at": now,
		"updated_at":   now,
	}).Error; err != nil {
		return nil, false, err
	}

	reloaded, err := s.reloadOrderWithItems(orderID, restaurantID)
	if err != nil {
		return nil, false, err
	}

	log.Printf("✅ Counter order #%d auto-completed after kitchen finished all items", reloaded.OrderNumber)
	return reloaded, true, nil
}

// UpdateOrderItemStatus updates the status of a specific order item
func (s *OrderService) UpdateOrderItemStatus(restaurantID string, orderID string, itemID string, status string) error {
	// First verify the order belongs to the restaurant
	var order models.Order
	if err := s.db.Where("id = ? AND restaurant_id = ?", orderID, restaurantID).
		First(&order).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New("order not found")
		}
		return err
	}

	// Update the order item status
	result := s.db.Model(&models.OrderItem{}).
		Where("id = ? AND order_id = ?", itemID, orderID).
		Update("status", status)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("order item not found")
	}

	log.Printf("✅ Order item status updated: Item %s -> %s", itemID, status)

	return nil
}

// UpdateOrderItemsByMenuID updates all items with a specific menu item ID to a new status
func (s *OrderService) UpdateOrderItemsByMenuID(restaurantID string, orderID string, menuItemID string, status string) error {
	// First verify the order belongs to the restaurant
	var order models.Order
	if err := s.db.Where("id = ? AND restaurant_id = ?", orderID, restaurantID).
		First(&order).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New("order not found")
		}
		return err
	}

	// Update all order items with this menu item ID to the new status
	result := s.db.Model(&models.OrderItem{}).
		Where("order_id = ? AND menu_id = ?", orderID, menuItemID).
		Update("status", status)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("no order items found with this menu item ID")
	}

	log.Printf("✅ Updated %d order items with menu_id %s -> status %s", result.RowsAffected, menuItemID, status)

	return nil
}

func inferOrderType(req CreateOrderRequest) string {
	switch req.OrderType {
	case "counter", "dine_in":
		return req.OrderType
	}
	if req.TableID != nil && strings.HasPrefix(*req.TableID, "self-service") {
		return "counter"
	}
	switch req.CustomerName {
	case "Self Service", "Takeaway", "Counter":
		return "counter"
	}
	if req.TableID != nil && *req.TableID != "" {
		return "dine_in"
	}
	return "dine_in"
}

func isLegacyCounterOrderClause() string {
	return `(order_type = 'counter' OR customer_name IN ('Self Service','Takeaway','Counter') OR (table_id IS NOT NULL AND table_id LIKE 'self-service-%'))`
}

func getMaxCounterTicketToday(tx *gorm.DB, restaurantID string, todayStart time.Time) (int, error) {
	var maxTicket int
	err := tx.Model(&models.Order{}).
		Where("restaurant_id = ? AND created_at >= ? AND "+isLegacyCounterOrderClause(), restaurantID, todayStart).
		Select(`COALESCE(MAX(CASE WHEN ticket_number > 0 THEN ticket_number ELSE order_number END), 0)`).
		Scan(&maxTicket).Error
	return maxTicket, err
}

// GetNextCounterTicket returns the next daily counter ticket number without creating an order.
func (s *OrderService) GetNextCounterTicket(restaurantID string) (int, error) {
	todayStart := StartOfRestaurantDay(time.Now())
	maxTicket, err := getMaxCounterTicketToday(s.db, restaurantID, todayStart)
	if err != nil {
		return 0, err
	}
	return maxTicket + 1, nil
}

// ListCounterOrdersToday returns counter/takeaway orders created today.
func (s *OrderService) ListCounterOrdersToday(restaurantID string, limit int) ([]models.Order, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	todayStart := StartOfRestaurantDay(time.Now())

	var orders []models.Order
	err := s.db.
		Where("restaurant_id = ? AND created_at >= ? AND "+isLegacyCounterOrderClause(), restaurantID, todayStart).
		Preload("Items").
		Preload("Items.MenuItem").
		Order("ticket_number DESC, order_number DESC, created_at DESC").
		Limit(limit).
		Find(&orders).Error
	return orders, err
}

func applyDineInOnlyFilter(query *gorm.DB) *gorm.DB {
	return query.Where(
		`(COALESCE(order_type, '') = '' OR order_type = 'dine_in')
		AND NOT (customer_name IN ('Self Service','Takeaway','Counter') OR (table_id IS NOT NULL AND table_id LIKE 'self-service-%'))`,
	)
}
