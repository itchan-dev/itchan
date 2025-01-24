package handler

import (
	"net/http"

	"encoding/json"
	"log"

	"github.com/go-playground/validator/v10"
	internal_errors "github.com/itchan-dev/itchan/backend/internal/errors"
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

var validate *validator.Validate

func LoadAndValidateRequestBody(r *http.Request, body any) error {
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
