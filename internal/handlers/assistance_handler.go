package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log"
	"net/http"
	"strings"
	"time"

	"restaurant-api/internal/middleware"
	"restaurant-api/internal/models"
	"restaurant-api/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var globalAssistanceHub *services.AssistanceHub

// SetAssistanceHub configures the in-memory SSE hub for customer assistance pages.
func SetAssistanceHub(hub *services.AssistanceHub) {
	globalAssistanceHub = hub
}

// SetupAssistanceRoutes registers public customer table-session pages (no auth).
func SetupAssistanceRoutes(router *gin.Engine, db *gorm.DB) {
	handler := &AssistanceHandler{
		db:           db,
		orderService: services.NewOrderService(db),
		hub:          globalAssistanceHub,
	}
	callWaiterLimit := middleware.RateLimit(20, 15*time.Minute)
	router.GET("/a/:token", handler.AssistancePage)
	router.GET("/a/:token/status", handler.AssistanceStatus)
	router.GET("/a/:token/events", handler.AssistanceEvents)
	router.GET("/a/:token/menu", handler.AssistanceMenu)
	router.POST("/a/:token/call-waiter", callWaiterLimit, handler.CallWaiter)
	router.POST("/a/:token/unlock", callWaiterLimit, handler.UnlockWaiterSession)
	log.Println("✅ Customer table session routes registered at /a/:token")
}

type AssistanceHandler struct {
	db           *gorm.DB
	orderService *services.OrderService
	hub          *services.AssistanceHub
}

func (h *AssistanceHandler) loadStatus(token string) (*services.AssistanceStatus, *models.RestaurantTable, int, string) {
	table, err := services.ResolveTableByAssistanceToken(h.db, token)
	if err != nil {
		return nil, nil, http.StatusNotFound, "Table link not found."
	}
	if err := services.EnsureTableAssistanceToken(h.db, table); err != nil {
		return nil, nil, http.StatusInternalServerError, "Could not open table session."
	}
	status, err := services.BuildAssistanceStatusForTable(h.db, h.orderService, table)
	if err != nil || status == nil {
		return nil, nil, http.StatusInternalServerError, "Could not load table status."
	}
	return status, table, http.StatusOK, ""
}

func (h *AssistanceHandler) AssistancePage(c *gin.Context) {
	token := c.Param("token")
	status, table, code, message := h.loadStatus(token)
	if status == nil {
		c.Data(code, "text/html; charset=utf-8", []byte(assistanceErrorHTML(message)))
		return
	}
	// Prefer the permanent table token in the page so SSE/API calls stay on the fixed QR.
	pageToken := token
	if table != nil && table.AssistanceToken != nil && strings.TrimSpace(*table.AssistanceToken) != "" {
		pageToken = *table.AssistanceToken
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(renderAssistancePageHTML(pageToken, *status)))
}

func (h *AssistanceHandler) AssistanceStatus(c *gin.Context) {
	token := c.Param("token")
	status, _, code, message := h.loadStatus(token)
	if status == nil {
		c.JSON(code, gin.H{"error": message})
		return
	}
	c.JSON(http.StatusOK, status)
}

