package handler

import (
	"net/http"

	"encoding/json"
	"log"

	"github.com/itchan-dev/itchan/backend/internal/service"
	"github.com/itchan-dev/itchan/backend/internal/utils/jwt"
	"github.com/itchan-dev/itchan/shared/config"
)

type handler struct {
	auth    service.AuthService
	board   service.BoardService
	thread  *service.Thread
	message *service.Message
	cfg     *config.Config
	jwt     *jwt.Jwt
}

func New(auth service.AuthService, board service.BoardService, thread *service.Thread, message *service.Message, cfg *config.Config, jwt *jwt.Jwt) *handler {
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
