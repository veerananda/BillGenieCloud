package handlers

import (
	"errors"
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
	checkoutLock *services.CheckoutLockService
	authService  *services.AuthService
	validator    *validator.Validate
}

// NewOrderHandler creates a new order handler
func NewOrderHandler(orderService *services.OrderService, checkoutLock *services.CheckoutLockService, authService *services.AuthService) *OrderHandler {
	return &OrderHandler{
		orderService: orderService,
		checkoutLock: checkoutLock,
		authService:  authService,
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
	if err := services.ValidateCreateOrderRequest(req); err != nil {
		log.Printf("❌ Order validation error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var restaurant models.Restaurant
	if err := h.orderService.GetDB().Where("id = ?", restaurantID.(string)).First(&restaurant).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load restaurant"})
		return
	}
	limits, err := services.LoadSubscriptionLimits(h.orderService.GetDB(), &restaurant)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load subscription"})
		return
	}
	if err := services.ValidateOrderCreate(limits, req); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	// Create order
	order, updatedIngredients, err := h.orderService.CreateOrder(restaurantID.(string), userID.(string), req)
	if err != nil {
		log.Printf("❌ Order creation failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Order created: #%d with inventory deduction", order.OrderNumber)

	// Broadcast order creation event via WebSocket
	if globalHub != nil {
		BroadcastOrderEvent(globalHub, restaurantID.(string), "order_created", order)
		BroadcastIngredientInventoryUpdates(globalHub, restaurantID.(string), updatedIngredients)
	}

	tableIDValue := ""
	if order.TableID != nil {
		tableIDValue = *order.TableID
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Order created successfully with inventory deducted",
		"order": gin.H{
			"id":            order.ID,
			"order_number":  order.OrderNumber,
			"ticket_number": order.TicketNumber,
			"order_type":    order.OrderType,
			"service_mode":  order.ServiceMode,
			"table_number":  order.TableNumber,
			"table_id":      tableIDValue,
			"customer_name":  order.CustomerName,
			"customer_phone": order.CustomerPhone,
			"is_self_service": order.OrderType == "counter",
			"status":        order.Status,
			"sub_total":     order.SubTotal,
			"tax_amount":    order.TaxAmount,
			"total":         order.Total,
			"created_at":    order.CreatedAt,
			"items":         order.Items,
		},
	})
}

// GetNextCounterTicket returns a preview of the next daily counter ticket (not reserved).
func (h *OrderHandler) GetNextCounterTicket(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	ticket, err := h.orderService.GetNextCounterTicket(restaurantID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ticket_number": ticket})
}

// ListCounterOrdersToday returns today's counter/takeaway orders.
func (h *OrderHandler) ListCounterOrdersToday(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	orders, err := h.orderService.ListCounterOrdersToday(restaurantID.(string), 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type itemResp struct {
		ID       string  `json:"id"`
		MenuID   string  `json:"menu_id"`
		Name     string  `json:"name"`
		Quantity int     `json:"quantity"`
		UnitRate float64 `json:"unit_rate"`
		Status   string  `json:"status"`
	}

	out := make([]gin.H, 0, len(orders))
	for _, order := range orders {
		items := make([]itemResp, 0, len(order.Items))
		for _, item := range order.Items {
			name := "Unknown Item"
			if item.MenuItem != nil {
				name = item.MenuItem.Name
			}
			items = append(items, itemResp{
				ID:       item.ID,
				MenuID:   item.MenuID,
				Name:     name,
				Quantity: item.Quantity,
				UnitRate: item.UnitRate,
				Status:   item.Status,
			})
		}
		ticket := order.TicketNumber
		if ticket == 0 {
			ticket = order.OrderNumber
		}
		tableID := ""
		if order.TableID != nil {
			tableID = *order.TableID
		}
		out = append(out, gin.H{
			"id":              order.ID,
			"order_number":    order.OrderNumber,
			"ticket_number":   ticket,
			"order_type":      order.OrderType,
			"service_mode":    order.ServiceMode,
			"table_number":    order.TableNumber,
			"table_id":        tableID,
			"customer_name":   order.CustomerName,
			"customer_phone":  order.CustomerPhone,
			"is_self_service": order.OrderType == "counter" || order.CustomerName == "Self Service",
			"status":          order.Status,
			"sub_total":       order.SubTotal,
			"tax_amount":      order.TaxAmount,
			"total":           order.Total,
			"created_at":      order.CreatedAt,
			"items":           items,
		})
	}

	c.JSON(http.StatusOK, gin.H{"orders": out, "total": len(out)})
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
			"customer_phone":  order.CustomerPhone,
			"order_number":    order.OrderNumber,
			"ticket_number":   order.TicketNumber,
			"order_type":      order.OrderType,
			"service_mode":    order.ServiceMode,
			"is_self_service": order.OrderType == "counter" || order.CustomerName == "Self Service",
			"status":          order.Status,
			"sub_total":       order.SubTotal,
			"tax_amount":      order.TaxAmount,
			"discount_amount": order.DiscountAmount,
			"total":           order.Total,
			"payment_method":  order.PaymentMethod,
			"amount_received": order.AmountReceived,
			"change_returned": order.ChangeReturned,
			"cash_amount":     order.CashAmount,
			"upi_amount":      order.UpiAmount,
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
	order, updatedIngredients, err := h.orderService.UpdateOrder(restaurantID.(string), orderID, req)
	if err != nil {
		log.Printf("❌ Order update failed: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Order updated: #%d", order.OrderNumber)

	// Broadcast order update event via WebSocket
	if globalHub != nil {
		BroadcastOrderEvent(globalHub, restaurantID.(string), "order_updated", order)
		BroadcastIngredientInventoryUpdates(globalHub, restaurantID.(string), updatedIngredients)
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
		CustomerPhone  string              `json:"customer_phone,omitempty"`
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
			CustomerPhone:  order.CustomerPhone,
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

// ListOrdersSummary returns lightweight active orders for table tiles and dashboards.
func (h *OrderHandler) ListOrdersSummary(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	status := c.DefaultQuery("status", "active")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	summaries, total, err := h.orderService.ListOrdersSummary(restaurantID.(string), status, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"orders": summaries,
		"total":  total,
		"limit":  limit,
	})
}

// GetSalesSummary returns aggregated revenue stats without loading full order history.
func (h *OrderHandler) GetSalesSummary(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	period := c.DefaultQuery("period", "today")
	summary, err := h.orderService.GetSalesSummary(restaurantID.(string), period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, summary)
}

// ListOrderHistory returns completed/paid orders for a date range (order history).
func (h *OrderHandler) ListOrderHistory(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	fromStr := c.Query("from")
	toStr := c.Query("to")
	orderType := c.DefaultQuery("order_type", "all")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit > 100 {
		limit = 100
	}

	from, toEnd, err := services.ParseHistoryDateRange(fromStr, toStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var restaurant models.Restaurant
	if err := h.orderService.GetDB().Where("id = ?", restaurantID.(string)).First(&restaurant).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load restaurant"})
		return
	}
	limits, err := services.LoadSubscriptionLimits(h.orderService.GetDB(), &restaurant)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load subscription"})
		return
	}
	from = services.ClampHistoryFrom(limits, from)

	switch orderType {
	case "all", "dine_in", "counter":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "order_type must be all, dine_in, or counter"})
		return
	}

	orders, total, err := h.orderService.ListOrderHistory(restaurantID.(string), from, toEnd, orderType, limit, offset)
	if err != nil {
		log.Printf("❌ Order history retrieval failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

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
		ID              string              `json:"id"`
		RestaurantID    string              `json:"restaurant_id"`
		TableNumber     string              `json:"table_number"`
		TableID         *string             `json:"table_id,omitempty"`
		CustomerName    string              `json:"customer_name"`
		CustomerPhone   string              `json:"customer_phone,omitempty"`
		OrderNumber     int                 `json:"order_number"`
		TicketNumber    int                 `json:"ticket_number"`
		OrderType       string              `json:"order_type"`
		ServiceMode     string              `json:"service_mode,omitempty"`
		Status          string              `json:"status"`
		SubTotal        float64             `json:"sub_total"`
		TaxAmount       float64             `json:"tax_amount"`
		DiscountAmount  float64             `json:"discount_amount"`
		Total           float64             `json:"total"`
		PaymentMethod   string              `json:"payment_method"`
		AmountReceived  float64             `json:"amount_received,omitempty"`
		ChangeReturned  float64             `json:"change_returned,omitempty"`
		CashAmount      float64             `json:"cash_amount,omitempty"`
		UpiAmount       float64             `json:"upi_amount,omitempty"`
		Notes           string              `json:"notes"`
		CreatedAt       time.Time           `json:"created_at"`
		UpdatedAt       time.Time           `json:"updated_at"`
		CompletedAt     *time.Time          `json:"completed_at,omitempty"`
		Items           []OrderItemResponse `json:"items"`
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
			if item.MenuItem != nil {
				itemResp.MenuItem = map[string]interface{}{
					"id":            item.MenuItem.ID,
					"name":          item.MenuItem.Name,
					"description":   item.MenuItem.Description,
					"price":         item.MenuItem.Price,
					"cost_price":    item.MenuItem.CostPrice,
					"is_veg":        item.MenuItem.IsVeg,
					"is_vegetarian": item.MenuItem.IsVeg,
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
			CustomerPhone:  order.CustomerPhone,
			OrderNumber:    order.OrderNumber,
			TicketNumber:   order.TicketNumber,
			OrderType:      order.OrderType,
			ServiceMode:    order.ServiceMode,
			Status:         order.Status,
			SubTotal:       order.SubTotal,
			TaxAmount:      order.TaxAmount,
			DiscountAmount: order.DiscountAmount,
			Total:          order.Total,
			PaymentMethod:  order.PaymentMethod,
			AmountReceived: order.AmountReceived,
			ChangeReturned: order.ChangeReturned,
			CashAmount:     order.CashAmount,
			UpiAmount:      order.UpiAmount,
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
		"from":   from.Format("2006-01-02"),
		"to":     toEnd.Add(-24 * time.Hour).Format("2006-01-02"),
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
		BroadcastOrderEvent(globalHub, restaurantID.(string), "order_completed", order)

		// If this is a dine-in order, also broadcast that the table is now vacant
		if order.TableID != nil && *order.TableID != "" {
			log.Printf("📍 Order #%d is dine-in, broadcasting table %s as vacant", order.OrderNumber, order.TableNumber)
			BroadcastTableUpdate(globalHub, restaurantID.(string), &models.RestaurantTable{
				ID:             *order.TableID,
				Name:           order.TableNumber,
				IsOccupied:     false,
				CurrentOrderID: nil,
			})
		}
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
		PaymentMethod  string  `json:"payment_method" binding:"required"` // cash | upi | split
		AmountReceived float64 `json:"amount_received,omitempty"`
		ChangeReturned float64 `json:"change_returned,omitempty"`
		CashAmount     float64 `json:"cash_amount,omitempty"`
		UpiAmount      float64 `json:"upi_amount,omitempty"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("❌ [Handler] JSON binding error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	log.Printf("   Payment Data: Method=%s, Received=%.2f, Change=%.2f", input.PaymentMethod, input.AmountReceived, input.ChangeReturned)

	// Validate payment method
	if input.PaymentMethod != "cash" && input.PaymentMethod != "upi" && input.PaymentMethod != "split" {
		log.Printf("❌ [Handler] Invalid payment method: %s", input.PaymentMethod)
		c.JSON(http.StatusBadRequest, gin.H{"error": "payment_method must be 'cash', 'upi', or 'split'"})
		return
	}

	// For cash payments, amount_received is required
	if input.PaymentMethod == "cash" && input.AmountReceived == 0 {
		log.Printf("❌ [Handler] Cash payment missing amount_received")
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount_received is required for cash payments"})
		return
	}

	if input.PaymentMethod == "split" {
		if input.CashAmount <= 0 || input.UpiAmount <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cash_amount and upi_amount are required for split payments"})
			return
		}
	}

	log.Printf("   → Calling service.CompleteOrderWithPayment...")

	// Complete the order with payment details
	order, err := h.orderService.CompleteOrderWithPayment(
		restaurantID.(string),
		orderID,
		services.OrderPaymentDetails{
			PaymentMethod:  input.PaymentMethod,
			AmountReceived: input.AmountReceived,
			ChangeReturned: input.ChangeReturned,
			CashAmount:     input.CashAmount,
			UpiAmount:      input.UpiAmount,
		},
	)

	if err != nil {
		log.Printf("❌ [Handler] Order payment completion failed: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ [Handler] Order #%d completed with %s payment. Response:", order.OrderNumber, input.PaymentMethod)

	if h.checkoutLock != nil {
		h.checkoutLock.ForceRelease(orderID)
	}

	// Broadcast payment / completion via WebSocket
	if globalHub != nil {
		eventType := "order_completed"
		if order.OrderType == "counter" {
			eventType = "order_updated"
		}
		BroadcastOrderEvent(globalHub, restaurantID.(string), eventType, order)

		// Dine-in only — table becomes vacant when order is fully completed
		if order.OrderType != "counter" && order.TableID != nil && *order.TableID != "" {
			log.Printf("📍 Order #%d is dine-in, broadcasting table %s as vacant", order.OrderNumber, order.TableNumber)
			BroadcastTableUpdate(globalHub, restaurantID.(string), &models.RestaurantTable{
				ID:             *order.TableID,
				Name:           order.TableNumber,
				IsOccupied:     false,
				CurrentOrderID: nil,
			})
		}
	}

	if order.OrderType == "counter" && order.TrackingToken != "" {
		NotifyOrderTrackingUpdate(h.orderService, orderID, restaurantID.(string))
	}

	resp := gin.H{
		"message": "Order completed successfully",
		"order": gin.H{
			"id":              order.ID,
			"order_number":    order.OrderNumber,
			"status":          order.Status,
			"total":           order.Total,
			"payment_method":  order.PaymentMethod,
			"amount_received": order.AmountReceived,
			"change_returned": order.ChangeReturned,
			"cash_amount":     order.CashAmount,
			"upi_amount":      order.UpiAmount,
			"completed_at":    order.CompletedAt,
		},
	}
	if order.OrderType == "counter" && order.TrackingToken != "" {
		ticket := order.TicketNumber
		if ticket <= 0 {
			ticket = order.OrderNumber
		}
		resp["tracking_token"] = order.TrackingToken
		resp["tracking_url"] = services.BuildTrackingURL(order.TrackingToken)
		resp["ticket_number"] = ticket
		orderResp := resp["order"].(gin.H)
		orderResp["ticket_number"] = ticket
		orderResp["tracking_token"] = order.TrackingToken
		orderResp["tracking_url"] = services.BuildTrackingURL(order.TrackingToken)
	}

	c.JSON(http.StatusOK, resp)
}

// StartCheckout acquires a checkout lock for an order (one device at a time).
func (h *OrderHandler) StartCheckout(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	orderID := c.Param("order_id")
	order, err := h.orderService.GetOrderByID(restaurantID.(string), orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if order.Status == "completed" || order.Status == "cancelled" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order is no longer active"})
		return
	}

	if h.checkoutLock == nil {
		c.JSON(http.StatusOK, gin.H{"message": "checkout started"})
		return
	}

	lock, err := h.checkoutLock.Acquire(orderID, restaurantID.(string), userID.(string))
	if err != nil {
		if errors.Is(err, services.ErrCheckoutInProgress) {
			lockedByName := "another staff member"
			lockedByUserID := ""
			if lock != nil {
				lockedByName = lock.UserName
				lockedByUserID = lock.UserID
			}
			c.JSON(http.StatusConflict, gin.H{
				"error":             "checkout already in progress on another device",
				"locked_by_name":    lockedByName,
				"locked_by_user_id": lockedByUserID,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	tableID := ""
	if order.TableID != nil {
		tableID = *order.TableID
	}
	if globalHub != nil {
		BroadcastCheckoutEvent(globalHub, restaurantID.(string), "checkout_started", models.CheckoutEventData{
			OrderID:        orderID,
			TableID:        tableID,
			LockedByUserID: lock.UserID,
			LockedByName:   lock.UserName,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "checkout started",
		"locked_by_name": lock.UserName,
	})
}

// CancelCheckout releases a checkout lock when staff leaves checkout without paying.
func (h *OrderHandler) CancelCheckout(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	orderID := c.Param("order_id")
	if h.checkoutLock != nil {
		h.checkoutLock.Release(orderID, userID.(string))
	}

	if globalHub != nil {
		BroadcastCheckoutEvent(globalHub, restaurantID.(string), "checkout_cancelled", models.CheckoutEventData{
			OrderID:        orderID,
			LockedByUserID: userID.(string),
		})
	}

	c.JSON(http.StatusOK, gin.H{"message": "checkout cancelled"})
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

	user, err := h.authService.GetUserByID(userID.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}
	if !services.UserCanCancelOrders(user) {
		c.JSON(http.StatusForbidden, gin.H{"error": "you do not have permission to cancel orders"})
		return
	}

	orderID := c.Param("order_id")

	restoredIngredients, err := h.orderService.CancelOrder(restaurantID.(string), orderID)
	if err != nil {
		log.Printf("❌ Order cancellation failed: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Order cancelled: %s, Inventory restored", orderID)

	// Fetch cancelled order and broadcast the cancellation
	order, err := h.orderService.GetOrderByID(restaurantID.(string), orderID)
	if err == nil && globalHub != nil {
		BroadcastOrderEvent(globalHub, restaurantID.(string), "order_cancelled", order)
		BroadcastIngredientInventoryUpdates(globalHub, restaurantID.(string), restoredIngredients)

		if order.TableID != nil && *order.TableID != "" {
			BroadcastTableUpdate(globalHub, restaurantID.(string), &models.RestaurantTable{
				ID:             *order.TableID,
				Name:           order.TableNumber,
				IsOccupied:     false,
				CurrentOrderID: nil,
			})
		}
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

	log.Printf("🔵 [UpdateOrderItemStatus] Called with orderID=%s, itemID=%s, restaurantID=%v", orderID, itemID, restaurantID)

	var input struct {
		Status string `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("❌ Binding error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("   New status: %s", input.Status)

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

	if err := services.EnforceKitchenUpdate(h.orderService.GetDB(), restaurantID.(string), orderID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
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
		if completedOrder, didComplete, completeErr := h.orderService.TryCompleteCounterOrderAfterKitchen(restaurantID.(string), orderID); completeErr == nil && didComplete {
			BroadcastOrderEvent(globalHub, restaurantID.(string), "order_completed", completedOrder)
			NotifyOrderTrackingUpdate(h.orderService, orderID, restaurantID.(string))
		} else {
			BroadcastOrderItemStatusEvent(globalHub, restaurantID.(string), updatedOrder, itemID, "", false)
		}
		// Table stays occupied until order is completed/paid — do not auto-vacate when all items served
		NotifyOrderTrackingUpdate(h.orderService, orderID, restaurantID.(string))
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

	if err := services.EnforceKitchenUpdate(h.orderService.GetDB(), restaurantID.(string), orderID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	err := h.orderService.UpdateOrderItemsByMenuID(restaurantID.(string), orderID, menuItemID, input.Status)
	if err != nil {
		log.Printf("❌ Order items status update failed: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Order items with menu_id %s updated to status: %s", menuItemID, input.Status)

	// Broadcast with full order context
	if globalHub != nil {
		updatedOrder, fetchErr := h.orderService.GetOrderByID(restaurantID.(string), orderID)
		if fetchErr == nil {
			if completedOrder, didComplete, completeErr := h.orderService.TryCompleteCounterOrderAfterKitchen(restaurantID.(string), orderID); completeErr == nil && didComplete {
				BroadcastOrderEvent(globalHub, restaurantID.(string), "order_completed", completedOrder)
			} else {
				BroadcastOrderItemStatusEvent(globalHub, restaurantID.(string), updatedOrder, "", menuItemID, true)
			}
			NotifyOrderTrackingUpdate(h.orderService, orderID, restaurantID.(string))
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Order items status updated successfully",
		"menu_id":  menuItemID,
		"order_id": orderID,
		"status":   input.Status,
	})
}

// CreateBillShare generates a customer bill link and QR token for review/download.
func (h *OrderHandler) CreateBillShare(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	orderID := c.Param("order_id")
	var input struct {
		DiscountAmount float64 `json:"discount_amount"`
	}
	if err := c.ShouldBindJSON(&input); err != nil && err.Error() != "EOF" {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	order, err := h.orderService.CreateBillShare(restaurantID.(string), orderID, input.DiscountAmount)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"bill_token": order.BillToken,
		"bill_url":   services.BuildBillURL(order.BillToken),
		"expires_at": order.BillExpiresAt,
	})
}
