package handlers

import (
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"strings"
	"time"

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

// SetupAssistanceRoutes registers public customer assistance pages (no auth).
func SetupAssistanceRoutes(router *gin.Engine, db *gorm.DB) {
	handler := &AssistanceHandler{
		db:           db,
		orderService: services.NewOrderService(db),
		hub:          globalAssistanceHub,
	}
	router.GET("/a/:token", handler.AssistancePage)
	router.GET("/a/:token/status", handler.AssistanceStatus)
	router.GET("/a/:token/events", handler.AssistanceEvents)
	router.POST("/a/:token/call-waiter", handler.CallWaiter)
	log.Println("✅ Customer assistance routes registered at /a/:token")
}

type AssistanceHandler struct {
	db           *gorm.DB
	orderService *services.OrderService
	hub          *services.AssistanceHub
}

func (h *AssistanceHandler) loadStatus(token string) (*services.AssistanceStatus, *models.RestaurantTable, int, string) {
	table, err := services.GetTableByAssistanceToken(h.db, token)
	if err != nil {
		return nil, nil, http.StatusNotFound, "Table link not found."
	}

	var restaurant models.Restaurant
	_ = h.db.Select("id", "name").Where("id = ?", table.RestaurantID).First(&restaurant).Error

	status := &services.AssistanceStatus{
		RestaurantName:      restaurant.Name,
		TableName:           table.Name,
		IsOccupied:          table.IsOccupied,
		AssistanceRequested: services.TableNeedsAssistance(table),
	}

	if table.CurrentOrderID != nil && *table.CurrentOrderID != "" {
		order, err := h.orderService.GetOrderByID(table.RestaurantID, *table.CurrentOrderID)
		if err == nil && order != nil && order.Status != "completed" && order.Status != "cancelled" {
			status.HasActiveOrder = true
			qty := 0
			for _, item := range order.Items {
				qty += item.Quantity
			}
			status.ItemCount = qty
			status.OrderTotal = order.Total
			if order.Total <= 0 {
				status.OrderTotal = order.SubTotal
			}

			if strings.TrimSpace(order.BillToken) != "" {
				if order.BillExpiresAt == nil || order.BillExpiresAt.After(time.Now()) {
					status.BillAvailable = true
					status.BillURL = services.BuildBillURL(order.BillToken)
					status.BillDownloadURL = status.BillURL + "/download"
				}
			}
		}
	}

	return status, table, http.StatusOK, ""
}

func (h *AssistanceHandler) AssistancePage(c *gin.Context) {
	token := c.Param("token")
	status, _, code, message := h.loadStatus(token)
	if status == nil {
		c.Data(code, "text/html; charset=utf-8", []byte(assistanceErrorHTML(message)))
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(renderAssistancePageHTML(token, *status)))
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

func (h *AssistanceHandler) AssistanceEvents(c *gin.Context) {
	token := c.Param("token")
	status, _, code, message := h.loadStatus(token)
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

	ch := h.hub.Subscribe(token)
	defer h.hub.Unsubscribe(token, ch)

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

func (h *AssistanceHandler) CallWaiter(c *gin.Context) {
	token := c.Param("token")
	table, err := services.GetTableByAssistanceToken(h.db, token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Table link not found."})
		return
	}

	if err := services.RequestTableAssistance(h.db, table); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not notify staff"})
		return
	}

	if globalHub != nil {
		BroadcastTableUpdate(globalHub, table.RestaurantID, table)
	}

	status, _, _, _ := h.loadStatus(token)
	if status != nil {
		publishAssistanceStatus(token, *status)
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

// NotifyAssistanceUpdateByTableID rebuilds and publishes assistance SSE status for a table.
func NotifyAssistanceUpdateByTableID(db *gorm.DB, orderService *services.OrderService, tableID string) {
	if globalAssistanceHub == nil || db == nil || orderService == nil || strings.TrimSpace(tableID) == "" {
		return
	}

	var table models.RestaurantTable
	if err := db.Where("id = ?", tableID).First(&table).Error; err != nil {
		return
	}
	token := ""
	if table.AssistanceToken != nil {
		token = strings.TrimSpace(*table.AssistanceToken)
	}
	if token == "" {
		return
	}

	handler := &AssistanceHandler{db: db, orderService: orderService, hub: globalAssistanceHub}
	status, _, _, _ := handler.loadStatus(token)
	if status == nil {
		return
	}
	publishAssistanceStatus(token, *status)
}

// NotifyAssistanceUpdateByOrder publishes assistance SSE updates for the order's dine-in table.
func NotifyAssistanceUpdateByOrder(db *gorm.DB, orderService *services.OrderService, order *models.Order) {
	if order == nil || order.TableID == nil || strings.TrimSpace(*order.TableID) == "" {
		return
	}
	NotifyAssistanceUpdateByTableID(db, orderService, *order.TableID)
}

func assistanceErrorHTML(message string) string {
	return `<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"/><meta name="viewport" content="width=device-width,initial-scale=1"/>
<title>Table assistance</title>
<style>body{font-family:system-ui,sans-serif;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0;background:#f5f5f5;color:#333}
.card{background:#fff;padding:32px;border-radius:16px;max-width:360px;text-align:center;box-shadow:0 4px 20px rgba(0,0,0,.08)}</style></head>
<body><div class="card"><h1>Table assistance</h1><p>` + html.EscapeString(message) + `</p></div></body></html>`
}
