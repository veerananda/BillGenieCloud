package handlers

import (
	"io"
	"log"
	"net/http"
	"strings"

	"restaurant-api/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SubscriptionWebhookHandler struct {
	renewalService *services.SubscriptionRenewalService
	razorpay       *services.RazorpayService
}

func NewSubscriptionWebhookHandler(db *gorm.DB) *SubscriptionWebhookHandler {
	return &SubscriptionWebhookHandler{
		renewalService: services.NewSubscriptionRenewalService(db),
		razorpay:       services.NewRazorpayService(),
	}
}

// HandleRazorpayWebhook processes Razorpay payment webhooks for subscription orders.
func (h *SubscriptionWebhookHandler) HandleRazorpayWebhook(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	signature := strings.TrimSpace(c.GetHeader("X-Razorpay-Signature"))
	if !h.razorpay.VerifyWebhookSignature(body, signature) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid webhook signature"})
		return
	}

	if err := h.renewalService.ProcessRazorpayWebhook(body); err != nil {
		log.Printf("razorpay webhook processing failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "webhook processing failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
