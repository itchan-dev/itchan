package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/itchan-dev/itchan/shared/domain"
	mw "github.com/itchan-dev/itchan/shared/middleware"
)

func (h *Handler) RegisterGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		Error template.HTML
		User  *domain.User // Might be nil if not logged in
	}
	templateData.User = mw.GetUserFromContext(r)
	templateData.Error, _ = parseMessagesFromQuery(r) // Get error from query param
	h.renderTemplate(w, "register.html", templateData)
}

func (h *Handler) RegisterPostHandler(w http.ResponseWriter, r *http.Request) {
	targetURL := "/register" // Redirect target on error
	successURL := "/check_confirmation_code"

	// Parse form data
	email := r.FormValue("email")
	password := r.FormValue("password")
	creds := credentials{Email: email, Password: password}

	// Prepare backend request data
	credsJson, err := json.Marshal(creds)
	if err != nil {
		log.Printf("Error marshalling registration data: %v", err)
		redirectWithError(w, r, targetURL, "Internal error: could not prepare data.")
		return
	}

	// Send request to backend
	resp, err := http.Post("http://api:8080/v1/auth/register", "application/json", bytes.NewBuffer(credsJson))
	if err != nil {
		log.Printf("Error contacting backend API for registration: %v", err)
		redirectWithError(w, r, targetURL, "Internal error: backend unavailable.")
		return
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusTooEarly {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("Registration failed (status %d)", resp.StatusCode) // Default message
		if readErr == nil && len(bodyBytes) > 0 {
			errMsg = fmt.Sprintf("Error: %s", string(bodyBytes))
		} else if readErr != nil {
			log.Printf("Error reading backend error response body: %v", readErr)
		}
		redirectWithError(w, r, targetURL, errMsg)
		return
	}

	// Handle specific case: confirmation needed
	if resp.StatusCode == http.StatusTooEarly {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		errMsg := "Confirmation required." // Default message
		if readErr == nil && len(bodyBytes) > 0 {
			// Include the backend message and a link hint in the error for the redirect
			errMsg = fmt.Sprintf(`%s Please check your email or use the confirmation page. <a href="/check_confirmation_code?email=%s">Go to Confirmation</a>`, string(bodyBytes), url.QueryEscape(email))
		} else if readErr != nil {
			log.Printf("Error reading backend StatusTooEarly response body: %v", readErr)
		}
		// Redirect back to register page with this specific error message
		redirectWithError(w, r, targetURL, errMsg)
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
	}
	templateData.User = mw.GetUserFromContext(r)
	templateData.EmailPlaceholder = parseEmail(r)                        // Get email from query param
	templateData.Error, templateData.Success = parseMessagesFromQuery(r) // Get messages

	// Customize success message if needed based on query param
	if r.URL.Query().Get("success") == "confirmed" {
		templateData.Success = template.HTML(`Success! You can now <a href="/login">login</a>.`)
	}

	h.renderTemplate(w, "check_confirmation_code.html", templateData)
}

func (h *Handler) ConfirmEmailPostHandler(w http.ResponseWriter, r *http.Request) {
	// Parse form data
	email := r.FormValue("email")
	code := r.FormValue("confirmation_code")

	// Redirect target on error (back to the confirmation page with email)
	targetURL := fmt.Sprintf("/check_confirmation_code?email=%s", url.QueryEscape(email))

	// Prepare backend request data
	backendData := struct {
		Email            string `json:"email"`
		ConfirmationCode string `json:"confirmation_code"`
	}{Email: email, ConfirmationCode: code}

	backendDataJson, err := json.Marshal(backendData)
	if err != nil {
		log.Printf("Error marshalling confirmation data: %v", err)
		redirectWithError(w, r, targetURL, "Internal error: could not prepare data.")
		return
	}

	// Send request to backend
	resp, err := http.Post("http://api:8080/v1/auth/check_confirmation_code", "application/json", bytes.NewBuffer(backendDataJson))
	if err != nil {
		log.Printf("Error contacting backend API for confirmation: %v", err)
		redirectWithError(w, r, targetURL, "Internal error: backend unavailable.")
		return
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("Confirmation failed (status %d)", resp.StatusCode) // Default message
		if readErr == nil && len(bodyBytes) > 0 {
			errMsg = string(bodyBytes)
		} else if readErr != nil {
			log.Printf("Error reading backend error response body: %v", readErr)
		}
		redirectWithError(w, r, targetURL, errMsg)
		return
	}

	// Success: Redirect back to confirmation page with a success flag/message
	// Using a simple flag like "confirmed"
	redirectWithSuccess(w, r, targetURL, "confirmed")
	// Or redirect to login page directly:
	// http.Redirect(w, r, "/login?success=confirmed", http.StatusSeeOther)
}

func (h *Handler) LoginGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		Error            template.HTML
		User             *domain.User
		EmailPlaceholder string
	}
	templateData.User = mw.GetUserFromContext(r)
	templateData.Error, _ = parseMessagesFromQuery(r)          // Get error message
	templateData.EmailPlaceholder = r.URL.Query().Get("email") // Pre-fill email if redirected with it
	if templateData.EmailPlaceholder == "" {
		// Fallback if not passed in query
		templateData.EmailPlaceholder = parseEmail(r)
	}

	h.renderTemplate(w, "login.html", templateData)
}

func (h *Handler) LoginPostHandler(w http.ResponseWriter, r *http.Request) {
	// Parse form data
	email := r.FormValue("email")
	password := r.FormValue("password")

	// Redirect target on error (back to login page, preserving email)
	targetURL := fmt.Sprintf("/login?email=%s", url.QueryEscape(email))
	successURL := "/" // Redirect target on success

	// Prepare backend request data
	backendData := struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}{Email: email, Password: password}

	backendDataJson, err := json.Marshal(backendData)
	if err != nil {
		log.Printf("Error marshalling login data: %v", err)
		redirectWithError(w, r, targetURL, "Internal error: could not prepare data.")
		return
	}

	// Send request to backend
	resp, err := http.Post("http://api:8080/v1/auth/login", "application/json", bytes.NewBuffer(backendDataJson))
	if err != nil {
		log.Printf("Error contacting backend API for login: %v", err)
		redirectWithError(w, r, targetURL, "Internal error: backend unavailable.")
		return
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("Login failed (status %d)", resp.StatusCode) // Default message
		if readErr == nil && len(bodyBytes) > 0 {
			errMsg = string(bodyBytes)
		} else if readErr != nil {
			log.Printf("Error reading backend error response body: %v", readErr)
		}
		redirectWithError(w, r, targetURL, errMsg)
		return
	}

	// Success: Forward cookies from backend response
	for _, cookie := range resp.Cookies() {
		// Make sure critical cookies like session tokens are HttpOnly
		// The backend should set HttpOnly=true, this just forwards it.
		http.SetCookie(w, cookie)
	}

	// Redirect to the success URL (index page)
	http.Redirect(w, r, successURL, http.StatusSeeOther)
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	// Clear the access token cookie
	cookie := &http.Cookie{
		Path:     "/",
		Name:     "accessToken",
		Value:    "",
		MaxAge:   -1, // Expire immediately
		HttpOnly: true,
		// Secure: true, // Add this if using HTTPS
		// SameSite: http.SameSiteLaxMode, // Or Strict
	}
	http.SetCookie(w, cookie)

	// Redirect to login page
	http.Redirect(w, r, "/login", http.StatusSeeOther) // Use SeeOther after logout action
}
