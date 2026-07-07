package handlers

import (
	"net/http"
	"strconv"

	"restaurant-api/internal/services"

	"github.com/gin-gonic/gin"
)

type PlatformHandler struct {
	ops *services.PlatformOpsService
}

func NewPlatformHandler(ops *services.PlatformOpsService) *PlatformHandler {
	return &PlatformHandler{ops: ops}
}

func (h *PlatformHandler) platformActor(c *gin.Context) string {
	if actor, ok := c.Get("platform_actor"); ok {
		if s, ok := actor.(string); ok && s != "" {
			return s
		}
	}
	return "platform_ops"
}

// ListRestaurants returns all registered restaurants for BillGenie creators.
func (h *PlatformHandler) ListRestaurants(c *gin.Context) {
	search := c.Query("search")
	phase := c.Query("phase")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	items, total, err := h.ops.ListRestaurants(search, phase, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"restaurants": items,
		"total":       total,
		"limit":       limit,
		"offset":      offset,
	})
}

// GetRestaurant returns full tenant detail for the platform console.
func (h *PlatformHandler) GetRestaurant(c *gin.Context) {
	detail, err := h.ops.GetRestaurant(c.Param("restaurant_id"))
	if err != nil {
		if err.Error() == "restaurant not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"restaurant": detail})
}

// GrantSubscription activates or extends a paid plan without Razorpay (comp / pilot).
func (h *PlatformHandler) GrantSubscription(c *gin.Context) {
	var req services.GrantSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	restaurant, err := h.ops.GrantSubscription(c.Param("restaurant_id"), req, h.platformActor(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message":    "Subscription granted",
		"restaurant": h.ops.BuildSummaryPublic(restaurant),
	})
}

// ExtendTrial extends the free trial period.
func (h *PlatformHandler) ExtendTrial(c *gin.Context) {
	var req services.ExtendTrialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	restaurant, err := h.ops.ExtendTrial(c.Param("restaurant_id"), req, h.platformActor(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message":    "Trial extended",
		"restaurant": h.ops.BuildSummaryPublic(restaurant),
	})
}

// UpdateSelection updates plan add-ons and limits without changing billing dates.
func (h *PlatformHandler) UpdateSelection(c *gin.Context) {
	var req services.UpdateSelectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	restaurant, err := h.ops.UpdateSelection(c.Param("restaurant_id"), req, h.platformActor(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message":    "Plan selection updated",
		"restaurant": h.ops.BuildSummaryPublic(restaurant),
	})
}

// SetActive suspends or reactivates a restaurant tenant.
func (h *PlatformHandler) SetActive(c *gin.Context) {
	var req services.SetActiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	restaurant, err := h.ops.SetActive(c.Param("restaurant_id"), req, h.platformActor(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	msg := "Restaurant suspended"
	if req.IsActive {
		msg = "Restaurant reactivated"
	}
	c.JSON(http.StatusOK, gin.H{
		"message":    msg,
		"restaurant": h.ops.BuildSummaryPublic(restaurant),
	})
}
