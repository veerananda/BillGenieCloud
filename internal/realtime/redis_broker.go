package realtime

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	"restaurant-api/internal/models"

	"github.com/redis/go-redis/v9"
)

const redisChannelPrefix = "billgenie:ws:"

// RoomBroadcaster delivers events to connected WebSocket clients in a room.
type RoomBroadcaster interface {
	BroadcastToRoom(roomID string, message interface{})
}

// RedisBroker publishes WebSocket events to Redis so multiple API instances stay in sync.
type RedisBroker struct {
	client      *redis.Client
	broadcaster RoomBroadcaster
	mu          sync.Mutex
	running     bool
}

// NewRedisBroker connects to Redis when REDIS_URL is set. Returns nil if unset.
func NewRedisBroker(broadcaster RoomBroadcaster) *RedisBroker {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		log.Println("ℹ️  REDIS_URL not set — WebSocket fan-out is local-only (single instance)")
		return nil
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Printf("⚠️  Invalid REDIS_URL: %v — continuing without Redis", err)
		return nil
	}

	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("⚠️  Redis ping failed: %v — continuing without Redis", err)
		return nil
	}

	broker := &RedisBroker{
		client:      client,
		broadcaster: broadcaster,
	}
	broker.Start()
	log.Println("✅ Redis pub/sub connected for WebSocket fan-out")
	return broker
}

// Start subscribes to all restaurant channels on this instance.
func (b *RedisBroker) Start() {
	b.mu.Lock()
	if b.running {
		b.mu.Unlock()
		return
	}
	b.running = true
	b.mu.Unlock()

	go func() {
		ctx := context.Background()
		pubsub := b.client.PSubscribe(ctx, redisChannelPrefix+"*")
		defer pubsub.Close()

		ch := pubsub.Channel()
		for msg := range ch {
			var event models.NotificationEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				log.Printf("⚠️  Redis WS message parse error: %v", err)
				continue
			}
			if event.RoomID != "" && b.broadcaster != nil {
				b.broadcaster.BroadcastToRoom(event.RoomID, event)
			}
		}
	}()
}

// Publish sends an event through Redis; subscribers on every instance (including this one) deliver to local WebSocket clients.
func (b *RedisBroker) Publish(roomID string, event models.NotificationEvent) {
	if b == nil || b.client == nil {
		if b != nil && b.broadcaster != nil {
			b.broadcaster.BroadcastToRoom(roomID, event)
		}
		return
	}

	payload, err := json.Marshal(event)
	if err != nil {
		log.Printf("⚠️  Redis publish marshal error: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	channel := redisChannelPrefix + roomID
	if err := b.client.Publish(ctx, channel, payload).Err(); err != nil {
		log.Printf("⚠️  Redis publish error: %v", err)
		// Fallback to local broadcast if Redis fails
		if b.broadcaster != nil {
			b.broadcaster.BroadcastToRoom(roomID, event)
		}
	}
}
