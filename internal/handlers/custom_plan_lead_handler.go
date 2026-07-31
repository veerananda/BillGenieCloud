package handlers

import (
	"net/http"
	"strconv"

	"restaurant-api/internal/services"

	"github.com/gin-gonic/gin"
)

type CustomPlanLeadHandler struct {
	service *services.CustomPlanLeadService
}

func NewCustomPlanLeadHandler(service *services.CustomPlanLeadService) *CustomPlanLeadHandler {
	return &CustomPlanLeadHandler{service: service}
}

func (h *CustomPlanLeadHandler) CreateLead(c *gin.Context) {
	var req services.CreateCustomPlanLeadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	lead, err := h.service.CreateLead(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Thanks — BillGenie will contact you shortly.",
		"lead":    lead,
	})
}

func (h *CustomPlanLeadHandler) ListPlatformLeads(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	leads, total, err := h.service.ListLeads(
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
		"leads":  leads,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (h *CustomPlanLeadHandler) UpdatePlatformLead(c *gin.Context) {
	var req services.UpdateCustomPlanLeadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	actor, _ := c.Get("platform_actor")
	actorStr, _ := actor.(string)
	if actorStr == "" {
		actorStr = "platform"
	}

	lead, err := h.service.UpdateLead(c.Param("lead_id"), req, actorStr)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "custom plan lead not found" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Lead updated",
		"lead":    lead,
	})
}
