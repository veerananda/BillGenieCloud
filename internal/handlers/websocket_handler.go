package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"restaurant-api/internal/models"
	"restaurant-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// WebSocketHub manages WebSocket connections and broadcasts
type WebSocketHub struct {
	clients    map[*WebSocketClient]bool
	broadcast  chan interface{}
	register   chan *WebSocketClient
	unregister chan *WebSocketClient
	mu         sync.RWMutex
	roomMap    map[string][]*WebSocketClient // room_id -> clients
}

// WebSocketClient represents a connected client
type WebSocketClient struct {
	hub          *WebSocketHub
	conn         *websocket.Conn
	send         chan interface{}
	userID       string
	restaurantID string
	roomID       string
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	Subprotocols:    []string{"billgenie"},
	CheckOrigin: func(r *http.Request) bool {
		return true // replaced by ConfigureWebSocketOrigins at startup
	},
}

var wsAllowedOrigins []string

// ConfigureWebSocketOrigins restricts browser WS Origin to the CORS allowlist.
// React Native / Expo often set Origin to the API host itself (not a web app origin);
// those are allowed when the Origin host matches the request Host. JWT auth still applies.
func ConfigureWebSocketOrigins(allowedOrigins []string) {
	wsAllowedOrigins = append([]string(nil), allowedOrigins...)
	upgrader.CheckOrigin = func(r *http.Request) bool {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" {
			return true // native / non-browser clients
		}
		for _, o := range wsAllowedOrigins {
			if o == "*" || o == origin {
				return true
			}
		}
		if originMatchesRequestHost(r, origin) {
			return true
		}
		log.Printf("❌ WebSocket origin rejected: %s", origin)
		return false
	}
}

func originMatchesRequestHost(r *http.Request, origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	reqHost := r.Host
	if fwd := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); fwd != "" {
		// Fly / proxies may forward the public host separately.
		reqHost = strings.Split(fwd, ",")[0]
		reqHost = strings.TrimSpace(reqHost)
	}
	return strings.EqualFold(u.Host, reqHost)
}

// ExtractWebSocketToken prefers Sec-WebSocket-Protocol (billgenie,<jwt>); falls back to ?token=.
func ExtractWebSocketToken(r *http.Request) (token string, viaQuery bool) {
	header := r.Header.Get("Sec-WebSocket-Protocol")
	if header != "" {
		parts := strings.Split(header, ",")
		cleaned := make([]string, 0, len(parts))
		for _, p := range parts {
			if t := strings.TrimSpace(p); t != "" {
				cleaned = append(cleaned, t)
			}
		}
		for i, p := range cleaned {
			if p == "billgenie" && i+1 < len(cleaned) {
				return cleaned[i+1], false
			}
		}
	}
	if q := strings.TrimSpace(r.URL.Query().Get("token")); q != "" {
		return q, true
	}
	return "", false
}

// Global hub instance for broadcasting
var globalHub *WebSocketHub

// EventPublisher publishes events to all connected clients (local hub + optional Redis).
type EventPublisher interface {
	Publish(roomID string, event models.NotificationEvent)
}

var globalPublisher EventPublisher

// SetGlobalHub sets the global WebSocket hub
func SetGlobalHub(hub *WebSocketHub) {
	globalHub = hub
}

// SetEventPublisher sets the global event publisher (hub-only or Redis-backed).
func SetEventPublisher(publisher EventPublisher) {
	globalPublisher = publisher
}

// NewWebSocketHub creates a new WebSocket hub
func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		broadcast:  make(chan interface{}, 256),
		register:   make(chan *WebSocketClient, 256),
		unregister: make(chan *WebSocketClient, 256),
		clients:    make(map[*WebSocketClient]bool),
		roomMap:    make(map[string][]*WebSocketClient),
	}
}

