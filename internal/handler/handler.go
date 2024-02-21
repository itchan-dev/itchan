package handler

import (
	"net/http"

	"github.com/itchan-dev/itchan/internal/config"
	"github.com/itchan-dev/itchan/internal/logic/auth"
)

type handler struct {
	auth *auth.Auth
	cfg  *config.Config
}

func New(storage *auth.Auth, cfg *config.Config) *handler {
	return &handler{storage, cfg}
}

func (h *handler) Test(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("TESTING"))
}
