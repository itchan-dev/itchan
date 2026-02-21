package handler

import (
	"fmt"
	"io"
	"net/http"

	"github.com/itchan-dev/itchan/shared/logger"
)

func (h *Handler) RegisterGetHandler(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, r, "register.html", nil)
}

func (h *Handler) RegisterPostHandler(w http.ResponseWriter, r *http.Request) {
	targetURL := "/register"
	successURL := "/check_confirmation_code"

	email := r.FormValue("email")
	password := r.FormValue("password")

	resp, err := h.APIClient.Register(email, password)
	if err != nil {
		logger.Log.Error("during registration API call", "error", err)
		h.redirectWithFlash(w, r, targetURL, flashCookieError, "Internal error: backend unavailable.")
		return
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusTooEarly {
		msg := string(bodyBytes) + " Please check your email or use the confirmation page."
		h.setFlash(w, flashCookieError, msg)
		h.setFlash(w, emailPrefillCookie, email)
		http.Redirect(w, r, "/check_confirmation_code", http.StatusSeeOther)
		return
	}

	if resp.StatusCode != http.StatusOK {
		h.redirectWithFlash(w, r, targetURL, flashCookieError, string(bodyBytes))
		return
	}

	h.redirectWithFlash(w, r, successURL, emailPrefillCookie, email)
}

func (h *Handler) ConfirmEmailGetHandler(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, r, "check_confirmation_code.html", nil)
}

func (h *Handler) ConfirmEmailPostHandler(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	code := r.FormValue("confirmation_code")

	err := h.APIClient.ConfirmEmail(email, code)
	if err != nil {
		logger.Log.Error("confirming email via API", "error", err)
		h.setFlash(w, flashCookieError, err.Error())
		h.setFlash(w, emailPrefillCookie, email)
		http.Redirect(w, r, "/check_confirmation_code", http.StatusSeeOther)
		return
	}

	h.setFlash(w, flashCookieSuccess, "Success! You can now login.")
	h.setFlash(w, emailPrefillCookie, email)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *Handler) LoginGetHandler(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, r, "login.html", nil)
}

func (h *Handler) LoginPostHandler(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	password := r.FormValue("password")

	resp, err := h.APIClient.Login(email, password)
	if err != nil {
		logger.Log.Error("during login API call", "error", err)
		h.setFlash(w, flashCookieError, "Internal error: backend unavailable.")
		h.setFlash(w, emailPrefillCookie, email)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		h.setFlash(w, flashCookieError, string(bodyBytes))
		h.setFlash(w, emailPrefillCookie, email)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	for _, cookie := range resp.Cookies() {
		http.SetCookie(w, cookie)
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie := &http.Cookie{
		Path:     "/",
		Name:     "accessToken",
		Value:    "",
		MaxAge:   -1, // Expire immediately
		HttpOnly: true,
		Secure:   h.Public.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *Handler) RegisterInviteGetHandler(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, r, "register_invite.html", nil)
}

func (h *Handler) RegisterInvitePostHandler(w http.ResponseWriter, r *http.Request) {
	targetURL := "/register_invite"

	inviteCode := r.FormValue("invite_code")
	password := r.FormValue("password")

	email, err := h.APIClient.RegisterWithInvite(inviteCode, password)
	if err != nil {
		logger.Log.Error("during invite registration API call", "error", err)
		h.redirectWithFlash(w, r, targetURL, flashCookieError, err.Error())
		return
	}

	successMsg := fmt.Sprintf("Registration successful! Your email is: %s. Please save this - it cannot be recovered!", email)
	h.setFlash(w, flashCookieSuccess, successMsg)
	h.setFlash(w, emailPrefillCookie, email)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
