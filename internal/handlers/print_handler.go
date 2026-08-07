package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

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

// SetupPrintRoutes registers restaurant print settings + print-agent claim/SSE APIs.
func SetupPrintRoutes(router *gin.Engine, db *gorm.DB) {
	printService := services.NewPrintService(db)
	h := NewPrintHandler(printService)
	authService := getAuthService(db)

	restaurant := router.Group("/restaurants")
	restaurant.Use(middleware.AuthMiddleware(authService))
	restaurant.Use(withSubscription(db))
	{
		// All restaurant roles can read settings; staff/chef configure hosts when printing is enabled.
		restaurant.GET("/print-settings", middleware.RoleMiddleware("admin", "manager", "staff", "chef"), h.GetPrintSettings)
		restaurant.PUT("/print-settings", middleware.RoleMiddleware("admin", "manager", "staff", "chef"), h.UpdatePrintSettings)
		restaurant.POST("/print-settings/rotate-agent-key", middleware.RoleMiddleware("admin"), h.RotateAgentKey)
		restaurant.POST("/print-settings/test", middleware.RoleMiddleware("admin", "manager", "staff", "chef"), h.EnqueueTestPrint)
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
		agent.GET("/events", h.AgentEvents)
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
	roleVal, _ := c.Get("role")
	role, _ := roleVal.(string)
	var input services.UpdatePrintSettingsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	isManager := role == "admin" || role == "manager"
	// Only admin/manager may flip master enable toggles or paper feed.
	if !isManager {
		input.BillPrintingEnabled = nil
		input.KotPrintingEnabled = nil
		input.BillAutoPrintOnCheckout = nil
		input.TopFeedLines = nil
		input.BottomFeedLines = nil
	}

	current, err := h.printService.GetOrCreateSettings(restaurantID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Staff/chef may only write hosts/paper when the matching print feature is enabled.
	if !isManager {
		if !current.KotPrintingEnabled {
			input.KotPrinterHost = nil
			input.KotPrinterPort = nil
			input.KotPaperWidthMm = nil
		}
		if !current.BillPrintingEnabled {
			input.BillPrinterHost = nil
			input.BillPrinterPort = nil
			input.BillPaperWidthMm = nil
		}
		if input.KotPrinterHost == nil && input.KotPrinterPort == nil &&
			input.BillPrinterHost == nil && input.BillPrinterPort == nil &&
			input.KotPaperWidthMm == nil && input.BillPaperWidthMm == nil {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "printing is disabled — ask an admin or manager to enable KOT or bill printing first",
			})
			return
		}
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

func (h *PrintHandler) EnqueueTestPrint(c *gin.Context) {
	restaurantID, _ := c.Get("restaurant_id")
	roleVal, _ := c.Get("role")
	role, _ := roleVal.(string)
	var body struct {
		Target string `json:"target"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	target := strings.ToLower(strings.TrimSpace(body.Target))
	if target != "kot" && target != "bill" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target must be kot or bill"})
		return
	}

	settings, err := h.printService.GetOrCreateSettings(restaurantID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	isManager := role == "admin" || role == "manager"
	if !isManager {
		if target == "kot" && !settings.KotPrintingEnabled {
			c.JSON(http.StatusForbidden, gin.H{"error": "KOT printing is disabled — ask an admin or manager to enable it"})
			return
		}
		if target == "bill" && !settings.BillPrintingEnabled {
			c.JSON(http.StatusForbidden, gin.H{"error": "Bill printing is disabled — ask an admin or manager to enable it"})
			return
		}
	}

	queued, err := h.printService.EnqueueTestPrint(restaurantID.(string), target)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "queued": false})
		return
	}
	if !queued {
		label := "KOT"
		if target == "bill" {
			label = "bill"
		}
		c.JSON(http.StatusOK, gin.H{
			"queued":  false,
			"message": label + " printer host is not set — save a Wi‑Fi / LAN IP first",
		})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{
		"queued":  true,
		"message": "Test print queued. Keep the print agent running.",
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
	queued, err := h.printService.EnqueueBillForOrder(order)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "queued": false})
		return
	}
	if !queued {
		c.JSON(http.StatusOK, gin.H{
			"queued":  false,
			"message": "bill print not queued (disabled or bill printer host not set)",
		})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"queued": true, "message": "bill print queued"})
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

// AgentEvents is an SSE stream that wakes the on-site print agent when jobs are queued.
func (h *PrintHandler) AgentEvents(c *gin.Context) {
	restaurantID, _ := c.Get("restaurant_id")
	rid, _ := restaurantID.(string)
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// Immediate wake so agents catch jobs queued while disconnected.
	fmt.Fprintf(c.Writer, "event: jobs\ndata: {\"ready\":true}\n\n")
	flusher.Flush()

	hub := h.printService.NotifyHub()
	if hub == nil {
		ticker := time.NewTicker(25 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-c.Request.Context().Done():
				return
			case <-ticker.C:
				fmt.Fprintf(c.Writer, ": keepalive\n\n")
				flusher.Flush()
			}
		}
	}

	ch := hub.Subscribe(rid)
	defer hub.Unsubscribe(rid, ch)

	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case _, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(c.Writer, "event: jobs\ndata: {\"ready\":true}\n\n")
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(c.Writer, ": keepalive\n\n")
			flusher.Flush()
		}
	}
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
