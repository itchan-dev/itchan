package handler

import (
	"github.com/itchan-dev/itchan/shared/logger"
	"fmt"
	"html/template"
	"io"

	"net/http"
	"net/url"

	"github.com/itchan-dev/itchan/shared/domain"
	mw "github.com/itchan-dev/itchan/shared/middleware"
)

func (h *Handler) RegisterGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		Error      template.HTML
		User       *domain.User
		Validation ValidationData
	}
	templateData.User = mw.GetUserFromContext(r)
	templateData.Error, _ = parseMessagesFromQuery(r)
	templateData.Validation = h.NewValidationData()

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
		redirectWithParams(w, r, targetURL, map[string]string{"error": "Internal error: backend unavailable."})
		return
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	// Handle specific case: confirmation needed
	if resp.StatusCode == http.StatusTooEarly {
		// Safely construct message without XSS risk - use template escaping
		msg := string(bodyBytes) + " Please check your email or use the confirmation page."
		// Pass email separately for URL construction in the redirect
		targetWithEmail := fmt.Sprintf("/check_confirmation_code?email=%s", url.QueryEscape(email))
		redirectWithParams(w, r, targetURL, map[string]string{"error": msg, "redirect_to": targetWithEmail})
		return
	}

	if resp.StatusCode != http.StatusOK {
		redirectWithParams(w, r, targetURL, map[string]string{"error": string(bodyBytes)})
		return
	}

	// Success (StatusOK): Redirect to confirmation page with email pre-filled
	finalSuccessURL := fmt.Sprintf("%s?email=%s", successURL, url.QueryEscape(email))
	http.Redirect(w, r, finalSuccessURL, http.StatusSeeOther)
}

func (h *Handler) ConfirmEmailGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		Error            template.HTML
		Success          template.HTML
		EmailPlaceholder string
		User             *domain.User
		Validation       ValidationData
	}
	templateData.User = mw.GetUserFromContext(r)
	templateData.EmailPlaceholder = parseEmail(r)                        // Get email from query param
	templateData.Error, templateData.Success = parseMessagesFromQuery(r) // Get messages
	templateData.Validation = h.NewValidationData()

	// Customize success message if needed based on query param
	if r.URL.Query().Get("success") == "confirmed" {
		templateData.Success = template.HTML(`Success! You can now <a href="/login">login</a>.`)
	}

	h.renderTemplate(w, "check_confirmation_code.html", templateData)
}

func (h *Handler) ConfirmEmailPostHandler(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	code := r.FormValue("confirmation_code")
	targetURL := fmt.Sprintf("/check_confirmation_code?email=%s", url.QueryEscape(email))

	err := h.APIClient.ConfirmEmail(email, code)
	if err != nil {
		logger.Log.Error("confirming email via API", "error", err)
		redirectWithParams(w, r, targetURL, map[string]string{"error": err.Error()})
		return
	}

	redirectWithParams(w, r, targetURL, map[string]string{"success": "confirmed"})
}

func (h *Handler) LoginGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		Error            template.HTML
		User             *domain.User
		EmailPlaceholder string
		Validation       ValidationData
	}
	templateData.User = mw.GetUserFromContext(r)
	templateData.Error, _ = parseMessagesFromQuery(r)          // Get error message
	templateData.EmailPlaceholder = r.URL.Query().Get("email") // Pre-fill email if redirected with it
	if templateData.EmailPlaceholder == "" {
		// Fallback if not passed in query
		templateData.EmailPlaceholder = parseEmail(r)
	}
	templateData.Validation = h.NewValidationData()

	h.renderTemplate(w, "login.html", templateData)
}

func (h *Handler) LoginPostHandler(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	password := r.FormValue("password")
	targetURL := fmt.Sprintf("/login?email=%s", url.QueryEscape(email))

	resp, err := h.APIClient.Login(email, password)
	if err != nil {
		logger.Log.Error("during login API call", "error", err)
		redirectWithParams(w, r, targetURL, map[string]string{"error": "Internal error: backend unavailable."})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		redirectWithParams(w, r, targetURL, map[string]string{"error": string(bodyBytes)})
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
