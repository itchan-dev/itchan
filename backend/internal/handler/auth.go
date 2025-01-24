package handler

import (
	"net/http"

	"github.com/itchan-dev/itchan/backend/internal/utils"
)

type credentials struct {
	Email    string `validate:"required" json:"email"`
	Password string `validate:"required" json:"password"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var creds credentials
	if err := LoadAndValidateRequestBody(r, &creds); err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	if err := h.auth.Register(creds.Email, creds.Password); err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("The confirmation code has been sent by email"))
}

func (h *Handler) CheckConfirmationCode(w http.ResponseWriter, r *http.Request) {
	var reqBody struct {
		Email            string `validate:"required" json:"email"`
		ConfirmationCode string `validate:"required" json:"confirmation_code"`
	}
	if err := LoadAndValidateRequestBody(r, &reqBody); err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	if err := h.auth.CheckConfirmationCode(reqBody.Email, reqBody.ConfirmationCode); err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var creds credentials
	if err := LoadAndValidateRequestBody(r, &creds); err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	accessToken, err := h.auth.Login(creds.Email, creds.Password)
	if err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}
	cookie := &http.Cookie{
		Path:     "/",
		Name:     "accessToken",
		Value:    accessToken,
		MaxAge:   int(h.cfg.JwtTTL().Seconds()),
		HttpOnly: true,
	}
	http.SetCookie(w, cookie)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("You logged in"))
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie := &http.Cookie{
		Path:     "/",
		Name:     "accessToken",
		Value:    "",
		MaxAge:   -1,
		HttpOnly: true,
	}
	http.SetCookie(w, cookie)

	w.WriteHeader(http.StatusOK)
}
