package handlers

import (
	"log"
	"net/http"
	"strings"

	"restaurant-api/internal/middleware"
	"restaurant-api/internal/models"
	"restaurant-api/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PushHandler struct {
	pushService *services.PushService
}

func NewPushHandler(pushService *services.PushService) *PushHandler {
	return &PushHandler{pushService: pushService}
}

// SetupPushRoutes registers device push-token endpoints for staff apps.
func SetupPushRoutes(router *gin.Engine, db *gorm.DB) {
	authService := getAuthService(db)
	h := NewPushHandler(services.NewPushService(db))

	protected := router.Group("/devices")
	protected.Use(middleware.AuthMiddleware(authService))
	{
		protected.POST("/push-token", h.UpsertPushToken)
		protected.DELETE("/push-token", h.DeletePushToken)
	}
	log.Println("✅ Device push-token routes registered")
}

func (h *PushHandler) UpsertPushToken(c *gin.Context) {
	userID, _ := c.Get("user_id")
	restaurantID, _ := c.Get("restaurant_id")
	var input services.UpsertPushTokenInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.pushService.UpsertToken(userID.(string), restaurantID.(string), input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

func (h *PushHandler) DeletePushToken(c *gin.Context) {
	userID, _ := c.Get("user_id")
	token := strings.TrimSpace(c.Query("token"))
	if token == "" {
		var body struct {
			Token string `json:"token"`
		}
		_ = c.ShouldBindJSON(&body)
		token = strings.TrimSpace(body.Token)
	}
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token required"})
		return
	}
	_ = h.pushService.DeleteToken(userID.(string), token)
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// notifyStaffPush is a fire-and-forget helper for kitchen/assistance alerts.
func notifyStaffPush(db *gorm.DB, restaurantID, alertType, title, body string, data map[string]string) {
	if db == nil || strings.TrimSpace(restaurantID) == "" {
		return
	}
	services.NewPushService(db).NotifyRestaurantStaff(restaurantID, alertType, title, body, data)
}

func notifyOrderItemStatusPush(db *gorm.DB, restaurantID string, order *models.Order, status string) {
	if order == nil {
		return
	}
	label := strings.TrimSpace(order.TableNumber)
	if label == "" {
		label = "an order"
	} else if label != "Counter" && label != "Takeaway" {
		label = "Table " + label
	}
	switch status {
	case "ready":
		notifyStaffPush(db, restaurantID, services.PushAlertItemsReady,
			"Items ready",
			label+" has items ready for pickup",
			map[string]string{"order_id": order.ID, "status": status},
		)
	case "cancelled":
		notifyStaffPush(db, restaurantID, services.PushAlertItemCancelled,
			"Item cancelled",
			"Kitchen cancelled an item on "+label,
			map[string]string{"order_id": order.ID, "status": status},
		)
	}
}
