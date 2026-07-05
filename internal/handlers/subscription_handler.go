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

type subscriptionSelectionBody struct {
	Selection services.SubscriptionSelection `json:"selection"`
}

// GetRenewalQuote returns the renewal amount for the restaurant's current or selected plan.
func (h *SubscriptionHandler) GetRenewalQuote(c *gin.Context) {
	restaurantID, _ := c.Get("restaurant_id")
	var body subscriptionSelectionBody
	_ = c.ShouldBindJSON(&body)
	var override *services.SubscriptionSelection
	if body.Selection.OperationMode != "" {
		override = &body.Selection
	}
	quote, err := h.renewalService.GetRenewalQuote(restaurantID.(string), override)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load renewal quote"})
		return
	}
	c.JSON(http.StatusOK, quote)
}

// QuoteSignupPlan returns pricing for a plan before registration (public).
func (h *SubscriptionHandler) QuoteSignupPlan(c *gin.Context) {
	var body subscriptionSelectionBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	quote, err := h.renewalService.QuoteForSelection(body.Selection)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, quote)
}

// CreateRenewalOrder creates a Razorpay order for subscription renewal or activation.
func (h *SubscriptionHandler) CreateRenewalOrder(c *gin.Context) {
	restaurantID, _ := c.Get("restaurant_id")
	var body subscriptionSelectionBody
	_ = c.ShouldBindJSON(&body)
	var override *services.SubscriptionSelection
	if body.Selection.OperationMode != "" {
		override = &body.Selection
	}
	result, err := h.renewalService.CreateRenewalOrder(restaurantID.(string), override)
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
