package admin

import (
	"fmt"
	"net/http"
	"time"

	"gyeon/backend/internal/auth"
)

type EventsHandler struct {
	hub       *Hub
	jwtSecret string
}

func NewEventsHandler(hub *Hub, jwtSecret string) *EventsHandler {
	return &EventsHandler{hub: hub, jwtSecret: jwtSecret}
}

// Stream upgrades the request to an SSE stream and pipes broadcasts from the
// hub to the client. Auth uses a `?token=` query param because the browser's
// EventSource API can't set custom headers.
func (h *EventsHandler) Stream(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}
	if _, err := auth.ValidateToken(tokenStr, h.jwtSecret); err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := h.hub.Subscribe()
	defer h.hub.Unsubscribe(ch)

	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if _, err := w.Write(msg); err != nil {
				return
			}
			flusher.Flush()
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
