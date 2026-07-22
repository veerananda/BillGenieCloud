package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"restaurant-api/internal/models"
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
	router.GET("/t/:token/download", handler.TrackDownload)
	router.GET("/t/:token/events", handler.TrackEvents)
	router.GET("/t/:token/status", handler.TrackStatusJSON)

	log.Println("✅ Order tracking routes registered at /t/:token")
}

type TrackHandler struct {
	orderService *services.OrderService
	hub          *services.OrderTrackingHub
}

type trackPageContext struct {
	order          *models.Order
	restaurant     *models.Restaurant
	summary        services.BillSummaryView
	status         services.TrackingStatus
	kitchenEnabled bool
}

func (h *TrackHandler) loadTrackPage(token string) (*trackPageContext, int, string) {
	order, restaurant, err := h.orderService.GetOrderByTrackingToken(token)
	if err != nil {
		if strings.Contains(err.Error(), "expired") {
			return nil, http.StatusGone, "This tracking link has expired."
		}
		return nil, http.StatusNotFound, "Order not found."
	}

	if !services.IsCounterOrderForTracking(order) {
		return nil, http.StatusNotFound, "Order not found."
	}

	restaurantName := ""
	var limits services.SubscriptionLimits
	if restaurant != nil {
		restaurantName = restaurant.Name
		limits, _ = services.LoadSubscriptionLimits(h.orderService.GetDB(), restaurant)
	}

	kitchenEnabled := services.OrderUsesKitchen(limits, order)
	summary := services.BuildBillSummary(order, restaurant)
	status := services.BuildTrackingStatus(order, restaurantName)

	return &trackPageContext{
		order:          order,
		restaurant:     restaurant,
		summary:        summary,
		status:         status,
		kitchenEnabled: kitchenEnabled,
	}, http.StatusOK, ""
}

func (h *TrackHandler) loadTracking(token string) (*services.TrackingStatus, int, string) {
	ctx, code, message := h.loadTrackPage(token)
	if ctx == nil {
		return nil, code, message
	}
	return &ctx.status, http.StatusOK, ""
}

