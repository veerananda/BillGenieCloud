package services

import (
	"errors"
	"fmt"
	"log"

	"restaurant-api/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type OrderService struct {
	db *gorm.DB
}

type CreateOrderRequest struct {
	RestaurantID string                   `json:"restaurant_id"` // Set by handler from JWT
	TableNumber  string                   `json:"table_number" validate:"required"`
	TableID      *string                  `json:"table_id"` // Link to RestaurantTable for dine-in orders
	CustomerName string                   `json:"customer_name"`
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

	// Get next order number
	var lastOrder models.Order
	tx.Where("restaurant_id = ?", restaurantID).
		Order("order_number DESC").
		Limit(1).
		First(&lastOrder)

	orderNumber := 1
	if lastOrder.ID != "" {
		orderNumber = lastOrder.OrderNumber + 1
	}

	// Create order with explicit UUID generation (don't rely on BeforeCreate hook in transaction)
	orderID := uuid.New().String()
	order := &models.Order{
		ID:              orderID,
		RestaurantID:    restaurantID,
		TableNumber:     req.TableNumber,
		TableID:         req.TableID,
		OrderNumber:     orderNumber,
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

	log.Printf("✅ [CreateOrder] Order created successfully: Order #%d, ID: %s, Total: ₹%.2f", order.OrderNumber, order.ID, total)

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

// CompleteOrder marks order as completed
func (s *OrderService) CompleteOrder(restaurantID string, orderID string) (*models.Order, error) {
	var order models.Order
	if err := s.db.Where("id = ? AND restaurant_id = ?", orderID, restaurantID).
		First(&order).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("order not found")
		}
		return nil, err
	}

	if err := s.db.Model(&order).Update("status", "completed").Error; err != nil {
		return nil, err
	}

	log.Printf("✅ Order completed: Order #%d", order.OrderNumber)

	return &order, nil
}

// CompleteOrderWithPayment completes order with payment details
func (s *OrderService) CompleteOrderWithPayment(restaurantID string, orderID string, paymentMethod string, amountReceived float64, changeReturned float64) (*models.Order, error) {
	var order models.Order
	if err := s.db.Where("id = ? AND restaurant_id = ?", orderID, restaurantID).
		First(&order).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("order not found")
		}
		return nil, err
	}

	log.Printf("🔵 [CompleteOrderWithPayment] BEFORE - Order #%d Status: %s, Total: %.2f", order.OrderNumber, order.Status, order.Total)

	// Update order with payment details and mark as completed
	updates := map[string]interface{}{
		"status":          "completed",
		"payment_method":  paymentMethod,
		"amount_received": amountReceived,
		"change_returned": changeReturned,
	}

	log.Printf("🔵 [CompleteOrderWithPayment] Updating order with: %+v", updates)

	if err := s.db.Model(&order).Updates(updates).Error; err != nil {
		log.Printf("❌ [CompleteOrderWithPayment] Update failed: %v", err)
		return nil, err
	}

	// Reload order to get updated data
	if err := s.db.Where("id = ?", orderID).First(&order).Error; err != nil {
		log.Printf("❌ [CompleteOrderWithPayment] Reload failed: %v", err)
		return nil, err
	}

	log.Printf("✅ [CompleteOrderWithPayment] AFTER - Order #%d Status: %s, PaymentMethod: %s, Received: %.2f, Change: %.2f",
		order.OrderNumber, order.Status, order.PaymentMethod, order.AmountReceived, order.ChangeReturned)

	return &order, nil
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