// Run starts the hub event loop
func (h *WebSocketHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			if room, exists := h.roomMap[client.roomID]; exists {
				h.roomMap[client.roomID] = append(room, client)
			} else {
				h.roomMap[client.roomID] = []*WebSocketClient{client}
			}
			h.mu.Unlock()
			log.Printf("✅ Client connected to room %s: %s", client.roomID, client.userID)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				if room, exists := h.roomMap[client.roomID]; exists {
					// Remove client from room
					newRoom := make([]*WebSocketClient, 0)
					for _, c := range room {
						if c != client {
							newRoom = append(newRoom, c)
						}
					}
					if len(newRoom) == 0 {
						delete(h.roomMap, client.roomID)
					} else {
						h.roomMap[client.roomID] = newRoom
					}
				}
			}
			h.mu.Unlock()
			log.Printf("✅ Client disconnected from room %s: %s", client.roomID, client.userID)

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// Channel full, skip this client
					log.Printf("⚠️  Client send buffer full, dropping message for %s", client.userID)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// BroadcastToRoom sends a message to all clients in a specific room
func (h *WebSocketHub) BroadcastToRoom(roomID string, message interface{}) {
	h.mu.RLock()
	room, exists := h.roomMap[roomID]
	clientCount := len(room)
	h.mu.RUnlock()

	if !exists {
		log.Printf("⚠️  [BROADCAST FAILED] Room %s does not exist! Connected rooms: %v", roomID, h.getRoomList())
		return
	}

	if clientCount == 0 {
		log.Printf("⚠️  [BROADCAST FAILED] Room %s exists but has 0 clients!", roomID)
		return
	}

	log.Printf("📤 [BROADCAST] Sending to %d clients in room %s", clientCount, roomID)

	for _, client := range room {
		select {
		case client.send <- message:
			log.Printf("   ✓ Message sent to client %s", client.userID)
		default:
			log.Printf("⚠️  Client send buffer full for %s in room %s", client.userID, roomID)
		}
	}
}

// getRoomList returns list of all connected rooms (for debugging)
func (h *WebSocketHub) getRoomList() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	rooms := make([]string, 0, len(h.roomMap))
	for roomID := range h.roomMap {
		rooms = append(rooms, roomID)
	}
	return rooms
}

// HandleWebSocket handles WebSocket connections
func HandleWebSocket(c *gin.Context, hub *WebSocketHub) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("❌ WebSocket upgrade error: %v", err)
		return
	}

	roomID := restaurantID.(string) // Use restaurant ID as room

	client := &WebSocketClient{
		hub:          hub,
		conn:         conn,
		send:         make(chan interface{}, 256),
		userID:       userID.(string),
		restaurantID: restaurantID.(string),
		roomID:       roomID,
	}

	hub.register <- client

	// Send welcome message
	welcomeMsg := models.NotificationEvent{
		Type:      "connected",
		RoomID:    roomID,
		Timestamp: time.Now(),
		Version:   1,
		Seq:       time.Now().UnixNano(),
		Data:      json.RawMessage(`{"message":"Connected to server"}`),
	}
	client.send <- welcomeMsg

	// Handle client messages
	go client.readPump()
	go client.writePump()
}

// readPump reads from WebSocket connection
func (c *WebSocketClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var event models.NotificationEvent
		err := c.conn.ReadJSON(&event)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("❌ WebSocket error: %v", err)
			}
			break
		}

		// Log received event
		log.Printf("📩 Message from %s: %s", c.userID, event.Type)

		// Broadcast to room (e.g., order updates)
		if event.Type == "order_update" || event.Type == "inventory_update" {
			event.RoomID = c.roomID
			c.hub.BroadcastToRoom(c.roomID, event)
		}
	}
}

