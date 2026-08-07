package services

import "sync"

// sharedPrintNotifyHub is process-wide so every PrintService (orders + print routes)
// wakes the same SSE subscribers.
var sharedPrintNotifyHub = NewPrintNotifyHub()

// PrintNotifyHub wakes print-agent SSE subscribers when jobs are enqueued for a restaurant.
type PrintNotifyHub struct {
	mu   sync.RWMutex
	subs map[string]map[chan struct{}]struct{}
}

func NewPrintNotifyHub() *PrintNotifyHub {
	return &PrintNotifyHub{
		subs: make(map[string]map[chan struct{}]struct{}),
	}
}

// SharedPrintNotifyHub returns the process-wide print wake hub.
func SharedPrintNotifyHub() *PrintNotifyHub {
	return sharedPrintNotifyHub
}

func (h *PrintNotifyHub) Subscribe(restaurantID string) chan struct{} {
	ch := make(chan struct{}, 1)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subs[restaurantID] == nil {
		h.subs[restaurantID] = make(map[chan struct{}]struct{})
	}
	h.subs[restaurantID][ch] = struct{}{}
	return ch
}

func (h *PrintNotifyHub) Unsubscribe(restaurantID string, ch chan struct{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if subs, ok := h.subs[restaurantID]; ok {
		delete(subs, ch)
		if len(subs) == 0 {
			delete(h.subs, restaurantID)
		}
	}
	close(ch)
}

// Notify signals agents listening for this restaurant that jobs may be ready.
func (h *PrintNotifyHub) Notify(restaurantID string) {
	if h == nil || restaurantID == "" {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subs[restaurantID] {
		select {
		case ch <- struct{}{}:
		default:
			// Already has a pending wake; coalesce.
		}
	}
}
