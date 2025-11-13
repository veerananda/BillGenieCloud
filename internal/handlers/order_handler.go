package handlers

import (
	"log"
	"net/http"
	"strconv"

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

	c.JSON(http.StatusCreated, gin.H{
		"message": "Order created successfully with inventory deducted",
		"order": gin.H{
			"id":           order.ID,
			"order_number": order.OrderNumber,
			"table_number": order.TableNumber,
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

	c.JSON(http.StatusOK, gin.H{
		"order": order,
	})
}

// ListOrders retrieves all orders for a restaurant
// @Summary List orders
// @Description Get paginated list of orders
// @Security ApiKeyAuth
// @Produce json
// @Param status query string false "Filter by status (pending, completed, cancelled)"
// @Param limit query int false "Items per page (default: 20)"
// @Param offset query int false "Pagination offset (default: 0)"
// @Success 200 {object} map[string]interface{}
// @Router /orders [get]
func (h *OrderHandler) ListOrders(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
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

	c.JSON(http.StatusOK, gin.H{
		"orders": orders,
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

	c.JSON(http.StatusOK, gin.H{
		"message":  "Order cancelled and inventory restored",
		"order_id": orderID,
	})
}
