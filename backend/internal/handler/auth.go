package handler

import (
	"net/http"

	"github.com/itchan-dev/itchan/shared/api"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/itchan-dev/itchan/shared/utils"
)

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req api.RegisterRequest
	if err := utils.DecodeValidate(r.Body, &req); err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	if err := h.auth.Register(domain.Credentials{Email: req.Email, Password: req.Password}); err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	writeJSON(w, api.RegisterResponse{Message: "The confirmation code has been sent by email"})
}

func (h *Handler) CheckConfirmationCode(w http.ResponseWriter, r *http.Request) {
	var req api.CheckConfirmationCodeRequest
	if err := utils.DecodeValidate(r.Body, &req); err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	if err := h.auth.CheckConfirmationCode(req.Email, req.ConfirmationCode); err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req api.LoginRequest
	if err := utils.DecodeValidate(r.Body, &req); err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	accessToken, err := h.auth.Login(domain.Credentials{Email: req.Email, Password: req.Password})
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
		Secure:   h.cfg.Public.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)

	writeJSON(w, api.LoginResponse{Message: "You logged in"})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie := &http.Cookie{
		Path:     "/",
		Name:     "accessToken",
		Value:    "",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cfg.Public.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)

	writeJSON(w, api.LogoutResponse{Message: "You logged out"})
}
