package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"restaurant-api/internal/models"

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
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now (configure in production)
	},
}

// Global hub instance for broadcasting
var globalHub *WebSocketHub

// SetGlobalHub sets the global WebSocket hub
func SetGlobalHub(hub *WebSocketHub) {
	globalHub = hub
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
	h.mu.RUnlock()

	if !exists {
		return
	}

	for _, client := range room {
		select {
		case client.send <- message:
		default:
			log.Printf("⚠️  Client send buffer full for %s in room %s", client.userID, roomID)
		}
	}
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

// BroadcastOrderUpdate broadcasts enhanced order creation/update with full details
func BroadcastOrderUpdate(hub *WebSocketHub, restaurantID string, order *models.Order) {
	tableOccupied := order.Status != "cancelled" && order.Status != "completed"
	
	event := models.NotificationEvent{
		Type:      "order_created",
		RoomID:    restaurantID,
		Timestamp: time.Now(),
		Data: json.RawMessage(toJSON(models.OrderEventData{
			OrderID:       order.ID,
			OrderNumber:   order.OrderNumber,
			TableID:       order.TableID,
			TableNo:       order.TableNumber,
			TableOccupied: tableOccupied,
			Status:        order.Status,
			SubTotal:      order.SubTotal,
			TaxAmount:     order.TaxAmount,
			TotalAmount:   order.Total,
			ItemCount:     len(order.Items),
			Items:         order.Items,
		})),
	}
	hub.BroadcastToRoom(restaurantID, event)
	log.Printf("📤 Broadcast order update: Order #%d (Table: %s, Occupied: %v) to room %s", order.OrderNumber, order.TableNumber, tableOccupied, restaurantID)
}

// BroadcastTableUpdate broadcasts table status changes (occupied/empty)
func BroadcastTableUpdate(hub *WebSocketHub, restaurantID string, table *models.RestaurantTable) {
	event := models.NotificationEvent{
		Type:      "table_status_changed",
		RoomID:    restaurantID,
		Timestamp: time.Now(),
		Data: json.RawMessage(toJSON(models.TableEventData{
			TableID:        table.ID,
			TableNumber:    table.Name,
			IsOccupied:     table.IsOccupied,
			CurrentOrderID: table.CurrentOrderID,
		})),
	}
	hub.BroadcastToRoom(restaurantID, event)
	log.Printf("📤 Broadcast table update: Table %s (Occupied: %v) to room %s", table.Name, table.IsOccupied, restaurantID)
}

// BroadcastInventoryUpdate broadcasts inventory changes
func BroadcastInventoryUpdate(hub *WebSocketHub, restaurantID string, itemName string, quantity float64, isLow bool) {
	event := models.NotificationEvent{
		Type:      "inventory_updated",
		RoomID:    restaurantID,
		Timestamp: time.Now(),
		Data: json.RawMessage(toJSON(models.InventoryEventData{
			ItemName: itemName,
			Quantity: quantity,
			IsLow:    isLow,
		})),
	}
	hub.BroadcastToRoom(restaurantID, event)
	log.Printf("📤 Broadcast inventory update: %s (Qty: %.2f) to room %s", itemName, quantity, restaurantID)
}

// Helper to convert struct to JSON
func toJSON(v interface{}) []byte {
	data, _ := json.Marshal(v)
	return data
}
