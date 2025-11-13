package services

import (
	"errors"
	"log"

	"restaurant-api/internal/models"

	"gorm.io/gorm"
)

type OrderService struct {
	db *gorm.DB
}

type CreateOrderRequest struct {
	RestaurantID string                   `json:"restaurant_id"` // Set by handler from JWT
	TableNumber  int                      `json:"table_number" validate:"required,min=1"`
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
	TableNumber int                 `json:"table_number"`
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

// CreateOrder creates a new order and deducts inventory
func (s *OrderService) CreateOrder(restaurantID string, userID string, req CreateOrderRequest) (*models.Order, error) {
	// Validate items exist and check inventory
	for _, item := range req.Items {
		var inventory models.Inventory
		if err := s.db.Where("restaurant_id = ? AND menu_item_id = ?", restaurantID, item.MenuItemID).
			First(&inventory).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil, errors.New("menu item not found or inventory not set up")
			}
			return nil, err
		}

		// Check if enough stock
		if inventory.Quantity < float64(item.Quantity) {
			return nil, errors.New("insufficient inventory for item")
		}
	}

	// Start transaction
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
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

	// Create order
	order := &models.Order{
		ID:              "",
		RestaurantID:    restaurantID,
		TableNumber:     req.TableNumber,
		OrderNumber:     orderNumber,
		Status:          "pending",
		SubTotal:        0,
		TaxAmount:       0,
		Total:           0,
		CreatedByUserID: userID,
		Notes:           req.Notes,
	}

	if err := tx.Create(order).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Create order items and deduct inventory
	subTotal := 0.0
	for _, itemReq := range req.Items {
		var menuItem models.MenuItem
		if err := tx.Where("id = ?", itemReq.MenuItemID).First(&menuItem).Error; err != nil {
			tx.Rollback()
			return nil, err
		}

		// Create order item
		orderItem := &models.OrderItem{
			OrderID:  order.ID,
			MenuID:   menuItem.ID,
			Quantity: itemReq.Quantity,
			UnitRate: menuItem.Price,
			Total:    menuItem.Price * float64(itemReq.Quantity),
			Status:   "pending",
			Notes:    itemReq.Notes,
		}

		if err := tx.Create(orderItem).Error; err != nil {
			tx.Rollback()
			return nil, err
		}

		subTotal += orderItem.Total

		// Deduct inventory
		if err := tx.Model(&models.Inventory{}).
			Where("restaurant_id = ? AND menu_item_id = ?", restaurantID, menuItem.ID).
			Update("quantity", gorm.Expr("quantity - ?", itemReq.Quantity)).Error; err != nil {
			tx.Rollback()
			return nil, err
		}

		log.Printf("✅ Inventory deducted: %s - %d units from stock", menuItem.Name, itemReq.Quantity)
	}

	// Calculate tax (assume 5% GST)
	taxAmount := subTotal * 0.05
	total := subTotal + taxAmount

	// Update order totals
	if err := tx.Model(order).Updates(map[string]interface{}{
		"sub_total":  subTotal,
		"tax_amount": taxAmount,
		"total":      total,
	}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	log.Printf("✅ Order created: Order #%d, Total: ₹%.2f, Inventory deducted", order.OrderNumber, total)

	return order, nil
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
		query = query.Where("status = ?", status)
	}

	if err := query.Model(&models.Order{}).Count(&count).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Preload("Items").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&orders).Error; err != nil {
		return nil, 0, err
	}

	return orders, count, nil
}