func (h *TrackHandler) TrackPage(c *gin.Context) {
	token := c.Param("token")
	ctx, code, message := h.loadTrackPage(token)
	if ctx == nil {
		c.Data(code, "text/html; charset=utf-8", []byte(trackErrorHTML(message)))
		return
	}

	var html string
	if ctx.kitchenEnabled {
		html = renderTrackPageWithKitchenHTML(token, ctx.status, ctx.summary)
	} else {
		html = renderTrackBillOnlyPageHTML(token, ctx.summary)
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

func (h *TrackHandler) TrackDownload(c *gin.Context) {
	token := c.Param("token")
	ctx, code, message := h.loadTrackPage(token)
	if ctx == nil {
		c.Data(code, "text/html; charset=utf-8", []byte(trackErrorHTML(message)))
		return
	}

	filename := fmt.Sprintf("bill-%d.html", ctx.summary.TicketNumber)
	if ctx.summary.TicketNumber <= 0 {
		filename = fmt.Sprintf("bill-%d.html", ctx.summary.OrderNumber)
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(buildBillDownloadHTML(ctx.summary)))
}

func (h *TrackHandler) TrackStatusJSON(c *gin.Context) {
	token := c.Param("token")
	ctx, code, message := h.loadTrackPage(token)
	if ctx == nil {
		c.JSON(code, gin.H{"error": message})
		return
	}
	if !ctx.kitchenEnabled {
		c.JSON(http.StatusOK, gin.H{
			"kitchen_enabled": false,
			"ticket_number":   ctx.status.TicketNumber,
			"message":         "Kitchen updates are not enabled for this order.",
		})
		return
	}
	c.JSON(http.StatusOK, ctx.status)
}

func (h *TrackHandler) TrackEvents(c *gin.Context) {
	token := c.Param("token")
	ctx, code, message := h.loadTrackPage(token)
	if ctx == nil {
		c.JSON(code, gin.H{"error": message})
		return
	}
	if !ctx.kitchenEnabled {
		c.JSON(http.StatusOK, gin.H{"kitchen_enabled": false})
		return
	}

	status := ctx.status
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

	writeSSE(status)

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

func NotifyOrderTrackingUpdate(orderService *services.OrderService, orderID, restaurantID string) {
	if globalTrackHub == nil || orderService == nil {
		return
	}
	order, err := orderService.GetOrderByID(restaurantID, orderID)
	if err != nil || order.TrackingToken == "" || !services.IsCounterOrderForTracking(order) {
		return
	}
	var restaurant models.Restaurant
	if err := orderService.GetDB().Where("id = ?", order.RestaurantID).First(&restaurant).Error; err != nil {
		return
	}
	limits, _ := services.LoadSubscriptionLimits(orderService.GetDB(), &restaurant)
	if !services.OrderUsesKitchen(limits, order) {
		return
	}
	restaurantName := orderService.GetRestaurantName(order.RestaurantID)
	status := services.BuildTrackingStatus(order, restaurantName)
	globalTrackHub.Publish(order.TrackingToken, status)
}

func trackErrorHTML(message string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"/><meta name="viewport" content="width=device-width,initial-scale=1"/>
<title>Order</title>
<style>body{font-family:system-ui,sans-serif;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0;background:#f5f5f5;color:#333}
.card{background:#fff;padding:32px;border-radius:16px;max-width:360px;text-align:center;box-shadow:0 4px 20px rgba(0,0,0,.08)}</style></head>
<body><div class="card"><h1>Order</h1><p>%s</p></div></body></html>`, message)
}

func trackStatusStylesBlock() string {
	return `<style>
    .track-wrap { max-width: 420px; margin: 0 auto; }
    .track-status { margin-bottom: 20px; }
    .track-status h1 { font-size: 1.25rem; margin: 0 0 4px; text-align: center; color: #475569; font-weight: 600; }
    .track-ticket { text-align: center; font-size: 2.5rem; font-weight: 800; margin: 8px 0 20px; color: #0f172a; }
    .track-box { border: 4px solid #ef4444; background: #fee2e2; border-radius: 20px; padding: 36px 20px; text-align: center; min-height: 160px; display: flex; flex-direction: column; justify-content: center; transition: background .4s, border-color .4s; }
    .track-message { font-size: 1.5rem; font-weight: 700; margin: 0; }
    .track-sub { margin-top: 10px; color: #475569; font-size: 1rem; }
    .track-mode { text-align: center; margin-top: 16px; color: #64748b; font-size: .95rem; }
    .track-footer { margin-top: 16px; text-align: center; color: #94a3b8; font-size: .85rem; }
  </style>`
}

func renderTrackBillSection(token string, summary services.BillSummaryView) string {
	fragment := renderCustomerBillPageFragment(summary)
	fragment = strings.Replace(fragment, "<!--BILL_BADGE-->", `<span class="badge paid">Paid</span>`, 1)
	actions := renderBillDownloadActions(
		fmt.Sprintf("/t/%s/download", token),
		"Save or print this bill from your browser. This link expires in 4 hours.",
	)
	return strings.Replace(fragment, "<!--BILL_ACTIONS-->", actions, 1)
}

func renderTrackBillOnlyPageHTML(token string, summary services.BillSummaryView) string {
	billSection := renderTrackBillSection(token, summary)
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>Bill #%d</title>
  %s
</head>
<body>
  <div class="track-wrap">
    %s
  </div>
</body>
</html>`, summary.OrderNumber, customerBillStylesBlock(), billSection)
}

func renderTrackPageWithKitchenHTML(token string, status services.TrackingStatus, summary services.BillSummaryView) string {
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

	statusSection := fmt.Sprintf(`<div class="track-status">
    <h1>%s</h1>
    <div class="track-ticket">#%d</div>
    <div class="track-box" id="status-box" style="border-color:%s;background:%s">
      <p class="track-message" id="status-message">%s</p>
      <p class="track-sub" id="status-sub">%s</p>
    </div>
    <p class="track-mode">%s order</p>
    <p class="track-footer">Order status updates automatically below.</p>
  </div>`,
		escapeBillHTML(title),
		status.TicketNumber,
		border, bg,
		escapeBillHTML(status.Message),
		escapeBillHTML(subTextForStatus(status)),
		mode,
	)

	billSection := renderTrackBillSection(token, summary)

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>Order #%d</title>
  %s
  %s
</head>
<body>
  <div class="track-wrap">
    %s
    %s
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
      const sub = data.color === 'orange'
        ? "We're preparing the rest of your order"
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
</html>`, status.TicketNumber, customerBillStylesBlock(), trackStatusStylesBlock(), statusSection, billSection, token)
}

func subTextForStatus(status services.TrackingStatus) string {
	switch status.Color {
	case "green":
		return "Please collect your order"
	case "orange":
		return "We're preparing the rest of your order"
	default:
		return "Kitchen is working on your order"
	}
}
