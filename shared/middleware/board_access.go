package middleware

import (
	"net/http"
	"slices"

	"github.com/gorilla/mux"
	"github.com/itchan-dev/itchan/shared/logger"
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
			if slices.Contains(allowedDomains, emailDomain) {
				next.ServeHTTP(w, r)
				return
			}

			// Log and deny access
			logger.Log.Warn("board access restricted",
				"user_id", user.Id,
				"board", board,
				"domain", emailDomain)
			http.Error(w, "Access restricted", http.StatusForbidden)
		})
	}
}
