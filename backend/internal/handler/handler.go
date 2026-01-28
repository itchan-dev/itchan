package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/itchan-dev/itchan/backend/internal/service"
	"github.com/itchan-dev/itchan/backend/internal/storage/fs"
	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/logger"
)

// HealthChecker is used by health endpoints to verify dependencies.
type HealthChecker interface {
	Ping(ctx context.Context) error
}

type Handler struct {
	auth         service.AuthService
	board        service.BoardService
	thread       service.ThreadService
	message      service.MessageService
	userActivity service.UserActivityService
	mediaStorage fs.MediaStorage
	cfg          *config.Config
	health       HealthChecker
}

func New(auth service.AuthService, board service.BoardService, thread service.ThreadService, message service.MessageService, userActivity service.UserActivityService, mediaStorage fs.MediaStorage, cfg *config.Config, health HealthChecker) *Handler {
	return &Handler{
		auth:         auth,
		board:        board,
		thread:       thread,
		message:      message,
		userActivity: userActivity,
		mediaStorage: mediaStorage,
		cfg:          cfg,
		health:       health,
	}
}

func (h *Handler) Test(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("TESTING"))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(v)
	if err != nil {
		logger.Log.Error("failed to encode json response", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
}

// GetPublicConfig exposes the public part of the configuration for clients (frontend)
func (h *Handler) GetPublicConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, h.cfg.Public)
}
