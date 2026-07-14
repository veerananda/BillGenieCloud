package services

import "sync"

// AssistanceHub fans out table assistance status to customer SSE subscribers per assistance token.
type AssistanceHub struct {
	mu   sync.RWMutex
	subs map[string]map[chan AssistanceStatus]struct{}
}

func NewAssistanceHub() *AssistanceHub {
	return &AssistanceHub{
		subs: make(map[string]map[chan AssistanceStatus]struct{}),
	}
}

func (h *AssistanceHub) Subscribe(token string) chan AssistanceStatus {
	ch := make(chan AssistanceStatus, 4)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subs[token] == nil {
		h.subs[token] = make(map[chan AssistanceStatus]struct{})
	}
	h.subs[token][ch] = struct{}{}
	return ch
}

func (h *AssistanceHub) Unsubscribe(token string, ch chan AssistanceStatus) {
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

func (h *AssistanceHub) Publish(token string, status AssistanceStatus) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subs[token] {
		select {
		case ch <- status:
		default:
		}
	}
}
