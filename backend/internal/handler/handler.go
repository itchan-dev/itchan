package handler

import (
	"net/http"

	"encoding/json"
	"log"

	"github.com/go-playground/validator/v10"
	internal_errors "github.com/itchan-dev/itchan/backend/internal/errors"
	"github.com/itchan-dev/itchan/backend/internal/service"
	"github.com/itchan-dev/itchan/backend/internal/utils/jwt"
	"github.com/itchan-dev/itchan/shared/config"
)

type handler struct {
	auth    service.AuthService
	board   service.BoardService
	thread  service.ThreadService
	message service.MessageService
	cfg     *config.Config
	jwt     *jwt.Jwt
}

func New(auth service.AuthService, board service.BoardService, thread service.ThreadService, message service.MessageService, cfg *config.Config, jwt *jwt.Jwt) *handler {
	return &handler{auth, board, thread, message, cfg, jwt}
}

func (h *handler) Test(w http.ResponseWriter, r *http.Request) {
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
	w.WriteHeader(http.StatusOK)
}

var validate *validator.Validate

func loadAndValidateRequestBody(r *http.Request, body any) error {
	if err := json.NewDecoder(r.Body).Decode(body); err != nil {
		log.Print(err.Error())
		return &internal_errors.ErrorWithStatusCode{Message: "Body is invalid json", StatusCode: 400}
	}
	validate = validator.New(validator.WithRequiredStructEnabled())
	if err := validate.Struct(body); err != nil {
		log.Print(err.Error())
		return &internal_errors.ErrorWithStatusCode{Message: "Required fields missing", StatusCode: 400}
	}
	return nil
}

func writeErrorAndStatusCode(w http.ResponseWriter, err error) {
	if e, ok := err.(*internal_errors.ErrorWithStatusCode); ok {
		http.Error(w, err.Error(), e.StatusCode)
	}
	// default error is 500
	http.Error(w, "Internal server error", http.StatusInternalServerError)
}
