package middleware

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

type BoardAccess interface {
	AllowedDomains(board string) []string
}

// RestrictBoardAccess assumes:
// 1. Email validation/confirmation is done in prior middleware.
// 2. User added to request context in prior middleware
func RestrictBoardAccess(access BoardAccess) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			board, ok := mux.Vars(r)["board"]
			if !ok {
				// if no board in vars - skip
				next.ServeHTTP(w, r)
				return
			}

			user := GetUserFromContext(r)
			if user == nil {
				next.ServeHTTP(w, r)
				return
			}
			if user.Admin {
				next.ServeHTTP(w, r) // Admin bypass
				return
			}

			allowedDomains := access.AllowedDomains(board)
			if allowedDomains == nil {
				next.ServeHTTP(w, r) // No restrictions
				return
			}

			// Fail-safe email format check
			emailDomain, err := user.EmailDomain()
			if err != nil {
				http.Error(w, "Access restricted", http.StatusForbidden)
				return
			}

			// Domain check
			for _, d := range allowedDomains {
				if d == emailDomain {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Log and deny access
			log.Printf("Restricted access: user=%d, board=%s, domain=%s", user.Id, board, emailDomain)
			http.Error(w, "Access restricted", http.StatusForbidden)
		})
	}
}
