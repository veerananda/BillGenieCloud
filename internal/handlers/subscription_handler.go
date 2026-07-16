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

// QuotePlanChange returns upgrade/downgrade pricing for an active paid subscription.
func (h *SubscriptionHandler) QuotePlanChange(c *gin.Context) {
	restaurantID, _ := c.Get("restaurant_id")
	var body subscriptionSelectionBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	quote, err := h.renewalService.QuotePlanChange(restaurantID.(string), body.Selection)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, quote)
}

// CreatePlanChangeOrder creates a Razorpay order for a mid-cycle upgrade.
func (h *SubscriptionHandler) CreatePlanChangeOrder(c *gin.Context) {
	restaurantID, _ := c.Get("restaurant_id")
	var body subscriptionSelectionBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.renewalService.CreatePlanChangeOrder(restaurantID.(string), body.Selection)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// VerifyPlanChangePayment verifies upgrade payment (same path as renewal verify).
func (h *SubscriptionHandler) VerifyPlanChangePayment(c *gin.Context) {
	h.VerifyRenewalPayment(c)
}

// SchedulePlanChange schedules a downgrade at period end.
func (h *SubscriptionHandler) SchedulePlanChange(c *gin.Context) {
	restaurantID, _ := c.Get("restaurant_id")
	var body subscriptionSelectionBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.renewalService.SchedulePlanChange(restaurantID.(string), body.Selection)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// CancelScheduledPlanChange clears a pending downgrade.
func (h *SubscriptionHandler) CancelScheduledPlanChange(c *gin.Context) {
	restaurantID, _ := c.Get("restaurant_id")
	if err := h.renewalService.CancelScheduledPlanChange(restaurantID.(string)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Scheduled plan change cancelled"})
}
