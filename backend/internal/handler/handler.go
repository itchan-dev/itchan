package handler

import (
	"net/http"

	"encoding/json"
	"log"

	"github.com/itchan-dev/itchan/backend/internal/service"
	"github.com/itchan-dev/itchan/shared/config"
)

type Handler struct {
	auth         service.AuthService
	board        service.BoardService
	thread       service.ThreadService
	message      service.MessageService
	mediaStorage service.MediaStorage
	cfg          *config.Config
}

func New(auth service.AuthService, board service.BoardService, thread service.ThreadService, message service.MessageService, mediaStorage service.MediaStorage, cfg *config.Config) *Handler {
	return &Handler{
		auth:         auth,
		board:        board,
		thread:       thread,
		message:      message,
		mediaStorage: mediaStorage,
		cfg:          cfg,
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
		log.Print(err.Error())
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
}

// GetPublicConfig exposes the public part of the configuration for clients (frontend)
func (h *Handler) GetPublicConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, h.cfg.Public)
}
