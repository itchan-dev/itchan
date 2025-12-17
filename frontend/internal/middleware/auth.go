package middleware

import (
	"net/http"
	"net/url"

	jwt_internal "github.com/itchan-dev/itchan/shared/jwt"
	mw "github.com/itchan-dev/itchan/shared/middleware"
)

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

	// Intercept auth errors and redirect
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

	// Pass through all other status codes
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

// NeedAuth wraps shared auth middleware with redirect behavior
func NeedAuth(jwtService jwt_internal.JwtService) func(http.Handler) http.Handler {
	return wrapWithRedirect(mw.NeedAuth(jwtService))
}

// AdminOnly wraps shared admin auth middleware with redirect behavior
func AdminOnly(jwtService jwt_internal.JwtService) func(http.Handler) http.Handler {
	return wrapWithRedirect(mw.AdminOnly(jwtService))
}
