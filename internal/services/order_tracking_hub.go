package services

import "sync"

// OrderTrackingHub fans out readiness updates to customer SSE subscribers per tracking token.
type OrderTrackingHub struct {
	mu   sync.RWMutex
	subs map[string]map[chan TrackingStatus]struct{}
}

func NewOrderTrackingHub() *OrderTrackingHub {
	return &OrderTrackingHub{
		subs: make(map[string]map[chan TrackingStatus]struct{}),
	}
}

func (h *OrderTrackingHub) Subscribe(token string) chan TrackingStatus {
	ch := make(chan TrackingStatus, 4)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subs[token] == nil {
		h.subs[token] = make(map[chan TrackingStatus]struct{})
	}
	h.subs[token][ch] = struct{}{}
	return ch
}

func (h *OrderTrackingHub) Unsubscribe(token string, ch chan TrackingStatus) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if subs, ok := h.subs[token]; ok {
		delete(subs, ch)
		if len(subs) == 0 {
			delete(h.subs, token)
		}
	}
	close(ch)
}

func (h *OrderTrackingHub) Publish(token string, status TrackingStatus) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subs[token] {
		select {
		case ch <- status:
		default:
		}
	}
}
