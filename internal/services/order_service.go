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
	Items        []CreateOrderItemRequest `json:"items" validate:"omitempty,dive"`
	Notes        string                   `json:"notes"`
}

type CreateOrderItemRequest struct {
	MenuItemID string `json:"menu_item_id" validate:"required"`
	VariantID  string `json:"variant_id,omitempty"`
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
// ValidateCreateOrderRequest enforces item rules after struct validation.
func ValidateCreateOrderRequest(req CreateOrderRequest) error {
	orderType := inferOrderType(req)
	if len(req.Items) == 0 {
		if orderType == "counter" {
			return errors.New("at least one item is required for counter orders")
		}
		if req.TableID == nil || strings.TrimSpace(*req.TableID) == "" {
			return errors.New("table_id is required for dine-in orders without items")
		}
		return nil
	}
	for _, item := range req.Items {
		if strings.TrimSpace(item.MenuItemID) == "" {
			return errors.New("menu_item_id is required for each item")
		}
		if item.Quantity < 1 {
			return errors.New("item quantity must be at least 1")
		}
	}
	return nil
}

func (s *OrderService) CreateOrder(restaurantID string, userID string, req CreateOrderRequest) (*models.Order, []models.Ingredient, error) {
	if err := ValidateCreateOrderRequest(req); err != nil {
		return nil, nil, err
	}

	// Validate items exist (batch load — one query instead of N)
	menuItemIDs := uniqueMenuItemIDs(req.Items)
	menuItemsByID, err := loadMenuItemsMap(s.db, restaurantID, menuItemIDs)
	if err != nil {
		return nil, nil, err
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
		var err error
		ticketNumber, err = allocateCounterTicket(tx, restaurantID, todayStart)
		if err != nil {
			log.Printf("❌ [CreateOrder] Failed to allocate counter ticket: %v", err)
			tx.Rollback()
			return nil, nil, err
		}
		orderNumber = ticketNumber
		tableNumber = strconv.Itoa(ticketNumber)
		tableID = nil // counter orders are not tied to restaurant tables
	} else {
		if strings.TrimSpace(tableNumber) == "" {
			tx.Rollback()
			return nil, nil, errors.New("table_number is required for dine-in orders")
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
		return nil, nil, createResult.Error
	}

	log.Printf("✅ [CreateOrder] Order created with ID: %s (RowsAffected: %d)", order.ID, createResult.RowsAffected)

	// Create order items (inventory deduction is now optional)
	var taxableGross, nonTaxableGross float64
	batchSubID := uuid.New().String()
	log.Printf("🔵 [CreateOrder] KOT batch sub_id: %s", batchSubID)
	log.Printf("🔵 [CreateOrder] Processing %d items for order #%d", len(req.Items), orderNumber)

	inventoryByMenuID, err := loadInventoryByMenuMap(tx, restaurantID, menuItemIDs)
	if err != nil {
		log.Printf("❌ [CreateOrder] Failed to batch-load inventory: %v", err)
		tx.Rollback()
		return nil, nil, err
	}

	stockQuantities := make([]MenuItemQuantity, 0, len(req.Items))
	for i, itemReq := range req.Items {
		menuItem, ok := menuItemsByID[itemReq.MenuItemID]
		if !ok {
			log.Printf("❌ [CreateOrder] Item %d: Menu item not found for ID: %s", i+1, itemReq.MenuItemID)
			tx.Rollback()
			return nil, nil, errors.New("menu item not found")
		}

		// Create order item with explicit UUID generation
		itemID := uuid.New().String()
		unitPrice, variantLabel, recipeScale, variantIDPtr, err := ResolveOrderVariant(
			tx, restaurantID, menuItem.ID, itemReq.VariantID, menuItem.Price,
		)
		if err != nil {
			log.Printf("❌ [CreateOrder] Item %d: variant resolve failed: %v", i+1, err)
			tx.Rollback()
			return nil, nil, err
		}
		variantID := ""
		if variantIDPtr != nil {
			variantID = *variantIDPtr
		}
		stockQuantities = append(stockQuantities, MenuItemQuantity{
			MenuItemID:  menuItem.ID,
			Quantity:    itemReq.Quantity,
			RecipeScale: recipeScale,
			VariantID:   variantID,
		})
		orderItem := &models.OrderItem{
			ID:           itemID,
			OrderID:      order.ID,
			MenuID:       menuItem.ID,
			VariantID:    variantIDPtr,
			VariantLabel: variantLabel,
			Quantity:     itemReq.Quantity,
			UnitRate:     unitPrice,
			Total:        unitPrice * float64(itemReq.Quantity),
			Status:       InitialOrderItemStatus(menuItem),
			Notes:        itemReq.Notes,
			SubId:        batchSubID,
		}

		log.Printf("🔵 [CreateOrder] Item %d: Creating OrderItem - ID: %s, MenuID: %s, Qty: %d, Total: ₹%.2f", i+1, itemID, menuItem.ID, itemReq.Quantity, orderItem.Total)
		if err := tx.Create(orderItem).Error; err != nil {
			log.Printf("❌ [CreateOrder] Item %d: Failed to create order item: %v", i+1, err)
			tx.Rollback()
			return nil, nil, err
		}
		log.Printf("✅ [CreateOrder] Item %d: Created with ID: %s", i+1, orderItem.ID)

		if menuItem.IsTaxable {
			taxableGross += orderItem.Total
		} else {
			nonTaxableGross += orderItem.Total
		}

		// Attempt to deduct inventory if it exists
		if inventory, ok := inventoryByMenuID[menuItem.ID]; ok {
			if inventory.Quantity >= float64(itemReq.Quantity) {
				log.Printf("🔵 [CreateOrder] Item %d: Deducting inventory - Current: %.0f, Deduct: %d", i+1, inventory.Quantity, itemReq.Quantity)
				if err := tx.Model(&models.Inventory{}).
					Where("restaurant_id = ? AND menu_item_id = ?", restaurantID, menuItem.ID).
					Update("quantity", gorm.Expr("quantity - ?", itemReq.Quantity)).Error; err != nil {
					log.Printf("❌ [CreateOrder] Item %d: Failed to deduct inventory: %v", i+1, err)
					tx.Rollback()
					return nil, nil, err
				}
				log.Printf("✅ [CreateOrder] Inventory deducted: %s - %d units from stock", menuItem.Name, itemReq.Quantity)
			}
		} else {
			log.Printf("⚠️  [CreateOrder] Item %d: No inventory record found for %s, skipping deduction", i+1, menuItem.ID)
		}
		// If inventory doesn't exist or has insufficient quantity, we skip deduction (order still created)
	}

	// Calculate tax from restaurant GST setting
	var restaurant models.Restaurant
	if err := tx.Where("id = ?", restaurantID).First(&restaurant).Error; err != nil {
		log.Printf("❌ [CreateOrder] Restaurant not found: %v", err)
		tx.Rollback()
		return nil, nil, err
	}
	subTotal, taxAmount, total := CalculateRestaurantOrderTax(taxableGross, nonTaxableGross, 0, SettingsFromRestaurant(&restaurant))

	// Update order totals
	log.Printf("🔵 [CreateOrder] Updating order totals - SubTotal: ₹%.2f, Tax: ₹%.2f, Total: ₹%.2f (composite=%v, prices_include_gst=%v)", subTotal, taxAmount, total, restaurant.CompositeScheme, restaurant.PricesIncludeGST)
	if err := tx.Model(order).Updates(map[string]interface{}{
		"sub_total":  subTotal,
		"tax_amount": taxAmount,
		"total":      total,
	}).Error; err != nil {
		log.Printf("❌ [CreateOrder] Failed to update order totals: %v", err)
		tx.Rollback()
		return nil, nil, err
	}

	var updatedIngredients []models.Ingredient
	if len(stockQuantities) > 0 {
		var deductErr error
		updatedIngredients, deductErr = DeductIngredientsForMenuItems(tx, restaurantID, stockQuantities)
		if deductErr != nil {
			log.Printf("❌ [CreateOrder] Ingredient stock deduction failed: %v", deductErr)
			tx.Rollback()
			return nil, nil, deductErr
		}
	}

	// Commit transaction
	log.Printf("🔵 [CreateOrder] Committing transaction for order #%d", orderNumber)
	if err := tx.Commit().Error; err != nil {
		log.Printf("❌ [CreateOrder] Transaction commit failed: %v", err)
		return nil, nil, err
	}
	log.Printf("✅ [CreateOrder] Transaction committed successfully for order #%d with ID: %s", orderNumber, order.ID)

	// Reload with items and totals for API response and WebSocket broadcast
	if err := s.db.Preload("Items").
		Preload("Items.MenuItem").
		Where("id = ? AND restaurant_id = ?", order.ID, restaurantID).
		First(order).Error; err != nil {
		log.Printf("❌ [CreateOrder] Failed to reload order after create: %v", err)
		return nil, nil, err
	}

	log.Printf("✅ [CreateOrder] Order created successfully: Order #%d, ID: %s, Total: ₹%.2f, Items: %d",
		order.OrderNumber, order.ID, order.Total, len(order.Items))

	return order, updatedIngredients, nil
}

// UpdateOrder adds items to an existing order
func (s *OrderService) UpdateOrder(restaurantID string, orderID string, req CreateOrderRequest) (*models.Order, []models.Ingredient, error) {
	menuItemIDs := uniqueMenuItemIDs(req.Items)
	menuItemsByID, err := loadMenuItemsMap(s.db, restaurantID, menuItemIDs)
	if err != nil {
		return nil, nil, err
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
			return nil, nil, errors.New("order not found")
		}
		return nil, nil, err
	}

	log.Printf("🔵 [UpdateOrder] Found existing order #%d with %d items", order.OrderNumber, len(order.Items))

	if len(req.Items) == 0 {
		customerName := strings.TrimSpace(req.CustomerName)
		if customerName != order.CustomerName {
			if err := tx.Model(&order).Update("customer_name", customerName).Error; err != nil {
				tx.Rollback()
				return nil, nil, err
			}
			order.CustomerName = customerName
		}
	}

	// Add new items to the order (one KOT batch per update)
	var totalAdded float64 = 0
	batchSubID := uuid.New().String()
	stockQuantities := make([]MenuItemQuantity, 0, len(req.Items))
	log.Printf("🔵 [UpdateOrder] KOT batch sub_id: %s", batchSubID)
	for _, itemReq := range req.Items {
		menuItem, ok := menuItemsByID[itemReq.MenuItemID]
		if !ok {
			tx.Rollback()
			return nil, nil, errors.New("menu item not found")
		}

		itemID := uuid.New().String()
		unitPrice, variantLabel, recipeScale, variantIDPtr, err := ResolveOrderVariant(
			tx, restaurantID, menuItem.ID, itemReq.VariantID, menuItem.Price,
		)
		if err != nil {
			tx.Rollback()
			return nil, nil, err
		}
		variantID := ""
		if variantIDPtr != nil {
			variantID = *variantIDPtr
		}
		stockQuantities = append(stockQuantities, MenuItemQuantity{
			MenuItemID:  menuItem.ID,
			Quantity:    itemReq.Quantity,
			RecipeScale: recipeScale,
			VariantID:   variantID,
		})
		orderItem := models.OrderItem{
			ID:           itemID,
			OrderID:      orderID,
			MenuID:       menuItem.ID,
			VariantID:    variantIDPtr,
			VariantLabel: variantLabel,
			Quantity:     itemReq.Quantity,
			UnitRate:     unitPrice,
			Total:        unitPrice * float64(itemReq.Quantity),
			Status:       InitialOrderItemStatus(menuItem),
			Notes:        itemReq.Notes,
			SubId:        batchSubID,
		}

		if err := tx.Create(&orderItem).Error; err != nil {
			tx.Rollback()
			log.Printf("❌ [UpdateOrder] Failed to create order item: %v", err)
			return nil, nil, err
		}

		totalAdded += orderItem.Total
		log.Printf("🔵 [UpdateOrder] Added item: %s (qty: %d, total: ₹%.2f)", menuItem.Name, itemReq.Quantity, orderItem.Total)

		// Deduct inventory
		if err := tx.Model(&models.Inventory{}).
			Where("restaurant_id = ? AND menu_item_id = ?", restaurantID, menuItem.ID).
			Update("quantity", gorm.Expr("quantity - ?", itemReq.Quantity)).Error; err != nil {
			tx.Rollback()
			log.Printf("❌ [UpdateOrder] Inventory deduction failed: %v", err)
			return nil, nil, err
		}
	}

	// Recalculate order totals from all line items
	var restaurant models.Restaurant
	if err := tx.Where("id = ?", restaurantID).First(&restaurant).Error; err != nil {
		tx.Rollback()
		return nil, nil, err
	}

	var allItems []models.OrderItem
	if err := tx.Preload("MenuItem").Where("order_id = ?", orderID).Find(&allItems).Error; err != nil {
		tx.Rollback()
		return nil, nil, err
	}
	taxableGross, nonTaxableGross := orderItemsGrossSplit(allItems)
	subTotal, taxAmount, total := CalculateRestaurantOrderTax(taxableGross, nonTaxableGross, order.DiscountAmount, SettingsFromRestaurant(&restaurant))
	order.SubTotal = subTotal
	order.TaxAmount = taxAmount
	order.Total = total

	if err := tx.Model(&order).Updates(map[string]interface{}{
		"sub_total":  order.SubTotal,
		"tax_amount": order.TaxAmount,
		"total":      order.Total,
	}).Error; err != nil {
		tx.Rollback()
		log.Printf("❌ [UpdateOrder] Failed to update order totals: %v", err)
		return nil, nil, err
	}

	var updatedIngredients []models.Ingredient
	if len(stockQuantities) > 0 {
		var deductErr error
		updatedIngredients, deductErr = DeductIngredientsForMenuItems(tx, restaurantID, stockQuantities)
		if deductErr != nil {
			log.Printf("❌ [UpdateOrder] Ingredient stock deduction failed: %v", deductErr)
			tx.Rollback()
			return nil, nil, deductErr
		}
	}

	log.Printf("🔵 [UpdateOrder] Committing transaction for order #%d", order.OrderNumber)
	if err := tx.Commit().Error; err != nil {
		log.Printf("❌ [UpdateOrder] Transaction commit failed: %v", err)
		return nil, nil, err
	}

	// Reload order with new items
	if err := s.db.Where("id = ?", orderID).
		Preload("Items").
		Preload("Items.MenuItem").
		First(&order).Error; err != nil {
		log.Printf("❌ [UpdateOrder] Failed to reload order: %v", err)
		return nil, nil, err
	}

	log.Printf("✅ [UpdateOrder] Order updated successfully: Order #%d, New Total: ₹%.2f", order.OrderNumber, order.Total)

	return &order, updatedIngredients, nil
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
		Preload("AttendedBy").
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

	if order.OrderType != "counter" && order.TableID != nil && strings.TrimSpace(*order.TableID) != "" {
		if err := tx.Model(&models.RestaurantTable{}).
			Where("id = ? AND restaurant_id = ?", *order.TableID, restaurantID).
			Updates(map[string]interface{}{
				"is_occupied":             false,
				"current_order_id":        nil,
				"assistance_requested_at": nil,
			}).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		log.Printf("✅ Table released after order complete: %s", *order.TableID)
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	log.Printf("✅ Order completed: Order #%d (all items marked served)", order.OrderNumber)

	return s.reloadOrderWithItems(orderID, restaurantID)
}

// OrderPaymentDetails captures cash/UPI/split payment completion.
type OrderPaymentDetails struct {
	PaymentMethod     string
	AmountReceived    float64
	ChangeReturned    float64
	CashAmount        float64
	UpiAmount         float64
	AttendedByUserID  string
}

// CompleteOrderWithPayment completes order with payment details
func (s *OrderService) CompleteOrderWithPayment(restaurantID string, orderID string, payment OrderPaymentDetails) (*models.Order, error) {
	paymentMethod := payment.PaymentMethod
	amountReceived := payment.AmountReceived
	changeReturned := payment.ChangeReturned
	cashAmount := payment.CashAmount
	upiAmount := payment.UpiAmount
	attendedByUserID := payment.AttendedByUserID
	if attendedByUserID != "" {
		if err := s.validateAttendant(restaurantID, attendedByUserID); err != nil {
			return nil, err
		}
	}
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

	if paymentMethod == "split" {
		if cashAmount <= 0 || upiAmount <= 0 {
			tx.Rollback()
			return nil, errors.New("split payment requires cash_amount and upi_amount greater than zero")
		}
		totalPaid := cashAmount + upiAmount
		if totalPaid < order.Total-0.02 || totalPaid > order.Total+0.02 {
			tx.Rollback()
			return nil, fmt.Errorf("cash and upi amounts must equal order total (%.2f)", order.Total)
		}
	}

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
		"cash_amount":     cashAmount,
		"upi_amount":      upiAmount,
		"updated_at":      now,
	}
	if attendedByUserID != "" {
		updates["attended_by_user_id"] = attendedByUserID
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

	// Dine-in checkout frees the table (same as cancel) so clients can't leave it occupied.
	if !isCounter && order.TableID != nil && strings.TrimSpace(*order.TableID) != "" {
		if err := tx.Model(&models.RestaurantTable{}).
			Where("id = ? AND restaurant_id = ?", *order.TableID, restaurantID).
			Updates(map[string]interface{}{
				"is_occupied":             false,
				"current_order_id":        nil,
				"assistance_requested_at": nil,
			}).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		log.Printf("✅ Table released after checkout: %s", *order.TableID)
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
func (s *OrderService) CancelOrder(restaurantID string, orderID string) ([]models.Ingredient, error) {
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

	// Restore inventory for all items
	var items []models.OrderItem
	if err := tx.Where("order_id = ?", orderID).Find(&items).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	for _, item := range items {
		if item.Status == "served" {
			tx.Rollback()
			return nil, errors.New("cannot cancel order with served items")
		}
	}

	for _, item := range items {
		if err := tx.Model(&models.Inventory{}).
			Where("restaurant_id = ? AND menu_item_id = ?", restaurantID, item.MenuID).
			Update("quantity", gorm.Expr("quantity + ?", item.Quantity)).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		log.Printf("✅ Inventory restored: %s - %d units to stock", item.MenuID, item.Quantity)
	}

	var restoredIngredients []models.Ingredient
	if len(items) > 0 {
		var restoreErr error
		restoredIngredients, restoreErr = RestoreIngredientsForMenuItems(tx, restaurantID, menuItemQuantitiesFromOrderItems(items))
		if restoreErr != nil {
			log.Printf("❌ [CancelOrder] Ingredient stock restore failed: %v", restoreErr)
			tx.Rollback()
			return nil, restoreErr
		}
	}

	// Update order status
	if err := tx.Model(&order).Update("status", "cancelled").Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Release dine-in table when order is cancelled
	if order.TableID != nil && strings.TrimSpace(*order.TableID) != "" {
		if err := tx.Model(&models.RestaurantTable{}).
			Where("id = ? AND restaurant_id = ?", *order.TableID, restaurantID).
			Updates(map[string]interface{}{
				"is_occupied":      false,
				"current_order_id": nil,
			}).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		log.Printf("✅ Table released after order cancel: %s", *order.TableID)
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	log.Printf("✅ Order cancelled: Order #%d, Inventory restored", order.OrderNumber)

	return restoredIngredients, nil
}

// GetOrderByID retrieves order with items
func (s *OrderService) GetOrderByID(restaurantID string, orderID string) (*models.Order, error) {
	var order models.Order
	if err := s.db.Preload("Items").
		Preload("Items.MenuItem").
		Preload("AttendedBy").
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
	ID        string  `json:"id"`
	MenuID    string  `json:"menu_id"`
	Quantity  int     `json:"quantity"`
	UnitRate  float64 `json:"unit_rate"`
	Status    string  `json:"status"`
	Name      string  `json:"name"`
	IsVeg     bool    `json:"is_vegetarian"`
	SubId     string  `json:"sub_id,omitempty"`
	Notes     string  `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
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

// SalesDayPoint is one day's revenue in a sales chart series.
type SalesDayPoint struct {
	Date    string  `json:"date"` // YYYY-MM-DD
	Label   string  `json:"label"`
	Revenue float64 `json:"revenue"`
	Orders  int64   `json:"orders"`
}

// SalesComparison compares the selected period with the previous one.
type SalesComparison struct {
	PreviousRevenue   float64 `json:"previous_revenue"`
	PreviousOrders    int64   `json:"previous_orders"`
	RevenueChangePct  float64 `json:"revenue_change_pct"`
	OrdersChangePct   float64 `json:"orders_change_pct"`
	Direction         string  `json:"direction"` // up | down | flat
}

// TopSellingItem is a ranked menu item by quantity sold.
type TopSellingItem struct {
	Name     string  `json:"name"`
	Category string  `json:"category"`
	Quantity int64   `json:"quantity"`
	Revenue  float64 `json:"revenue"`
}

// SalesAnalytics is the full sales chart payload for week/month views.
type SalesAnalytics struct {
	Period            string           `json:"period"`
	From              string           `json:"from"`
	To                string           `json:"to"`
	TotalRevenue      float64          `json:"total_revenue"`
	TotalOrders       int64            `json:"total_orders"`
	AverageOrderValue float64          `json:"average_order_value"`
	Series            []SalesDayPoint  `json:"series"`
	Comparison        SalesComparison  `json:"comparison"`
	TopItems          []TopSellingItem `json:"top_items"`
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
			return db.Select("id", "order_id", "menu_id", "quantity", "unit_rate", "status", "total", "sub_id", "notes", "created_at")
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
			name := "Unknown Item"
			isVeg := false
			if item.MenuItem != nil {
				name = item.MenuItem.Name
				isVeg = item.MenuItem.IsVeg
			}
			name = FormatOrderItemDisplayName(name, item.VariantLabel)
			// Keep cancelled lines in the summary so Orders tiles can prioritize
			// "cancelled" over "ready" until the waiter opens the table.
			items = append(items, OrderSummaryItem{
				ID:        item.ID,
				MenuID:    item.MenuID,
				Quantity:  item.Quantity,
				UnitRate:  item.UnitRate,
				Status:    item.Status,
				Name:      name,
				IsVeg:     isVeg,
				SubId:     item.SubId,
				Notes:     item.Notes,
				CreatedAt: item.CreatedAt,
			})
			if item.Status == "cancelled" {
				continue
			}
			itemCount += item.Quantity
			if item.Status == "ready" {
				readyCount += item.Quantity
			}
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

// GetSalesAnalytics returns daily sales, previous-period comparison, and top-selling items.
// period must be "week", "last_week", or "month".
func (s *OrderService) GetSalesAnalytics(restaurantID string, period string) (*SalesAnalytics, error) {
	loc := RestaurantLocation()
	now := time.Now().In(loc)
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday → 7 so Monday is start of week
	}
	thisWeekStart := todayStart.AddDate(0, 0, -(weekday - 1))

	var currentStart, currentEnd, previousStart, previousEnd time.Time
	label := period
	switch period {
	case "month":
		currentStart = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
		currentEnd = todayStart.Add(24 * time.Hour)
		previousStart = currentStart.AddDate(0, -1, 0)
		previousEnd = currentStart
		label = "month"
	case "last_week":
		currentStart = thisWeekStart.AddDate(0, 0, -7)
		currentEnd = thisWeekStart
		previousStart = currentStart.AddDate(0, 0, -7)
		previousEnd = currentStart
		label = "last_week"
	case "week":
		currentStart = thisWeekStart
		currentEnd = todayStart.Add(24 * time.Hour)
		previousStart = thisWeekStart.AddDate(0, 0, -7)
		previousEnd = thisWeekStart
		label = "week"
	default:
		return nil, errors.New("period must be week, last_week, or month")
	}

	series, err := s.dailySalesSeries(restaurantID, currentStart, currentEnd)
	if err != nil {
		return nil, err
	}

	currentRevenue, currentOrders, err := s.salesTotals(restaurantID, currentStart, currentEnd)
	if err != nil {
		return nil, err
	}
	previousRevenue, previousOrders, err := s.salesTotals(restaurantID, previousStart, previousEnd)
	if err != nil {
		return nil, err
	}

	avg := float64(0)
	if currentOrders > 0 {
		avg = currentRevenue / float64(currentOrders)
	}

	revenueChangePct := pctChange(currentRevenue, previousRevenue)
	ordersChangePct := pctChange(float64(currentOrders), float64(previousOrders))
	direction := "flat"
	if revenueChangePct > 0.5 {
		direction = "up"
	} else if revenueChangePct < -0.5 {
		direction = "down"
	}

	topItems, err := s.topSellingItems(restaurantID, currentStart, currentEnd, 10)
	if err != nil {
		return nil, err
	}

	toInclusive := currentEnd.Add(-24 * time.Hour)
	return &SalesAnalytics{
		Period:            label,
		From:              currentStart.Format("2006-01-02"),
		To:                toInclusive.Format("2006-01-02"),
		TotalRevenue:      currentRevenue,
		TotalOrders:       currentOrders,
		AverageOrderValue: avg,
		Series:            series,
		Comparison: SalesComparison{
			PreviousRevenue:  previousRevenue,
			PreviousOrders:   previousOrders,
			RevenueChangePct: revenueChangePct,
			OrdersChangePct:  ordersChangePct,
			Direction:        direction,
		},
		TopItems: topItems,
	}, nil
}

func pctChange(current, previous float64) float64 {
	if previous == 0 {
		if current == 0 {
			return 0
		}
		return 100
	}
	return ((current - previous) / previous) * 100
}

func (s *OrderService) salesTotals(restaurantID string, from, toEnd time.Time) (float64, int64, error) {
	var result struct {
		TotalOrders  int64
		TotalRevenue float64
	}
	err := s.db.Model(&models.Order{}).
		Where("restaurant_id = ?", restaurantID).
		Where("(status = ? OR (order_type = ? AND payment_method <> ''))", "completed", "counter").
		Where(historyActivityAtSQL+" >= ? AND "+historyActivityAtSQL+" < ?", from, toEnd).
		Select("COUNT(*) AS total_orders, COALESCE(SUM(total), 0) AS total_revenue").
		Scan(&result).Error
	return result.TotalRevenue, result.TotalOrders, err
}

// SalesStatsForRange returns revenue, order count, AOV, and top-selling items for [from, toEnd).
func (s *OrderService) SalesStatsForRange(restaurantID string, from, toEnd time.Time, topN int) (float64, int64, float64, []TopSellingItem, error) {
	revenue, orders, err := s.salesTotals(restaurantID, from, toEnd)
	if err != nil {
		return 0, 0, 0, nil, err
	}
	avg := float64(0)
	if orders > 0 {
		avg = revenue / float64(orders)
	}
	if topN <= 0 {
		topN = 5
	}
	topItems, err := s.topSellingItems(restaurantID, from, toEnd, topN)
	if err != nil {
		return 0, 0, 0, nil, err
	}
	return revenue, orders, avg, topItems, nil
}

func (s *OrderService) dailySalesSeries(restaurantID string, from, toEnd time.Time) ([]SalesDayPoint, error) {
	loc := RestaurantLocation()
	tz := loc.String()

	type dayRow struct {
		Day     time.Time
		Orders  int64
		Revenue float64
	}

	// Convert activity timestamp into the restaurant local calendar date.
	daySQL := fmt.Sprintf("((%s AT TIME ZONE '%s')::date)", historyActivityAtSQL, tz)

	var rows []dayRow
	err := s.db.Model(&models.Order{}).
		Select(daySQL+" AS day, COUNT(*) AS orders, COALESCE(SUM(total), 0) AS revenue").
		Where("restaurant_id = ?", restaurantID).
		Where("(status = ? OR (order_type = ? AND payment_method <> ''))", "completed", "counter").
		Where(historyActivityAtSQL+" >= ? AND "+historyActivityAtSQL+" < ?", from, toEnd).
		Group(daySQL).
		Order("day ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	byDate := make(map[string]dayRow, len(rows))
	for _, row := range rows {
		key := row.Day.In(loc).Format("2006-01-02")
		// Postgres date scan can arrive as UTC midnight; format via date components.
		if row.Day.Location() == time.UTC {
			key = time.Date(row.Day.Year(), row.Day.Month(), row.Day.Day(), 0, 0, 0, 0, loc).Format("2006-01-02")
		}
		byDate[key] = row
	}

	series := make([]SalesDayPoint, 0)
	for d := from; d.Before(toEnd); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		row := byDate[key]
		label := d.Format("Mon")
		if toEnd.Sub(from) > 8*24*time.Hour {
			label = d.Format("2")
		}
		series = append(series, SalesDayPoint{
			Date:    key,
			Label:   label,
			Revenue: row.Revenue,
			Orders:  row.Orders,
		})
	}
	return series, nil
}

func (s *OrderService) topSellingItems(restaurantID string, from, toEnd time.Time, limit int) ([]TopSellingItem, error) {
	if limit <= 0 {
		limit = 10
	}

	type itemRow struct {
		Name     string
		Category string
		Quantity int64
		Revenue  float64
	}

	var rows []itemRow
	err := s.db.Table("order_items AS oi").
		Select("COALESCE(mi.name, 'Unknown item') AS name, COALESCE(mi.category, '') AS category, COALESCE(SUM(oi.quantity), 0) AS quantity, COALESCE(SUM(oi.total), 0) AS revenue").
		Joins("JOIN orders AS o ON o.id = oi.order_id").
		Joins("LEFT JOIN menu_items AS mi ON mi.id = oi.menu_id").
		Where("o.restaurant_id = ?", restaurantID).
		Where("(o.status = ? OR (o.order_type = ? AND o.payment_method <> ''))", "completed", "counter").
		Where("COALESCE(o.completed_at, o.created_at) >= ? AND COALESCE(o.completed_at, o.created_at) < ?", from, toEnd).
		Group("mi.name, mi.category").
		Order("quantity DESC, revenue DESC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	items := make([]TopSellingItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, TopSellingItem{
			Name:     row.Name,
			Category: row.Category,
			Quantity: row.Quantity,
			Revenue:  row.Revenue,
		})
	}
	return items, nil
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
		// cancelled = kitchen voided the line; treat as finished for counter completion
		if item.Status != "ready" && item.Status != "served" && item.Status != "cancelled" {
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

// AdjustOrderItemQuantity lets admin/manager remove a line (quantity 0) or reduce its qty on an active order.
// Restores menu + ingredient stock for the removed quantity and recalculates order totals (excluding cancelled lines).
func (s *OrderService) AdjustOrderItemQuantity(
	restaurantID string,
	orderID string,
	itemID string,
	newQuantity int,
) (*models.Order, []models.Ingredient, error) {
	if newQuantity < 0 {
		return nil, nil, errors.New("quantity cannot be negative")
	}

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
			return nil, nil, errors.New("order not found")
		}
		return nil, nil, err
	}

	if order.Status == "completed" || order.Status == "cancelled" {
		tx.Rollback()
		return nil, nil, errors.New("cannot adjust items on a completed or cancelled order")
	}

	var item models.OrderItem
	if err := tx.Where("id = ? AND order_id = ?", itemID, orderID).First(&item).Error; err != nil {
		tx.Rollback()
		if err == gorm.ErrRecordNotFound {
			return nil, nil, errors.New("order item not found")
		}
		return nil, nil, err
	}

	if item.Status == "cancelled" {
		tx.Rollback()
		return nil, nil, errors.New("kitchen-cancelled items must be dismissed, not adjusted")
	}

	if item.Status == "served" {
		tx.Rollback()
		return nil, nil, errors.New("cannot adjust served items")
	}

	if newQuantity > item.Quantity {
		tx.Rollback()
		return nil, nil, errors.New("cannot increase quantity here — use add items")
	}
	if newQuantity == item.Quantity {
		tx.Rollback()
		existing, err := s.GetOrderByID(restaurantID, orderID)
		return existing, nil, err
	}

	removedQty := item.Quantity - newQuantity

	if newQuantity == 0 {
		if err := tx.Where("id = ? AND order_id = ?", itemID, orderID).Delete(&models.OrderItem{}).Error; err != nil {
			tx.Rollback()
			return nil, nil, err
		}
	} else {
		newTotal := item.UnitRate * float64(newQuantity)
		if err := tx.Model(&item).Updates(map[string]interface{}{
			"quantity": newQuantity,
			"total":    newTotal,
		}).Error; err != nil {
			tx.Rollback()
			return nil, nil, err
		}
	}

	if err := tx.Model(&models.Inventory{}).
		Where("restaurant_id = ? AND menu_item_id = ?", restaurantID, item.MenuID).
		Update("quantity", gorm.Expr("quantity + ?", removedQty)).Error; err != nil {
		tx.Rollback()
		return nil, nil, err
	}

	restoreQty := []MenuItemQuantity{{
		MenuItemID: item.MenuID,
		Quantity:   removedQty,
	}}
	if item.VariantID != nil {
		restoreQty[0].VariantID = *item.VariantID
	}
	restoreQty, enrichErr := enrichMenuItemQuantitiesWithScales(tx, restaurantID, restoreQty)
	if enrichErr != nil {
		tx.Rollback()
		return nil, nil, enrichErr
	}
	restoredIngredients, restoreErr := RestoreIngredientsForMenuItems(tx, restaurantID, restoreQty)
	if restoreErr != nil {
		tx.Rollback()
		return nil, nil, restoreErr
	}

	var restaurant models.Restaurant
	if err := tx.Where("id = ?", restaurantID).First(&restaurant).Error; err != nil {
		tx.Rollback()
		return nil, nil, err
	}

	var allItems []models.OrderItem
	if err := tx.Preload("MenuItem").Where("order_id = ?", orderID).Find(&allItems).Error; err != nil {
		tx.Rollback()
		return nil, nil, err
	}
	taxableGross, nonTaxableGross := orderItemsGrossSplit(allItems)
	subTotal, taxAmount, total := CalculateRestaurantOrderTax(taxableGross, nonTaxableGross, order.DiscountAmount, SettingsFromRestaurant(&restaurant))
	if err := tx.Model(&order).Updates(map[string]interface{}{
		"sub_total":  subTotal,
		"tax_amount": taxAmount,
		"total":      total,
	}).Error; err != nil {
		tx.Rollback()
		return nil, nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, nil, err
	}

	updated, err := s.GetOrderByID(restaurantID, orderID)
	if err != nil {
		return nil, restoredIngredients, err
	}

	log.Printf("✅ Adjusted order item %s on order %s: qty %d -> %d (restored %d)", itemID, orderID, item.Quantity, newQuantity, removedQty)
	return updated, restoredIngredients, nil
}

// DeleteCancelledOrderItem permanently removes a kitchen-cancelled line after waiter dismisses it.
func (s *OrderService) DeleteCancelledOrderItem(restaurantID string, orderID string, itemID string) error {
	var order models.Order
	if err := s.db.Where("id = ? AND restaurant_id = ?", orderID, restaurantID).
		First(&order).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New("order not found")
		}
		return err
	}

	if order.OrderType == "counter" || IsLegacyCounterOrder(&order) {
		return errors.New("dismissing cancelled items is only allowed for dine-in orders")
	}

	var item models.OrderItem
	if err := s.db.Where("id = ? AND order_id = ?", itemID, orderID).First(&item).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New("order item not found")
		}
		return err
	}
	if item.Status != "cancelled" {
		return errors.New("only kitchen-cancelled items can be dismissed")
	}

	result := s.db.Where("id = ? AND order_id = ?", itemID, orderID).Delete(&models.OrderItem{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("order item not found")
	}

	log.Printf("✅ Dismissed kitchen-cancelled order item %s from order %s", itemID, orderID)
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

// allocateCounterTicket assigns the next daily counter ticket inside the caller's transaction.
// pg_advisory_xact_lock serializes allocation per restaurant per business day so concurrent
// counter devices cannot receive the same ticket number.
func allocateCounterTicket(tx *gorm.DB, restaurantID string, todayStart time.Time) (int, error) {
	lockKey := restaurantID + ":" + todayStart.Format("2006-01-02")
	if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", lockKey).Error; err != nil {
		return 0, fmt.Errorf("counter ticket lock: %w", err)
	}
	maxTicket, err := getMaxCounterTicketToday(tx, restaurantID, todayStart)
	if err != nil {
		return 0, err
	}
	return maxTicket + 1, nil
}

// GetNextCounterTicket returns a preview of the next daily counter ticket (not reserved).
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

func (s *OrderService) validateAttendant(restaurantID, userID string) error {
	var user models.User
	if err := s.db.Where(
		"id = ? AND restaurant_id = ? AND is_active = ? AND role IN ('admin', 'manager', 'staff')",
		userID, restaurantID, true,
	).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("attended by user not found")
		}
		return err
	}
	return nil
}

// AttendedByName returns the display name for who served the order.
func AttendedByName(order *models.Order) string {
	if order == nil {
		return ""
	}
	if order.AttendedBy != nil && order.AttendedBy.Name != "" {
		return order.AttendedBy.Name
	}
	if order.AttendedByUserID == nil || *order.AttendedByUserID == "" {
		return ""
	}
	return ""
}
