package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/itchan-dev/itchan/shared/middleware/ratelimiter"

	"github.com/itchan-dev/itchan/shared/utils"
)

func RateLimit(rl *ratelimiter.UserRateLimiter, getIdentity func(r *http.Request) (string, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if user := GetUserFromContext(r); user != nil && user.Admin { // disable for admin
				next.ServeHTTP(w, r)
				return
			}

			identity, err := getIdentity(r)
			if err != nil {
				utils.WriteErrorAndStatusCode(w, err)
				return
			}
			if !rl.Allow(identity) {
				http.Error(w, "Rate limit exceeded, try again later", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func GlobalRateLimit(rl *ratelimiter.UserRateLimiter) func(http.Handler) http.Handler {
	return RateLimit(rl, func(r *http.Request) (string, error) { return "global", nil })
}

// Possible if user was authorized with previous middleware
func GetUserIDFromContext(r *http.Request) (string, error) {
	user := GetUserFromContext(r)
	if user == nil {
		return "", errors.New("Can't get user id")
	}
	// Email no longer stored - use user ID for rate limiting
	return fmt.Sprintf("user_%d", user.Id), nil
}

// GetIP extracts the real client IP from RemoteAddr
// Does NOT trust X-Real-IP or X-Forwarded-For headers (no reverse proxy)
func GetIP(r *http.Request) (string, error) {
	// Only trust RemoteAddr - can't be spoofed (comes from TCP connection)
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// Fallback: if RemoteAddr doesn't have port, use it directly
		ip = r.RemoteAddr
	}

	// Validate it's a real IP
	if net.ParseIP(ip) == nil {
		return "", fmt.Errorf("invalid IP address: %s", ip)
	}

	return ip, nil
}

// GetEmailFromBody extracts email from JSON request body for rate limiting purposes
// It reads the body and restores it so the handler can read it again
// Used by backend API endpoints
func GetEmailFromBody(r *http.Request) (string, error) {
	// Read the body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return "", errors.New("failed to read request body")
	}
	// Restore the body so the handler can read it
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	// Parse JSON to extract email
	var data struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", errors.New("invalid request body")
	}

	if data.Email == "" {
		return "", errors.New("email field is required")
	}

	return data.Email, nil
}

// GetFieldFromForm extracts email from form data for rate limiting purposes
// Used by frontend HTML form submissions
func GetFieldFromForm(field string) func(r *http.Request) (string, error) {
	return func(r *http.Request) (string, error) {
		// Parse form if not already parsed
		if err := r.ParseForm(); err != nil {
			return "", errors.New("failed to parse form")
		}

		email := r.FormValue(field)
		if email == "" {
			return "", fmt.Errorf("%s field is required", field)
		}

		return email, nil
	}
}

// GetEmailFromForm extracts email from form data for rate limiting purposes
// Used by frontend HTML form submissions
func GetEmailFromForm(r *http.Request) (string, error) {
	return GetFieldFromForm("email")(r)
}