func (h *AssistanceHandler) AssistanceMenu(c *gin.Context) {
	token := c.Param("token")
	table, err := services.ResolveTableByAssistanceToken(h.db, token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Table link not found."})
		return
	}
	items, err := services.LoadAssistanceMenu(h.db, table.RestaurantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not load menu"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *AssistanceHandler) AssistanceEvents(c *gin.Context) {
	token := c.Param("token")
	status, table, code, message := h.loadStatus(token)
	if status == nil {
		c.JSON(code, gin.H{"error": message})
		return
	}

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	writeSSE := func(payload services.AssistanceStatus) {
		data, _ := json.Marshal(payload)
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		flusher.Flush()
	}

	writeSSE(*status)

	subscribeToken := token
	if table != nil && table.AssistanceToken != nil && strings.TrimSpace(*table.AssistanceToken) != "" {
		subscribeToken = *table.AssistanceToken
	}

	if h.hub == nil {
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

	ch := h.hub.Subscribe(subscribeToken)
	defer h.hub.Unsubscribe(subscribeToken, ch)

	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case next, ok := <-ch:
			if !ok {
				return
			}
			writeSSE(next)
		case <-ticker.C:
			fmt.Fprintf(c.Writer, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

func (h *AssistanceHandler) UnlockWaiterSession(c *gin.Context) {
	token := c.Param("token")
	table, err := services.ResolveTableByAssistanceToken(h.db, token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Table link not found."})
		return
	}
	if !table.IsOccupied || table.CurrentOrderID == nil || strings.TrimSpace(*table.CurrentOrderID) == "" {
		status, _ := services.BuildAssistanceStatusForTable(h.db, h.orderService, table)
		c.JSON(http.StatusConflict, gin.H{
			"error":  "No active seating at this table yet.",
			"status": status,
		})
		return
	}

	var body struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Enter the 4-digit code from staff."})
		return
	}
	want := services.DeriveAssistanceUnlockCode(*table.CurrentOrderID)
	got := strings.TrimSpace(body.Code)
	if want == "" || got != want {
		c.JSON(http.StatusForbidden, gin.H{"error": "Incorrect code. Ask staff for the current table code."})
		return
	}

	sess, err := services.IssueWaiterSession(table.ID, *table.CurrentOrderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not unlock call waiter."})
		return
	}
	status, _ := services.BuildAssistanceStatusForTable(h.db, h.orderService, table)
	if status != nil {
		status.WaiterSession = sess
		status.UnlockRequired = false
	}
	c.JSON(http.StatusOK, gin.H{
		"message":        "Unlocked",
		"waiter_session": sess,
		"status":         status,
	})
}

func (h *AssistanceHandler) CallWaiter(c *gin.Context) {
	token := c.Param("token")
	table, err := services.ResolveTableByAssistanceToken(h.db, token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Table link not found."})
		return
	}
	if !table.IsOccupied || table.CurrentOrderID == nil || strings.TrimSpace(*table.CurrentOrderID) == "" {
		status, _ := services.BuildAssistanceStatusForTable(h.db, h.orderService, table)
		if status != nil {
			publishAssistanceStatusForTable(table, *status)
		}
		c.JSON(http.StatusConflict, gin.H{
			"error":  "Call waiter is only available while you are seated with an active order.",
			"status": status,
		})
		return
	}

	var body struct {
		WaiterSession string `json:"waiter_session"`
	}
	_ = c.ShouldBindJSON(&body)
	sessionTok := strings.TrimSpace(body.WaiterSession)
	if sessionTok == "" {
		sessionTok = strings.TrimSpace(c.GetHeader("X-Waiter-Session"))
	}
	if err := services.ValidateWaiterSession(sessionTok, table.ID, *table.CurrentOrderID); err != nil {
		status, _ := services.BuildAssistanceStatusForTable(h.db, h.orderService, table)
		c.JSON(http.StatusForbidden, gin.H{
			"error":  "Ask staff for this seating's Call waiter code, then unlock on this page.",
			"status": status,
		})
		return
	}

	if err := services.EnsureTableAssistanceToken(h.db, table); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not open table session."})
		return
	}

	newly, err := services.RequestTableAssistance(h.db, table)
	if err != nil {
		if errors.Is(err, services.ErrTableVacant) || errors.Is(err, services.ErrNoActiveOrder) {
			status, _ := services.BuildAssistanceStatusForTable(h.db, h.orderService, table)
			c.JSON(http.StatusConflict, gin.H{
				"error":  "Call waiter is only available while you are seated with an active order.",
				"status": status,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not notify staff"})
		return
	}

	if newly {
		if globalHub != nil {
			BroadcastTableUpdate(globalHub, table.RestaurantID, table)
		}
		notifyStaffPush(h.db, table.RestaurantID, services.PushAlertAssistance,
			"Customer needs assistance",
			"Table "+table.Name+" requested a waiter",
			map[string]string{
				"table_id":   table.ID,
				"table_name": table.Name,
			},
		)
	}

	status, err := services.BuildAssistanceStatusForTable(h.db, h.orderService, table)
	if err == nil && status != nil {
		publishAssistanceStatusForTable(table, *status)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Staff notified",
		"status":  status,
	})
}

func publishAssistanceStatus(token string, status services.AssistanceStatus) {
	if globalAssistanceHub == nil || strings.TrimSpace(token) == "" {
		return
	}
	globalAssistanceHub.Publish(token, status)
}

func publishAssistanceStatusForTable(table *models.RestaurantTable, status services.AssistanceStatus) {
	if table == nil || table.AssistanceToken == nil {
		return
	}
	publishAssistanceStatus(strings.TrimSpace(*table.AssistanceToken), status)
}

// NotifyAssistanceUpdateByTableID rebuilds and publishes live table-session SSE status.
func NotifyAssistanceUpdateByTableID(db *gorm.DB, orderService *services.OrderService, tableID string) {
	if globalAssistanceHub == nil || db == nil || strings.TrimSpace(tableID) == "" {
		return
	}

	var table models.RestaurantTable
	if err := db.Where("id = ?", tableID).First(&table).Error; err != nil {
		return
	}
	if err := services.EnsureTableAssistanceToken(db, &table); err != nil {
		return
	}
	status, err := services.BuildAssistanceStatusForTable(db, orderService, &table)
	if err != nil || status == nil {
		return
	}
	publishAssistanceStatusForTable(&table, *status)
}

// NotifyAssistanceUpdateByOrder publishes SSE updates for the order's dine-in table.
func NotifyAssistanceUpdateByOrder(db *gorm.DB, orderService *services.OrderService, order *models.Order) {
	if order == nil || order.TableID == nil || strings.TrimSpace(*order.TableID) == "" {
		return
	}
	NotifyAssistanceUpdateByTableID(db, orderService, *order.TableID)
}

func assistanceErrorHTML(message string) string {
	return `<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"/><meta name="viewport" content="width=device-width,initial-scale=1"/>
<title>Table</title>
<style>body{font-family:system-ui,sans-serif;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0;background:#f5f5f5;color:#333}
.card{background:#fff;padding:32px;border-radius:16px;max-width:360px;text-align:center;box-shadow:0 4px 20px rgba(0,0,0,.08)}</style></head>
<body><div class="card"><h1>Table</h1><p>` + html.EscapeString(message) + `</p></div></body></html>`
}
