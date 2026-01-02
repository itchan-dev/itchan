package middleware

import (
	"net/http"
)

// SecurityHeadersWithCSP adds security headers with custom Content-Security-Policy
// isHTTPS: if true, adds Strict-Transport-Security header
// csp: Content-Security-Policy value (if empty, no CSP header is set)
func SecurityHeadersWithCSP(isHTTPS bool, csp string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			headers := w.Header()

			// Clickjacking protection
			headers.Set("X-Frame-Options", "DENY")

			// Prevent MIME type sniffing
			headers.Set("X-Content-Type-Options", "nosniff")

			// Legacy XSS protection (older browsers)
			headers.Set("X-XSS-Protection", "1; mode=block")

			// Referrer policy for privacy
			headers.Set("Referrer-Policy", "strict-origin-when-cross-origin")

			// Disable unnecessary browser features
			headers.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")

			// Add CSP if provided
			if csp != "" {
				headers.Set("Content-Security-Policy", csp)
			}

			// HSTS - only when using HTTPS
			if isHTTPS {
				headers.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}

			next.ServeHTTP(w, r)
		})
	}
}
