package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/itchan-dev/itchan/shared/api"
	"github.com/itchan-dev/itchan/shared/logger"
	"github.com/itchan-dev/itchan/shared/middleware"
)

func (h *Handler) RegisterGetHandler(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, r, "register.html", nil)
}

func (h *Handler) RegisterPostHandler(w http.ResponseWriter, r *http.Request) {
	targetURL := "/register"
	successURL := "/check_confirmation_code"

	email := r.FormValue("email")
	password := r.FormValue("password")

	resp, err := h.APIClient.Register(r, email, password)
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

	refSource := getRefCookie(r)

	err := h.APIClient.ConfirmEmail(r, email, code, refSource)
	if err != nil {
		logger.Log.Error("confirming email via API", "error", err)
		h.setFlash(w, flashCookieError, err.Error())
		h.setFlash(w, emailPrefillCookie, email)
		http.Redirect(w, r, "/check_confirmation_code", http.StatusSeeOther)
		return
	}

	clearRefCookie(w, h.Public.SecureCookies)
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

	resp, err := h.APIClient.Login(r, email, password)
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

	var loginResp api.LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil || loginResp.AccessToken == "" {
		logger.Log.Error("parsing login response", "error", err)
		h.setFlash(w, flashCookieError, "Internal error: invalid login response.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Path:     "/",
		Name:     middleware.CookieName,
		Value:    loginResp.AccessToken,
		MaxAge:   int(h.Public.JwtTTL.Seconds()),
		HttpOnly: true,
		Secure:   h.Public.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Path:     "/",
		Name:     middleware.CookieName,
		Value:    "",
		MaxAge:   -1, // Expire immediately
		HttpOnly: true,
		Secure:   h.Public.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *Handler) RegisterInviteGetHandler(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, r, "register_invite.html", nil)
}

func (h *Handler) RegisterInvitePostHandler(w http.ResponseWriter, r *http.Request) {
	targetURL := "/register_invite"

	inviteCode := r.FormValue("invite_code")
	password := r.FormValue("password")

	refSource := getRefCookie(r)

	email, err := h.APIClient.RegisterWithInvite(r, inviteCode, password, refSource)
	if err != nil {
		logger.Log.Error("during invite registration API call", "error", err)
		h.redirectWithFlash(w, r, targetURL, flashCookieError, err.Error())
		return
	}

	clearRefCookie(w, h.Public.SecureCookies)
	successMsg := fmt.Sprintf("Registration successful! Your email is: %s. Please save this - it cannot be recovered!", email)
	h.setFlash(w, flashCookieSuccess, successMsg)
	h.setFlash(w, emailPrefillCookie, email)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
