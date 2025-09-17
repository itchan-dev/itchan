package handler

import (
	"net/http"

	"encoding/json"
	"log"

	"github.com/itchan-dev/itchan/backend/internal/service"
	"github.com/itchan-dev/itchan/shared/config"
)

type Handler struct {
	auth    service.AuthService
	board   service.BoardService
	thread  service.ThreadService
	message service.MessageService
	cfg     *config.Config
}

func New(auth service.AuthService, board service.BoardService, thread service.ThreadService, message service.MessageService, cfg *config.Config) *Handler {
	return &Handler{auth, board, thread, message, cfg}
}

func (h *Handler) Test(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("TESTING"))
}

func writeJSON(w http.ResponseWriter, v any) {
	err := json.NewEncoder(w).Encode(v)
	if err != nil {
		log.Print(err.Error())
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
}

// GetPublicConfig exposes the public part of the configuration for clients (frontend)
func (h *Handler) GetPublicConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, h.cfg.Public)
}