// writePump writes to WebSocket connection
func (c *WebSocketClient) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteJSON(message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// HubPublisher publishes events via the in-memory WebSocket hub only.
type HubPublisher struct {
	Hub *WebSocketHub
}

func (p *HubPublisher) Publish(roomID string, event models.NotificationEvent) {
	if p != nil && p.Hub != nil {
		p.Hub.BroadcastToRoom(roomID, event)
	}
}

func buildWSOrderItems(items []models.OrderItem) []models.WSOrderItem {
	out := make([]models.WSOrderItem, 0, len(items))
	for _, item := range items {
		ws := models.WSOrderItem{
			ID:           item.ID,
			MenuID:       item.MenuID,
			Quantity:     item.Quantity,
			UnitRate:     item.UnitRate,
			Total:        item.Total,
			Status:       item.Status,
			SubId:        item.SubId,
			Notes:        item.Notes,
			VariantID:    item.VariantID,
			VariantLabel: item.VariantLabel,
			CreatedAt:    item.CreatedAt,
		}
		if item.MenuItem != nil {
			ws.Name = services.FormatOrderItemDisplayName(item.MenuItem.Name, item.VariantLabel)
			ws.IsVegetarian = item.MenuItem.IsVeg
		} else if strings.TrimSpace(item.VariantLabel) != "" {
			ws.Name = services.FormatOrderItemDisplayName("Item", item.VariantLabel)
		}
		out = append(out, ws)
	}
	return out
}

func buildOrderEventData(order *models.Order) models.OrderEventData {
	tableOccupied := order.Status != "cancelled" && order.Status != "completed"
	isSelfService := order.OrderType == "counter" || order.CustomerName == "Self Service" ||
		order.CustomerName == "Takeaway" || order.CustomerName == "Counter"

	return models.OrderEventData{
		OrderID:       order.ID,
		OrderNumber:   order.OrderNumber,
		OrderType:     order.OrderType,
		TicketNumber:  order.TicketNumber,
		ServiceMode:   order.ServiceMode,
		TableID:       order.TableID,
		TableNo:       order.TableNumber,
		TableOccupied: tableOccupied,
		CustomerName:  order.CustomerName,
		Status:        order.Status,
		SubTotal:      order.SubTotal,
		TaxAmount:     order.TaxAmount,
		TotalAmount:   order.Total,
		ItemCount:     len(order.Items),
		Items:         buildWSOrderItems(order.Items),
		PaymentMethod: order.PaymentMethod,
		IsSelfService: isSelfService,
		CreatedAt:     order.CreatedAt,
		UpdatedAt:     order.UpdatedAt,
	}
}

func publishEvent(restaurantID, eventType string, data interface{}) {
	if globalPublisher == nil && globalHub == nil {
		return
	}

	event := models.NotificationEvent{
		Type:      eventType,
		RoomID:    restaurantID,
		Timestamp: time.Now(),
		Version:   1,
		Seq:       time.Now().UnixNano(),
		Data:      json.RawMessage(toJSON(data)),
	}

	if globalPublisher != nil {
		globalPublisher.Publish(restaurantID, event)
	} else if globalHub != nil {
		globalHub.BroadcastToRoom(restaurantID, event)
	}
}

// BroadcastSessionRevoked notifies connected clients that a user signed in elsewhere.
func BroadcastSessionRevoked(restaurantID, userID string) {
	if restaurantID == "" || userID == "" {
		return
	}
	data := map[string]string{
		"user_id": userID,
		"reason":  "logged_in_elsewhere",
	}
	publishEvent(restaurantID, "session_revoked", data)
	log.Printf("📤 Broadcast session_revoked for user %s in room %s", userID, restaurantID)
}

// BroadcastOrderEvent broadcasts an order event with the correct type.
func BroadcastOrderEvent(hub *WebSocketHub, restaurantID, eventType string, order *models.Order) {
	if hub == nil && globalPublisher == nil {
		return
	}
	_ = hub // kept for call-site compatibility
	data := buildOrderEventData(order)
	publishEvent(restaurantID, eventType, data)
	log.Printf("📤 Broadcast %s: Order #%d (Table: %s) to room %s", eventType, order.OrderNumber, order.TableNumber, restaurantID)
}

// BroadcastOrderUpdate is deprecated — use BroadcastOrderEvent with an explicit type.
func BroadcastOrderUpdate(hub *WebSocketHub, restaurantID string, order *models.Order) {
	BroadcastOrderEvent(hub, restaurantID, "order_updated", order)
}

// BroadcastOrderItemStatusEvent broadcasts a kitchen item status change with full order context.
func BroadcastOrderItemStatusEvent(hub *WebSocketHub, restaurantID string, order *models.Order, itemID, menuID string, bulk bool) {
	if order == nil {
		return
	}
	data := buildOrderEventData(order)
	data.ItemID = itemID
	data.MenuID = menuID
	data.BulkUpdate = bulk
	publishEvent(restaurantID, "order_item_status_changed", data)
}

func BroadcastTableUpdate(hub *WebSocketHub, restaurantID string, table *models.RestaurantTable) {
	_ = hub
	data := models.TableEventData{
		TableID:             table.ID,
		TableNumber:         table.Name,
		IsOccupied:          table.IsOccupied,
		CurrentOrderID:      table.CurrentOrderID,
		AssistanceRequested: services.TableNeedsAssistance(table),
	}
	publishEvent(restaurantID, "table_status_changed", data)
	log.Printf("📤 Broadcast table update: Table %s (Occupied: %v, Assistance: %v) to room %s", table.Name, table.IsOccupied, data.AssistanceRequested, restaurantID)
}

// BroadcastCheckoutEvent broadcasts checkout lock start/cancel events.
func BroadcastCheckoutEvent(hub *WebSocketHub, restaurantID, eventType string, data models.CheckoutEventData) {
	_ = hub
	publishEvent(restaurantID, eventType, data)
	log.Printf("📤 Broadcast %s: order %s by %s to room %s", eventType, data.OrderID, data.LockedByName, restaurantID)
}

// BroadcastMenuUpdate notifies clients that the menu changed.
// Cost price is cleared so non-admin WS clients never receive margin data.
func BroadcastMenuUpdate(hub *WebSocketHub, restaurantID, action string, item *models.MenuItem, menuItemID string) {
	if action == "" {
		return
	}
	_ = hub
	var safeItem *models.MenuItem
	if item != nil {
		copy := *item
		copy.CostPrice = 0
		safeItem = &copy
	}
	data := models.MenuEventData{
		Action:     action,
		MenuItemID: menuItemID,
		MenuItem:   safeItem,
	}
	if action == "deleted" && menuItemID == "" && item != nil {
		data.MenuItemID = item.ID
	}
	publishEvent(restaurantID, "menu_updated", data)
	if action == "deleted" {
		log.Printf("📤 Broadcast menu_updated (deleted): %s to room %s", data.MenuItemID, restaurantID)
		return
	}
	if item != nil {
		log.Printf("📤 Broadcast menu_updated (%s): %s to room %s", action, item.Name, restaurantID)
	}
}

// BroadcastInventoryUpdate broadcasts menu-item inventory changes
func BroadcastInventoryUpdate(hub *WebSocketHub, restaurantID string, itemName string, quantity float64, isLow bool) {
	_ = hub
	data := models.InventoryEventData{
		Kind:     "menu_item",
		ItemName: itemName,
		Quantity: quantity,
		IsLow:    isLow,
	}
	publishEvent(restaurantID, "inventory_updated", data)
	log.Printf("📤 Broadcast inventory update: %s (Qty: %.2f) to room %s", itemName, quantity, restaurantID)
}

func ingredientIsLowStock(currentStock, alertQuantity float64) bool {
	if alertQuantity <= 0 {
		return false
	}
	return currentStock <= alertQuantity
}

// BroadcastIngredientInventoryUpdate broadcasts raw ingredient stock changes.
func BroadcastIngredientInventoryUpdate(hub *WebSocketHub, restaurantID string, ingredient models.Ingredient) {
	_ = hub
	data := models.InventoryEventData{
		Kind:          "ingredient",
		IngredientID:  ingredient.ID,
		ItemName:      ingredient.Name,
		Unit:          ingredient.Unit,
		Quantity:      ingredient.CurrentStock,
		FullStock:     ingredient.FullStock,
		AlertQuantity: ingredient.AlertQuantity,
		IsLow:         ingredientIsLowStock(ingredient.CurrentStock, ingredient.AlertQuantity),
	}
	publishEvent(restaurantID, "inventory_updated", data)
	log.Printf("📤 Broadcast ingredient inventory: %s (stock: %.2f %s) to room %s",
		ingredient.Name, ingredient.CurrentStock, ingredient.Unit, restaurantID)
}

// BroadcastIngredientInventoryUpdates broadcasts multiple ingredient rows.
func BroadcastIngredientInventoryUpdates(hub *WebSocketHub, restaurantID string, ingredients []models.Ingredient) {
	for _, ingredient := range ingredients {
		BroadcastIngredientInventoryUpdate(hub, restaurantID, ingredient)
	}
}

// Helper to convert struct to JSON
func toJSON(v interface{}) []byte {
	data, _ := json.Marshal(v)
	return data
}
