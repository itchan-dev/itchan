package middleware

import (
	"encoding/base64"
	"net/http"

	mw "github.com/itchan-dev/itchan/shared/middleware"
)

const (
	flashCookieError = "flash_error"
)

// Auth wraps shared auth middleware with redirect behavior for frontend
type Auth struct {
	sharedAuth    *mw.Auth
	secureCookies bool
}

// NewAuth creates a frontend auth middleware wrapper
func NewAuth(sharedAuth *mw.Auth, secureCookies bool) *Auth {
	return &Auth{
		sharedAuth:    sharedAuth,
		secureCookies: secureCookies,
	}
}

// NeedAuth returns middleware with redirect behavior
func (a *Auth) NeedAuth() func(http.Handler) http.Handler {
	return a.wrapWithRedirect(a.sharedAuth.NeedAuth())
}

// AdminOnly returns admin middleware with redirect behavior
func (a *Auth) AdminOnly() func(http.Handler) http.Handler {
	return a.wrapWithRedirect(a.sharedAuth.AdminOnly())
}

// OptionalAuth returns middleware that populates user context if available (no redirect needed)
func (a *Auth) OptionalAuth() func(http.Handler) http.Handler {
	return a.sharedAuth.OptionalAuth()
}

// authRedirectWriter intercepts 401/403 errors and redirects to login
type authRedirectWriter struct {
	http.ResponseWriter
	request       *http.Request
	secureCookies bool
	redirected    bool // Single flag: true if we've handled a redirect
}

func (w *authRedirectWriter) WriteHeader(statusCode int) {
	if w.redirected {
		return // Already handled
	}

	if statusCode == http.StatusUnauthorized {
		w.redirected = true
		redirectToLogin(w.ResponseWriter, w.request, w.secureCookies, "Please log in to continue")
		return
	}

	if statusCode == http.StatusForbidden {
		w.redirected = true
		redirectToLogin(w.ResponseWriter, w.request, w.secureCookies, "Access denied")
		return
	}

	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *authRedirectWriter) Write(data []byte) (int, error) {
	if w.redirected {
		return len(data), nil // Discard body after redirect
	}
	return w.ResponseWriter.Write(data)
}

func redirectToLogin(w http.ResponseWriter, r *http.Request, secureCookies bool, errorMsg string) {
	// Set flash error cookie (base64 encoded for safe storage of special characters)
	encodedMessage := base64.StdEncoding.EncodeToString([]byte(errorMsg))
	cookie := &http.Cookie{
		Name:     flashCookieError,
		Value:    encodedMessage,
		Path:     "/",
		MaxAge:   300, // 5 minutes (enough time for redirect)
		HttpOnly: true,
		Secure:   secureCookies,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// wrapWithRedirect wraps any middleware to intercept auth errors
func (a *Auth) wrapWithRedirect(authMiddleware func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wrapper := &authRedirectWriter{
				ResponseWriter: w,
				request:        r,
				secureCookies:  a.secureCookies,
			}
			authMiddleware(next).ServeHTTP(wrapper, r)
		})
	}
}
