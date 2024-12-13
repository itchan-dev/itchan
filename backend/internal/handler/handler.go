package handler

import (
	"errors"
	"fmt"
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
	message *service.Message
	cfg     *config.Config
	jwt     *jwt.Jwt
}

func New(auth service.AuthService, board service.BoardService, thread service.ThreadService, message *service.Message, cfg *config.Config, jwt *jwt.Jwt) *handler {
	return &handler{auth, board, thread, message, cfg, jwt}
}

func (h *handler) Test(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("TESTING"))
}

// would be cool to implement as a handler method, but generic methods isnt allowed in golang
func getFieldFromCookie[T any](h *handler, cookie *http.Cookie, field string) (*T, error) {
	jwtClaims, err := h.jwt.DecodeToken(cookie.Value)
	if err != nil {
		log.Print(err.Error())
		return nil, errors.New("Can't decode accessToken cookie")
	}
	value, ok := jwtClaims[field].(T)
	if !ok {
		return nil, errors.New(fmt.Sprintf("Cant find %s in jwtClaim", field))
	}
	return &value, nil
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
