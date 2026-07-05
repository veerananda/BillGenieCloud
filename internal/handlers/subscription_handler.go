package handlers

import (
	"net/http"

	"restaurant-api/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SubscriptionHandler struct {
	renewalService *services.SubscriptionRenewalService
}

func NewSubscriptionHandler(db *gorm.DB) *SubscriptionHandler {
	return &SubscriptionHandler{
		renewalService: services.NewSubscriptionRenewalService(db),
	}
}

// GetRenewalQuote returns the renewal amount for the restaurant's current plan.
func (h *SubscriptionHandler) GetRenewalQuote(c *gin.Context) {
	restaurantID, _ := c.Get("restaurant_id")
	quote, err := h.renewalService.GetRenewalQuote(restaurantID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load renewal quote"})
		return
	}
	c.JSON(http.StatusOK, quote)
}

// CreateRenewalOrder creates a Razorpay order for subscription renewal.
func (h *SubscriptionHandler) CreateRenewalOrder(c *gin.Context) {
	restaurantID, _ := c.Get("restaurant_id")
	result, err := h.renewalService.CreateRenewalOrder(restaurantID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// VerifyRenewalPayment verifies Razorpay payment and extends subscription_end.
func (h *SubscriptionHandler) VerifyRenewalPayment(c *gin.Context) {
	restaurantID, _ := c.Get("restaurant_id")

	var req services.VerifyRenewalPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.renewalService.VerifyRenewalPayment(restaurantID.(string), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
