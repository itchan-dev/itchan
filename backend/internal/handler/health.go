package handler

import (
	"context"
	"net/http"
	"time"
)

// Health is a liveness probe endpoint.
// Returns 200 OK if the server is running.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// Ready is a readiness probe endpoint.
// Returns 200 OK if the server can handle requests (DB is connected).
// Returns 503 Service Unavailable if dependencies are not ready.
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Use a short timeout for health checks
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := h.health.Ping(ctx); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("database unavailable"))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
