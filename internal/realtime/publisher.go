package realtime

import "restaurant-api/internal/models"

// EventBridge routes events through Redis (multi-instance) or the local hub only.
type EventBridge struct {
	broker *RedisBroker
	hub    RoomBroadcaster
}

// NewEventBridge creates a publisher; uses Redis when REDIS_URL is set.
func NewEventBridge(hub RoomBroadcaster) *EventBridge {
	broker := NewRedisBroker(hub)
	return &EventBridge{broker: broker, hub: hub}
}

// Publish delivers an event to all WebSocket clients (via Redis or local hub).
func (e *EventBridge) Publish(roomID string, event models.NotificationEvent) {
	if e.broker != nil {
		e.broker.Publish(roomID, event)
		return
	}
	if e.hub != nil {
		e.hub.BroadcastToRoom(roomID, event)
	}
}
