package handler

import (
	"net/http"

	"github.com/itchan-dev/itchan/backend/internal/utils"
)

type credentials struct {
	Email    string `validate:"required" json:"email"`
	Password string `validate:"required" json:"password"`
}

func (h *handler) Signup(w http.ResponseWriter, r *http.Request) {
	var creds credentials
	if err := loadAndValidateRequestBody(r, &creds); err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	_, err := h.auth.Signup(creds.Email, creds.Password)
	if err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("Created. You can login now"))
}

func (h *handler) Login(w http.ResponseWriter, r *http.Request) {
	var creds credentials
	if err := loadAndValidateRequestBody(r, &creds); err != nil {
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

func (h handler) Logout(w http.ResponseWriter, r *http.Request) {
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
