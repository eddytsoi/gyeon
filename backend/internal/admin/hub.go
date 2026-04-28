package admin

import (
	"encoding/json"
	"fmt"
	"sync"
)

// Hub is an in-memory broadcast channel for admin SSE clients. Each subscriber
// gets a buffered channel; if a slow client's buffer is full, messages are
// dropped for that client rather than blocking the broadcaster.
type Hub struct {
	mu      sync.RWMutex
	clients map[chan []byte]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: make(map[chan []byte]struct{})}
}

func (h *Hub) Subscribe() chan []byte {
	ch := make(chan []byte, 16)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *Hub) Unsubscribe(ch chan []byte) {
	h.mu.Lock()
	if _, ok := h.clients[ch]; ok {
		delete(h.clients, ch)
		close(ch)
	}
	h.mu.Unlock()
}

// Broadcast formats the data as an SSE event frame and sends it to every
// subscriber. Frame ends with the required blank line so clients flush it.
func (h *Hub) Broadcast(eventType string, data any) {
	payload, err := json.Marshal(data)
	if err != nil {
		return
	}
	msg := []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, payload))
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.clients {
		select {
		case ch <- msg:
		default:
			// drop on slow client; don't block the broadcaster
		}
	}
}
