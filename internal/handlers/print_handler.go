package handlers

import (
	"log"
	"net/http"
	"strings"

	"restaurant-api/internal/middleware"
	"restaurant-api/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PrintHandler struct {
	printService *services.PrintService
}

func NewPrintHandler(printService *services.PrintService) *PrintHandler {
	return &PrintHandler{printService: printService}
}

// SetupPrintRoutes registers restaurant print settings + print-agent poll APIs.
func SetupPrintRoutes(router *gin.Engine, db *gorm.DB) {
	printService := services.NewPrintService(db)
	h := NewPrintHandler(printService)
	authService := getAuthService(db)

	restaurant := router.Group("/restaurants")
	restaurant.Use(middleware.AuthMiddleware(authService))
	restaurant.Use(withSubscription(db))
	{
		restaurant.GET("/print-settings", middleware.RoleMiddleware("admin", "manager"), h.GetPrintSettings)
		restaurant.PUT("/print-settings", middleware.RoleMiddleware("admin", "manager"), h.UpdatePrintSettings)
		restaurant.POST("/print-settings/rotate-agent-key", middleware.RoleMiddleware("admin"), h.RotateAgentKey)
	}

	orders := router.Group("/orders")
	orders.Use(middleware.AuthMiddleware(authService))
	orders.Use(withSubscription(db))
	{
		orders.POST("/:order_id/print-bill", h.EnqueueBillPrint)
	}

	agent := router.Group("/print-agent")
	agent.Use(middleware.PrintAgentAuthMiddleware(printService))
	{
		agent.POST("/jobs/claim", h.ClaimJobs)
		agent.POST("/jobs/:job_id/complete", h.CompleteJob)
		agent.POST("/jobs/:job_id/fail", h.FailJob)
	}

	log.Println("✅ Print agent routes registered")
}

func (h *PrintHandler) GetPrintSettings(c *gin.Context) {
	restaurantID, _ := c.Get("restaurant_id")
	settings, err := h.printService.GetOrCreateSettings(restaurantID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"settings": settings,
		"has_agent_key": settings.AgentAPIKeyHash != "",
	})
}

func (h *PrintHandler) UpdatePrintSettings(c *gin.Context) {
	restaurantID, _ := c.Get("restaurant_id")
	var input services.UpdatePrintSettingsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	settings, err := h.printService.UpdateSettings(restaurantID.(string), input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"settings": settings, "has_agent_key": settings.AgentAPIKeyHash != ""})
}

func (h *PrintHandler) RotateAgentKey(c *gin.Context) {
	restaurantID, _ := c.Get("restaurant_id")
	plaintext, settings, err := h.printService.RotateAgentAPIKey(restaurantID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"agent_api_key": plaintext,
		"settings":      settings,
		"has_agent_key": true,
		"message":       "Copy this key into the print agent now. It will not be shown again.",
	})
}

func (h *PrintHandler) EnqueueBillPrint(c *gin.Context) {
	restaurantID, _ := c.Get("restaurant_id")
	orderID := c.Param("order_id")
	order, err := h.printService.LoadOrderForEnqueue(restaurantID.(string), orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}
	h.printService.EnqueueBillForOrder(order)
	c.JSON(http.StatusAccepted, gin.H{"message": "bill print queued if enabled"})
}

func (h *PrintHandler) ClaimJobs(c *gin.Context) {
	restaurantID, _ := c.Get("restaurant_id")
	var body struct {
		AgentID string `json:"agent_id"`
		Limit   int    `json:"limit"`
	}
	_ = c.ShouldBindJSON(&body)
	agentID := strings.TrimSpace(body.AgentID)
	if agentID == "" {
		agentID = c.ClientIP()
	}
	jobs, err := h.printService.ClaimPendingJobs(restaurantID.(string), agentID, body.Limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"jobs": jobs})
}

func (h *PrintHandler) CompleteJob(c *gin.Context) {
	restaurantID, _ := c.Get("restaurant_id")
	jobID := c.Param("job_id")
	if err := h.printService.CompleteJob(restaurantID.(string), jobID, false, ""); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

func (h *PrintHandler) FailJob(c *gin.Context) {
	restaurantID, _ := c.Get("restaurant_id")
	jobID := c.Param("job_id")
	var body struct {
		Error string `json:"error"`
	}
	_ = c.ShouldBindJSON(&body)
	if err := h.printService.CompleteJob(restaurantID.(string), jobID, true, body.Error); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}
