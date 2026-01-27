package handler

import (
	"fmt"
	"html/template"
	"io"
	"net/http"

	"github.com/itchan-dev/itchan/shared/logger"
)

func (h *Handler) RegisterGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		CommonTemplateData
	}
	templateData.CommonTemplateData = h.InitCommonTemplateData(w, r)

	h.renderTemplate(w, "register.html", templateData)
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

	// Handle specific case: confirmation needed
	if resp.StatusCode == http.StatusTooEarly {
		// Safely construct message without XSS risk - use template escaping
		msg := template.HTMLEscapeString(string(bodyBytes)) + " Please check your email or use the confirmation page."
		// Redirect to confirmation page with email pre-filled (via cookie) and error message
		h.setFlash(w, flashCookieError, msg)
		h.setFlash(w, emailPrefillCookie, email)
		http.Redirect(w, r, "/check_confirmation_code", http.StatusSeeOther)
		return
	}

	if resp.StatusCode != http.StatusOK {
		h.redirectWithFlash(w, r, targetURL, flashCookieError, template.HTMLEscapeString(string(bodyBytes)))
		return
	}

	// Success (StatusOK): Redirect to confirmation page with email pre-filled (via cookie)W
	h.redirectWithFlash(w, r, successURL, emailPrefillCookie, email)
}

func (h *Handler) ConfirmEmailGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		CommonTemplateData
	}
	templateData.CommonTemplateData = h.InitCommonTemplateData(w, r)

	h.renderTemplate(w, "check_confirmation_code.html", templateData)
}

func (h *Handler) ConfirmEmailPostHandler(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	code := r.FormValue("confirmation_code")

	err := h.APIClient.ConfirmEmail(email, code)
	if err != nil {
		logger.Log.Error("confirming email via API", "error", err)
		h.setFlash(w, flashCookieError, template.HTMLEscapeString(err.Error()))
		h.setFlash(w, emailPrefillCookie, email)
		http.Redirect(w, r, "/check_confirmation_code", http.StatusSeeOther)
		return
	}

	h.setFlash(w, flashCookieSuccess, `Success! You can now <a href="/login">login</a>.`)
	h.setFlash(w, emailPrefillCookie, email)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *Handler) LoginGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		CommonTemplateData
	}
	templateData.CommonTemplateData = h.InitCommonTemplateData(w, r)

	h.renderTemplate(w, "login.html", templateData)
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
		h.setFlash(w, flashCookieError, template.HTMLEscapeString(string(bodyBytes)))
		h.setFlash(w, emailPrefillCookie, email)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Success: Forward cookies from the backend response to the user's browser
	for _, cookie := range resp.Cookies() {
		http.SetCookie(w, cookie)
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	// Clear the access token cookie
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

	// Redirect to login page
	http.Redirect(w, r, "/login", http.StatusSeeOther) // Use SeeOther after logout action
}

func (h *Handler) RegisterInviteGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		CommonTemplateData
	}
	templateData.CommonTemplateData = h.InitCommonTemplateData(w, r)

	h.renderTemplate(w, "register_invite.html", templateData)
}

func (h *Handler) RegisterInvitePostHandler(w http.ResponseWriter, r *http.Request) {
	targetURL := "/register_invite"

	inviteCode := r.FormValue("invite_code")
	password := r.FormValue("password")

	email, err := h.APIClient.RegisterWithInvite(inviteCode, password)
	if err != nil {
		logger.Log.Error("during invite registration API call", "error", err)
		h.redirectWithFlash(w, r, targetURL, flashCookieError, template.HTMLEscapeString(err.Error()))
		return
	}

	// Success: Redirect to login page with generated email pre-filled (via cookie) and success message
	// Note: HTML tags are intentional, but escape the email value for safety
	successMsg := fmt.Sprintf("<strong>Registration successful!</strong> Your email is: <strong>%s</strong><br>Please save this - it cannot be recovered!", template.HTMLEscapeString(email))
	h.setFlash(w, flashCookieSuccess, successMsg)
	h.setFlash(w, emailPrefillCookie, email)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
