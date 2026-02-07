package middleware

import (
	"net/http"
	"net/url"

	mw "github.com/itchan-dev/itchan/shared/middleware"
)

// Auth wraps shared auth middleware with redirect behavior for frontend
type Auth struct {
	sharedAuth *mw.Auth
}

// NewAuth creates a frontend auth middleware wrapper
func NewAuth(sharedAuth *mw.Auth) *Auth {
	return &Auth{sharedAuth: sharedAuth}
}

// NeedAuth returns middleware with redirect behavior
func (a *Auth) NeedAuth() func(http.Handler) http.Handler {
	return wrapWithRedirect(a.sharedAuth.NeedAuth())
}

// AdminOnly returns admin middleware with redirect behavior
func (a *Auth) AdminOnly() func(http.Handler) http.Handler {
	return wrapWithRedirect(a.sharedAuth.AdminOnly())
}

// OptionalAuth returns middleware that populates user context if available (no redirect needed)
func (a *Auth) OptionalAuth() func(http.Handler) http.Handler {
	return a.sharedAuth.OptionalAuth()
}

// authRedirectWriter intercepts 401/403 errors and redirects to login
type authRedirectWriter struct {
	http.ResponseWriter
	request    *http.Request
	redirected bool // Single flag: true if we've handled a redirect
}

func (w *authRedirectWriter) WriteHeader(statusCode int) {
	if w.redirected {
		return // Already handled
	}

	if statusCode == http.StatusUnauthorized {
		w.redirected = true
		redirectToLogin(w.ResponseWriter, w.request, "Please log in to continue")
		return
	}

	if statusCode == http.StatusForbidden {
		w.redirected = true
		redirectToLogin(w.ResponseWriter, w.request, "Access denied")
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

func redirectToLogin(w http.ResponseWriter, r *http.Request, errorMsg string) {
	u, _ := url.Parse("/login") // /login is always valid
	query := u.Query()
	query.Set("error", errorMsg)
	u.RawQuery = query.Encode()
	http.Redirect(w, r, u.String(), http.StatusSeeOther)
}

// wrapWithRedirect wraps any middleware to intercept auth errors
func wrapWithRedirect(authMiddleware func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wrapper := &authRedirectWriter{
				ResponseWriter: w,
				request:        r,
			}
			authMiddleware(next).ServeHTTP(wrapper, r)
		})
	}
}
