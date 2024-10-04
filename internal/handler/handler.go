package handler

import (
	"net/http"

	"encoding/json"
	"log"

	"github.com/itchan-dev/itchan/internal/config"
	"github.com/itchan-dev/itchan/internal/models/auth"
	"github.com/itchan-dev/itchan/internal/models/board"
)

type handler struct {
	auth  *auth.Auth
	board *board.Board
	cfg   *config.Config
}

func New(auth *auth.Auth, board *board.Board, cfg *config.Config) *handler {
	return &handler{auth, board, cfg}
}

func (h *handler) Test(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("TESTING"))
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(v)
	if err != nil {
		log.Print(err.Error())
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}
