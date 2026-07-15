package handlers

import (
	"net/http"
	"strconv"

	"restaurant-api/internal/services"

	"github.com/gin-gonic/gin"
)

type SupportIssueHandler struct {
	service *services.SupportIssueService
}

func NewSupportIssueHandler(service *services.SupportIssueService) *SupportIssueHandler {
	return &SupportIssueHandler{service: service}
}

func (h *SupportIssueHandler) CreateIssue(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant_id not found in context"})
		return
	}

	userID, _ := c.Get("user_id")
	role, _ := c.Get("role")

	var req services.CreateSupportIssueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	issue, err := h.service.CreateIssue(
		restaurantID.(string),
		stringValue(userID),
		stringValue(role),
		req,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Support issue submitted",
		"issue":   issue,
	})
}

func (h *SupportIssueHandler) ListRestaurantIssues(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant_id not found in context"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	issues, total, err := h.service.ListRestaurantIssues(
		restaurantID.(string),
		c.Query("status"),
		limit,
		offset,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"issues": issues,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (h *SupportIssueHandler) ListPlatformIssues(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	issues, total, err := h.service.ListPlatformIssues(
		c.Query("status"),
		c.Query("search"),
		limit,
		offset,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"issues": issues,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (h *SupportIssueHandler) UpdatePlatformIssue(c *gin.Context) {
	var req services.UpdateSupportIssueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	issue, err := h.service.UpdateIssue(c.Param("issue_id"), req, h.platformActor(c))
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "support issue not found" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Support issue updated",
		"issue":   issue,
	})
}

func (h *SupportIssueHandler) platformActor(c *gin.Context) string {
	if actor, ok := c.Get("platform_actor"); ok {
		if s, ok := actor.(string); ok && s != "" {
			return s
		}
	}
	return "platform_ops"
}

func stringValue(value interface{}) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}
