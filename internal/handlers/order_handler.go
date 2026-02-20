package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"restaurant-api/internal/models"
	"restaurant-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type OrderHandler struct {
	orderService *services.OrderService
	validator    *validator.Validate
}

// NewOrderHandler creates a new order handler
func NewOrderHandler(orderService *services.OrderService) *OrderHandler {
	return &OrderHandler{
		orderService: orderService,
		validator:    validator.New(),
	}
}

// CreateOrder handles order creation with inventory deduction
// @Summary Create new order
// @Description Create a new order and automatically deduct inventory
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param request body services.CreateOrderRequest true "Order data"
// @Success 201 {object} services.OrderResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /orders [post]
func (h *OrderHandler) CreateOrder(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	var req services.CreateOrderRequest

	// Bind JSON
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ Binding error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set restaurant_id from JWT context BEFORE validation
	req.RestaurantID = restaurantID.(string)

	// Validate
	if err := h.validator.Struct(req); err != nil {
		log.Printf("❌ Validation error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create order
	order, err := h.orderService.CreateOrder(restaurantID.(string), userID.(string), req)
	if err != nil {
		log.Printf("❌ Order creation failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Order created: #%d with inventory deduction", order.OrderNumber)

	// Broadcast order creation event via WebSocket
	if globalHub != nil {
		BroadcastOrderUpdate(globalHub, restaurantID.(string), order)
	}

	tableIDValue := ""
	if order.TableID != nil {
		tableIDValue = *order.TableID
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Order created successfully with inventory deducted",
		"order": gin.H{
			"id":           order.ID,
			"order_number": order.OrderNumber,
			"table_number": order.TableNumber,
			"table_id":     tableIDValue,
			"status":       order.Status,
			"sub_total":    order.SubTotal,
			"tax_amount":   order.TaxAmount,
			"total":        order.Total,
			"created_at":   order.CreatedAt,
		},
	})
}

// GetOrder retrieves a specific order
// @Summary Get order details
// @Description Get order with all items
// @Security ApiKeyAuth
// @Produce json
// @Param order_id path string true "Order ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /orders/:order_id [get]
func (h *OrderHandler) GetOrder(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	orderID := c.Param("order_id")

	order, err := h.orderService.GetOrderByID(restaurantID.(string), orderID)
	if err != nil {
		log.Printf("❌ Order retrieval failed: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Order retrieved: #%d", order.OrderNumber)

	// Transform order to include items with menu details
	type OrderItemResponse struct {
		ID        string                 `json:"id"`
		OrderID   string                 `json:"order_id"`
		MenuID    string                 `json:"menu_id"`
		Quantity  int                    `json:"quantity"`
		UnitRate  float64                `json:"unit_rate"`
		Total     float64                `json:"total"`
		Status    string                 `json:"status"`
		SubId     string                 `json:"sub_id,omitempty"`
		Notes     string                 `json:"notes"`
		CreatedAt time.Time              `json:"created_at"`
		MenuItem  map[string]interface{} `json:"menu_item,omitempty"`
	}

	items := make([]OrderItemResponse, 0, len(order.Items))
	for _, item := range order.Items {
		itemResp := OrderItemResponse{
			ID:        item.ID,
			OrderID:   item.OrderID,
			MenuID:    item.MenuID,
			Quantity:  item.Quantity,
			UnitRate:  item.UnitRate,
			Total:     item.Total,
			Status:    item.Status,
			SubId:     item.SubId,
			Notes:     item.Notes,
			CreatedAt: item.CreatedAt,
		}

		// Include menu item details if available
		if item.MenuItem != nil {
			itemResp.MenuItem = map[string]interface{}{
				"id":            item.MenuItem.ID,
				"name":          item.MenuItem.Name,
				"description":   item.MenuItem.Description,
				"price":         item.MenuItem.Price,
				"cost_price":    item.MenuItem.CostPrice,
				"is_veg":        item.MenuItem.IsVeg,
				"is_vegetarian": item.MenuItem.IsVeg, // Alias for compatibility
				"is_available":  item.MenuItem.IsAvailable,
				"category":      item.MenuItem.Category,
				"restaurant_id": item.MenuItem.RestaurantID,
			}
		}

		items = append(items, itemResp)
	}

	c.JSON(http.StatusOK, gin.H{
		"order": gin.H{
			"id":              order.ID,
			"restaurant_id":   order.RestaurantID,
			"table_number":    order.TableNumber,
			"table_id":        order.TableID,
			"customer_name":   order.CustomerName,
			"order_number":    order.OrderNumber,
			"status":          order.Status,
			"sub_total":       order.SubTotal,
			"tax_amount":      order.TaxAmount,
			"discount_amount": order.DiscountAmount,
			"total":           order.Total,
			"payment_method":  order.PaymentMethod,
			"notes":           order.Notes,
			"created_at":      order.CreatedAt,
			"updated_at":      order.UpdatedAt,
			"completed_at":    order.CompletedAt,
			"items":           items,
		},
	})
}

// UpdateOrder updates an existing order with new items
// @Summary Update order
// @Description Add items to an existing order
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param order_id path string true "Order ID"
// @Param request body services.CreateOrderRequest true "Updated order data"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /orders/:order_id [put]
func (h *OrderHandler) UpdateOrder(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	orderID := c.Param("order_id")

	var req services.CreateOrderRequest

	// Bind JSON
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ Binding error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set restaurant_id from JWT context
	req.RestaurantID = restaurantID.(string)

	// Update order via service
	order, err := h.orderService.UpdateOrder(restaurantID.(string), orderID, req)
	if err != nil {
		log.Printf("❌ Order update failed: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Order updated: #%d", order.OrderNumber)

	// Broadcast order update event via WebSocket
	if globalHub != nil {
		BroadcastOrderUpdate(globalHub, restaurantID.(string), order)
	}

	tableIDValue := ""
	if order.TableID != nil {
		tableIDValue = *order.TableID
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Order updated successfully",
		"order": gin.H{
			"id":           order.ID,
			"order_number": order.OrderNumber,
			"table_number": order.TableNumber,
			"table_id":     tableIDValue,
			"status":       order.Status,
			"sub_total":    order.SubTotal,
			"tax_amount":   order.TaxAmount,
			"total":        order.Total,
			"items":        order.Items,
		},
	})
}

// ListOrders retrieves all orders for a restaurant
// @Summary List orders
// @Description Get paginated list of orders
// @Security ApiKeyAuth
// @Produce json
// @Param status query string false "Filter by status: pending, cooking, completed, or active (pending+cooking only)"
// @Param limit query int false "Items per page (default: 20)"
// @Param offset query int false "Pagination offset (default: 0)"
// @Success 200 {object} map[string]interface{}
// @Router /orders [get]
func (h *OrderHandler) ListOrders(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	// Parse query params
	status := c.DefaultQuery("status", "")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit > 100 {
		limit = 100
	}

	orders, total, err := h.orderService.ListOrders(restaurantID.(string), status, limit, offset)
	if err != nil {
		log.Printf("❌ Order list retrieval failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Orders listed: %d found", len(orders))

	// Transform orders to include items with menu details
	type OrderItemResponse struct {
		ID        string                 `json:"id"`
		OrderID   string                 `json:"order_id"`
		MenuID    string                 `json:"menu_id"`
		Quantity  int                    `json:"quantity"`
		UnitRate  float64                `json:"unit_rate"`
		Total     float64                `json:"total"`
		Status    string                 `json:"status"`
		SubId     string                 `json:"sub_id,omitempty"`
		Notes     string                 `json:"notes"`
		CreatedAt time.Time              `json:"created_at"`
		MenuItem  map[string]interface{} `json:"menu_item,omitempty"`
	}

	type OrderResponse struct {
		ID             string              `json:"id"`
		RestaurantID   string              `json:"restaurant_id"`
		TableNumber    string              `json:"table_number"`
		TableID        *string             `json:"table_id,omitempty"`
		CustomerName   string              `json:"customer_name"`
		OrderNumber    int                 `json:"order_number"`
		Status         string              `json:"status"`
		SubTotal       float64             `json:"sub_total"`
		TaxAmount      float64             `json:"tax_amount"`
		DiscountAmount float64             `json:"discount_amount"`
		Total          float64             `json:"total"`
		PaymentMethod  string              `json:"payment_method"`
		Notes          string              `json:"notes"`
		CreatedAt      time.Time           `json:"created_at"`
		UpdatedAt      time.Time           `json:"updated_at"`
		CompletedAt    *time.Time          `json:"completed_at,omitempty"`
		Items          []OrderItemResponse `json:"items"`
	}

	ordersResponse := make([]OrderResponse, 0, len(orders))
	for _, order := range orders {
		items := make([]OrderItemResponse, 0, len(order.Items))
		for _, item := range order.Items {
			itemResp := OrderItemResponse{
				ID:        item.ID,
				OrderID:   item.OrderID,
				MenuID:    item.MenuID,
				Quantity:  item.Quantity,
				UnitRate:  item.UnitRate,
				Total:     item.Total,
				Status:    item.Status,
				SubId:     item.SubId,
				Notes:     item.Notes,
				CreatedAt: item.CreatedAt,
			}

			// Include menu item details if available
			if item.MenuItem != nil {
				itemResp.MenuItem = map[string]interface{}{
					"id":            item.MenuItem.ID,
					"name":          item.MenuItem.Name,
					"description":   item.MenuItem.Description,
					"price":         item.MenuItem.Price,
					"cost_price":    item.MenuItem.CostPrice,
					"is_veg":        item.MenuItem.IsVeg,
					"is_vegetarian": item.MenuItem.IsVeg, // Alias for compatibility
					"is_available":  item.MenuItem.IsAvailable,
					"category":      item.MenuItem.Category,
					"restaurant_id": item.MenuItem.RestaurantID,
				}
			}

			items = append(items, itemResp)
		}

		ordersResponse = append(ordersResponse, OrderResponse{
			ID:             order.ID,
			RestaurantID:   order.RestaurantID,
			TableNumber:    order.TableNumber,
			TableID:        order.TableID,
			CustomerName:   order.CustomerName,
			OrderNumber:    order.OrderNumber,
			Status:         order.Status,
			SubTotal:       order.SubTotal,
			TaxAmount:      order.TaxAmount,
			DiscountAmount: order.DiscountAmount,
			Total:          order.Total,
			PaymentMethod:  order.PaymentMethod,
			Notes:          order.Notes,
			CreatedAt:      order.CreatedAt,
			UpdatedAt:      order.UpdatedAt,
			CompletedAt:    order.CompletedAt,
			Items:          items,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"orders": ordersResponse,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// CompleteOrder marks an order as completed
// @Summary Complete order
// @Description Mark order as completed/served
// @Security ApiKeyAuth
// @Produce json
// @Param order_id path string true "Order ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /orders/:order_id/complete [put]
func (h *OrderHandler) CompleteOrder(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	orderID := c.Param("order_id")

	order, err := h.orderService.CompleteOrder(restaurantID.(string), orderID)
	if err != nil {
		log.Printf("❌ Order completion failed: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Order completed: #%d", order.OrderNumber)

	// Broadcast order completion via WebSocket
	if globalHub != nil {
		BroadcastOrderUpdate(globalHub, restaurantID.(string), order)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Order marked as completed",
		"order": gin.H{
			"id":           order.ID,
			"order_number": order.OrderNumber,
			"status":       order.Status,
			"completed_at": order.CompletedAt,
		},
	})
}

// CompleteOrderWithPayment completes an order with payment details (cash/UPI)
// @Summary Complete order with payment
// @Description Complete order and record payment details (amount received, change)
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param order_id path string true "Order ID"
// @Param request body object true "Payment details"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /orders/:order_id/complete-payment [post]
func (h *OrderHandler) CompleteOrderWithPayment(c *gin.Context) {
	log.Printf("🔵 [Handler] CompleteOrderWithPayment called")
	log.Printf("   Order ID from URL: %s", c.Param("order_id"))

	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}
	log.Printf("   Restaurant ID: %v", restaurantID)

	orderID := c.Param("order_id")

	var input struct {
		PaymentMethod  string  `json:"payment_method" binding:"required"` // "cash" or "upi"
		AmountReceived float64 `json:"amount_received,omitempty"`
		ChangeReturned float64 `json:"change_returned,omitempty"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("❌ [Handler] JSON binding error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	log.Printf("   Payment Data: Method=%s, Received=%.2f, Change=%.2f", input.PaymentMethod, input.AmountReceived, input.ChangeReturned)

	// Validate payment method
	if input.PaymentMethod != "cash" && input.PaymentMethod != "upi" {
		log.Printf("❌ [Handler] Invalid payment method: %s", input.PaymentMethod)
		c.JSON(http.StatusBadRequest, gin.H{"error": "payment_method must be 'cash' or 'upi'"})
		return
	}

	// For cash payments, amount_received is required
	if input.PaymentMethod == "cash" && input.AmountReceived == 0 {
		log.Printf("❌ [Handler] Cash payment missing amount_received")
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount_received is required for cash payments"})
		return
	}

	log.Printf("   → Calling service.CompleteOrderWithPayment...")

	// Complete the order with payment details
	order, err := h.orderService.CompleteOrderWithPayment(
		restaurantID.(string),
		orderID,
		input.PaymentMethod,
		input.AmountReceived,
		input.ChangeReturned,
	)

	if err != nil {
		log.Printf("❌ [Handler] Order payment completion failed: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ [Handler] Order #%d completed with %s payment. Response:", order.OrderNumber, input.PaymentMethod)

	// Broadcast order completion via WebSocket
	if globalHub != nil {
		BroadcastOrderUpdate(globalHub, restaurantID.(string), order)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Order completed successfully",
		"order": gin.H{
			"id":              order.ID,
			"order_number":    order.OrderNumber,
			"status":          order.Status,
			"total":           order.Total,
			"payment_method":  order.PaymentMethod,
			"amount_received": order.AmountReceived,
			"change_returned": order.ChangeReturned,
			"completed_at":    order.CompletedAt,
		},
	})
}

// CancelOrder cancels an order and restores inventory
// @Summary Cancel order
// @Description Cancel order and restore inventory
// @Security ApiKeyAuth
// @Produce json
// @Param order_id path string true "Order ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /orders/:order_id/cancel [put]
func (h *OrderHandler) CancelOrder(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	orderID := c.Param("order_id")

	err := h.orderService.CancelOrder(restaurantID.(string), orderID)
	if err != nil {
		log.Printf("❌ Order cancellation failed: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Order cancelled: %s, Inventory restored", orderID)

	// Fetch cancelled order and broadcast the cancellation
	order, err := h.orderService.GetOrderByID(restaurantID.(string), orderID)
	if err == nil && globalHub != nil {
		BroadcastOrderUpdate(globalHub, restaurantID.(string), order)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Order cancelled and inventory restored",
		"order_id": orderID,
	})
}

// UpdateOrderItemStatus updates the status of a specific order item
// @Summary Update order item status
// @Description Update the status of an order item (pending, cooking, ready, served)
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param order_id path string true "Order ID"
// @Param item_id path string true "Order Item ID"
// @Param request body map[string]string true "Status update"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /orders/:order_id/items/:item_id/status [put]
func (h *OrderHandler) UpdateOrderItemStatus(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	orderID := c.Param("order_id")
	itemID := c.Param("item_id")

	var input struct {
		Status string `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate status
	validStatuses := []string{"pending", "cooking", "ready", "served"}
	isValid := false
	for _, s := range validStatuses {
		if input.Status == s {
			isValid = true
			break
		}
	}
	if !isValid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status. Must be: pending, cooking, ready, or served"})
		return
	}

	err := h.orderService.UpdateOrderItemStatus(restaurantID.(string), orderID, itemID, input.Status)
	if err != nil {
		log.Printf("❌ Order item status update failed: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Order item %s status updated to: %s", itemID, input.Status)

	// Fetch updated order with all items for comprehensive broadcast
	updatedOrder, err := h.orderService.GetOrderByID(restaurantID.(string), orderID)
	if err != nil {
		log.Printf("⚠️  Could not fetch updated order for broadcast: %v", err)
	} else if globalHub != nil {
		// Broadcast full order status change with complete order details
		BroadcastOrderUpdate(globalHub, restaurantID.(string), updatedOrder)
		
		// Check if ALL items are served - if so, broadcast table as unoccupied
		allServed := true
		for _, item := range updatedOrder.Items {
			if item.Status != "served" && item.Status != "cancelled" {
				allServed = false
				break
			}
		}
		
		if allServed && updatedOrder.TableID != nil {
			log.Printf("📍 All items served for order #%d on table %s, marking table as unoccupied", updatedOrder.OrderNumber, updatedOrder.TableNumber)
			// Broadcast table status change (unoccupied)
			BroadcastTableUpdate(globalHub, restaurantID.(string), &models.RestaurantTable{
				ID:             *updatedOrder.TableID,
				Name:           updatedOrder.TableNumber,
				IsOccupied:     false,
				CurrentOrderID: nil,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Order item status updated successfully",
		"item_id":  itemID,
		"order_id": orderID,
		"status":   input.Status,
	})
}

// UpdateOrderItemsByMenuID updates all items with a specific menu item ID
// @Summary Update order items by menu item ID
// @Description Update status of all items in an order with a specific menu item ID (for bulk updates)
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param order_id path string true "Order ID"
// @Param menu_id path string true "Menu Item ID"
// @Param request body map[string]string true "Status update"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /orders/:order_id/menu-items/:menu_id/status [put]
func (h *OrderHandler) UpdateOrderItemsByMenuID(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	orderID := c.Param("order_id")
	menuItemID := c.Param("menu_id")

	var input struct {
		Status string `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate status
	validStatuses := []string{"pending", "cooking", "ready", "served"}
	isValid := false
	for _, s := range validStatuses {
		if input.Status == s {
			isValid = true
			break
		}
	}
	if !isValid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status. Must be: pending, cooking, ready, or served"})
		return
	}

	err := h.orderService.UpdateOrderItemsByMenuID(restaurantID.(string), orderID, menuItemID, input.Status)
	if err != nil {
		log.Printf("❌ Order items status update failed: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Order items with menu_id %s updated to status: %s", menuItemID, input.Status)

	// Broadcast order status change via WebSocket
	if globalHub != nil {
		event := models.NotificationEvent{
			Type:      "order_status_changed",
			RoomID:    restaurantID.(string),
			Timestamp: time.Now(),
			Data: json.RawMessage(toJSON(map[string]interface{}{
				"order_id":    orderID,
				"menu_id":     menuItemID,
				"status":      input.Status,
				"bulk_update": true,
			})),
		}
		globalHub.BroadcastToRoom(restaurantID.(string), event)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Order items status updated successfully",
		"menu_id":  menuItemID,
		"order_id": orderID,
		"status":   input.Status,
	})
}
