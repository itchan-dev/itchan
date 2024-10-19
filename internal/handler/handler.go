package handler

import (
	"net/http"

	"encoding/json"
	"log"

	"github.com/itchan-dev/itchan/internal/config"
	"github.com/itchan-dev/itchan/internal/models/auth"
	"github.com/itchan-dev/itchan/internal/models/board"
	"github.com/itchan-dev/itchan/internal/models/message"
	"github.com/itchan-dev/itchan/internal/models/thread"
	"github.com/itchan-dev/itchan/internal/scripts/jwt"
)

type handler struct {
	auth    *auth.Auth
	board   *board.Board
	thread  *thread.Thread
	message *message.Message
	cfg     *config.Config
	jwt     *jwt.Jwt
}

func New(auth *auth.Auth, board *board.Board, thread *thread.Thread, message *message.Message, cfg *config.Config, jwt *jwt.Jwt) *handler {
	return &handler{auth, board, thread, message, cfg, jwt}
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
