package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"restaurant-api/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var globalTrackHub *services.OrderTrackingHub

// SetOrderTrackingHub configures the in-memory SSE hub for customer order tracking.
func SetOrderTrackingHub(hub *services.OrderTrackingHub) {
	globalTrackHub = hub
}

// SetupTrackRoutes registers public customer tracking pages (no auth).
func SetupTrackRoutes(router *gin.Engine, db *gorm.DB) {
	orderService := services.NewOrderService(db)
	handler := &TrackHandler{
		orderService: orderService,
		hub:          globalTrackHub,
	}

	router.GET("/t/:token", handler.TrackPage)
	router.GET("/t/:token/events", handler.TrackEvents)
	router.GET("/t/:token/status", handler.TrackStatusJSON)

	log.Println("✅ Order tracking routes registered at /t/:token")
}

type TrackHandler struct {
	orderService *services.OrderService
	hub          *services.OrderTrackingHub
}

func (h *TrackHandler) loadTracking(token string) (*services.TrackingStatus, int, string) {
	order, restaurant, err := h.orderService.GetOrderByTrackingToken(token)
	if err != nil {
		if strings.Contains(err.Error(), "expired") {
			return nil, http.StatusGone, "This tracking link has expired."
		}
		return nil, http.StatusNotFound, "Order not found."
	}

	restaurantName := ""
	if restaurant != nil {
		restaurantName = restaurant.Name
	}
	status := services.BuildTrackingStatus(order, restaurantName)
	return &status, http.StatusOK, ""
}

func (h *TrackHandler) TrackPage(c *gin.Context) {
	token := c.Param("token")
	status, code, message := h.loadTracking(token)
	if status == nil {
		c.Data(code, "text/html; charset=utf-8", []byte(trackErrorHTML(message)))
		return
	}

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(renderTrackPageHTML(token, *status)))
}

func (h *TrackHandler) TrackStatusJSON(c *gin.Context) {
	token := c.Param("token")
	status, code, message := h.loadTracking(token)
	if status == nil {
		c.JSON(code, gin.H{"error": message})
		return
	}
	c.JSON(http.StatusOK, status)
}

func (h *TrackHandler) TrackEvents(c *gin.Context) {
	token := c.Param("token")
	status, code, message := h.loadTracking(token)
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

	writeSSE := func(payload services.TrackingStatus) {
		data, _ := json.Marshal(payload)
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		flusher.Flush()
	}

	writeSSE(*status)

	if h.hub == nil {
		// Fallback: keep connection open with periodic noop until client disconnects
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

func NotifyOrderTrackingUpdate(orderService *services.OrderService, orderID, restaurantID string) {
	if globalTrackHub == nil || orderService == nil {
		return
	}
	order, err := orderService.GetOrderByID(restaurantID, orderID)
	if err != nil || order.TrackingToken == "" || !services.IsCounterOrderForTracking(order) {
		return
	}
	restaurantName := orderService.GetRestaurantName(order.RestaurantID)
	status := services.BuildTrackingStatus(order, restaurantName)
	globalTrackHub.Publish(order.TrackingToken, status)
}

func trackErrorHTML(message string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"/><meta name="viewport" content="width=device-width,initial-scale=1"/>
<title>Order tracking</title>
<style>body{font-family:system-ui,sans-serif;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0;background:#f5f5f5;color:#333}
.card{background:#fff;padding:32px;border-radius:16px;max-width:360px;text-align:center;box-shadow:0 4px 20px rgba(0,0,0,.08)}</style></head>
<body><div class="card"><h1>Order tracking</h1><p>%s</p></div></body></html>`, message)
}

func renderTrackPageHTML(token string, status services.TrackingStatus) string {
	color := status.Color
	if color != "orange" && color != "green" {
		color = "red"
	}
	bg := map[string]string{
		"red":    "#fee2e2",
		"orange": "#ffedd5",
		"green":  "#dcfce7",
	}[color]
	border := map[string]string{
		"red":    "#ef4444",
		"orange": "#f97316",
		"green":  "#22c55e",
	}[color]
	mode := "Counter"
	if status.ServiceMode == "takeaway" {
		mode = "Takeaway"
	}
	title := status.RestaurantName
	if title == "" {
		title = "BillGenie"
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>Order #%d</title>
  <style>
    * { box-sizing: border-box; }
    body { font-family: system-ui, -apple-system, sans-serif; margin: 0; min-height: 100vh; background: #f8fafc; color: #0f172a; }
    .wrap { max-width: 420px; margin: 0 auto; padding: 24px 16px 40px; }
    h1 { font-size: 1.25rem; margin: 0 0 4px; text-align: center; color: #475569; font-weight: 600; }
    .ticket { text-align: center; font-size: 2.5rem; font-weight: 800; margin: 8px 0 20px; }
    .box { border: 4px solid %s; background: %s; border-radius: 20px; padding: 36px 20px; text-align: center; min-height: 180px; display: flex; flex-direction: column; justify-content: center; transition: background .4s, border-color .4s; }
    .message { font-size: 1.5rem; font-weight: 700; margin: 0; }
    .sub { margin-top: 10px; color: #475569; font-size: 1rem; }
    .mode { text-align: center; margin-top: 16px; color: #64748b; font-size: .95rem; }
    .footer { margin-top: 28px; text-align: center; color: #94a3b8; font-size: .85rem; }
  </style>
</head>
<body>
  <div class="wrap">
    <h1>%s</h1>
    <div class="ticket">#%d</div>
    <div class="box" id="status-box">
      <p class="message" id="status-message">%s</p>
      <p class="sub" id="status-sub">%s</p>
    </div>
    <p class="mode">%s order</p>
    <p class="footer">This page updates automatically. Show your ticket number at the counter.</p>
  </div>
  <script>
    const token = %q;
    const colors = { red: { bg: '#fee2e2', border: '#ef4444' }, orange: { bg: '#ffedd5', border: '#f97316' }, green: { bg: '#dcfce7', border: '#22c55e' } };
    function applyStatus(data) {
      const c = colors[data.color] || colors.red;
      const box = document.getElementById('status-box');
      box.style.background = c.bg;
      box.style.borderColor = c.border;
      document.getElementById('status-message').textContent = data.message;
      const sub = data.total_count > 0 && data.color === 'orange'
        ? data.ready_count + ' of ' + data.total_count + ' items ready'
        : (data.color === 'green' ? 'Please collect your order' : 'Kitchen is working on your order');
      document.getElementById('status-sub').textContent = sub;
    }
    function connect() {
      const es = new EventSource('/t/' + token + '/events');
      es.onmessage = (e) => { try { applyStatus(JSON.parse(e.data)); } catch (_) {} };
      es.onerror = () => { es.close(); setTimeout(connect, 3000); };
    }
    connect();
  </script>
</body>
</html>`, status.TicketNumber, border, bg, title, status.TicketNumber, status.Message,
		subTextForStatus(status), mode, token)
}

func subTextForStatus(status services.TrackingStatus) string {
	switch status.Color {
	case "green":
		return "Please collect your order"
	case "orange":
		if status.TotalCount > 0 {
			return fmt.Sprintf("%d of %d items ready", status.ReadyCount, status.TotalCount)
		}
		return "Some items are ready"
	default:
		return "Kitchen is working on your order"
	}
}
