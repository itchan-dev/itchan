package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/itchan-dev/itchan/shared/csrf"
	"github.com/itchan-dev/itchan/shared/logger"
)

const (
	csrfCookieName = "csrf_token"
	csrfFormField  = "csrf_token"
)

type csrfContextKey string

const csrfTokenContextKey csrfContextKey = "csrf_token"

// CSRFConfig holds CSRF middleware configuration
type CSRFConfig struct {
	SecureCookies bool // Use Secure flag on cookies (requires HTTPS)
}

// GenerateCSRFToken middleware generates and sets CSRF token cookie
func GenerateCSRFToken(config CSRFConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if token already exists in cookie
			cookie, err := r.Cookie(csrfCookieName)
			var token string

			if err != nil || cookie.Value == "" {
				// Generate new token
				token, err = csrf.GenerateToken()
				if err != nil {
					logger.Log.Error("failed to generate CSRF token", "error", err)
					http.Error(w, "Internal server error", http.StatusInternalServerError)
					return
				}

				// Set CSRF token cookie
				http.SetCookie(w, &http.Cookie{
					Name:     csrfCookieName,
					Value:    token,
					Path:     "/",
					HttpOnly: true,
					Secure:   config.SecureCookies,
					SameSite: http.SameSiteLaxMode,
					MaxAge:   86400, // 24 hours
				})
			} else {
				token = cookie.Value
			}

			// Store token in context for template rendering
			ctx := context.WithValue(r.Context(), csrfTokenContextKey, token)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ValidateCSRFToken middleware validates CSRF token from form submission
func ValidateCSRFToken() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only validate POST, PUT, PATCH, DELETE methods
			if r.Method != http.MethodPost && r.Method != http.MethodPut &&
				r.Method != http.MethodPatch && r.Method != http.MethodDelete {
				next.ServeHTTP(w, r)
				return
			}

			// Get token from cookie
			cookie, err := r.Cookie(csrfCookieName)
			if err != nil {
				logger.Log.Warn("CSRF token cookie missing", "path", r.URL.Path)
				http.Error(w, "CSRF token missing", http.StatusForbidden)
				return
			}

			// Get token from form
			// Check Content-Type to determine parsing method
			contentType := r.Header.Get("Content-Type")
			if strings.HasPrefix(contentType, "multipart/form-data") {
				// Multipart form (file uploads) - must use ParseMultipartForm
				if err := r.ParseMultipartForm(32 << 20); err != nil {
					logger.Log.Error("failed to parse multipart form", "error", err)
					http.Error(w, "Invalid form data", http.StatusBadRequest)
					return
				}
			} else if r.Form == nil {
				// URL-encoded form
				if err := r.ParseForm(); err != nil {
					logger.Log.Error("failed to parse form", "error", err)
					http.Error(w, "Invalid form data", http.StatusBadRequest)
					return
				}
			}

			formToken := r.FormValue(csrfFormField)

			// Validate tokens match
			if !csrf.ValidateToken(cookie.Value, formToken) {
				logger.Log.Warn("CSRF token validation failed", "path", r.URL.Path)
				http.Error(w, "CSRF token invalid", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetCSRFTokenFromContext retrieves CSRF token from request context
func GetCSRFTokenFromContext(r *http.Request) string {
	token, _ := r.Context().Value(csrfTokenContextKey).(string)
	return token
}
