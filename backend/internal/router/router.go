package router

import (
	"net/http"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	"github.com/itchan-dev/itchan/backend/internal/setup"
	mw "github.com/itchan-dev/itchan/shared/middleware"
	rl "github.com/itchan-dev/itchan/shared/middleware/ratelimiter"
)

// New creates and configures a new mux router with all the routes.
// IMPORTANT! ratelimiters set with .Use limit request for all endpoints combined in that subrouter
func New(deps *setup.Dependencies) *mux.Router {
	r := mux.NewRouter()

	// Enable gzip compression for all responses
	r.Use(handlers.CompressHandler)

	// setup CORS for frontend
	r.Use(handlers.CORS(
		handlers.AllowedOrigins([]string{"http://localhost:8081"}),
		handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
		handlers.AllowedHeaders([]string{"Content-Type", "Authorization"}),
	))

	// Add security headers
	// Backend CSP: strict policy (JSON API only, no scripts/styles needed)
	backendCSP := "default-src 'none'; frame-ancestors 'none'"
	r.Use(mw.SecurityHeadersWithCSP(deps.Config.Public.SecureCookies, backendCSP))

	// Add a wildcard OPTIONS handler to avoid 404s for preflight requests
	r.Methods("OPTIONS").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h := deps.Handler
	authMw := deps.AuthMiddleware

	v1 := r.PathPrefix("/v1").Subrouter()
	// Public config endpoint
	v1.HandleFunc("/public_config", h.GetPublicConfig).Methods("GET")

	// Admin routes
	admin := v1.PathPrefix("/admin").Subrouter()
	admin.Use(authMw.AdminOnly())
	admin.HandleFunc("/boards", h.CreateBoard).Methods("POST")
	admin.HandleFunc("/{board}", h.DeleteBoard).Methods("DELETE")
	admin.HandleFunc("/{board}/{thread}", h.DeleteThread).Methods("DELETE")
	admin.HandleFunc("/{board}/{thread}/{message}", h.DeleteMessage).Methods("DELETE")

	// Admin blacklist routes
	admin.HandleFunc("/users/{userId}/blacklist", h.BlacklistUser).Methods("POST")
	admin.HandleFunc("/users/{userId}/blacklist", h.UnblacklistUser).Methods("DELETE")
	admin.HandleFunc("/blacklist/refresh", h.RefreshBlacklistCache).Methods("POST")
	admin.HandleFunc("/blacklist", h.GetBlacklistedUsers).Methods("GET")

	// Auth routes
	auth := v1.PathPrefix("/auth").Subrouter()
	// Rate-limited email sending endpoints
	authSendingEmail := auth.NewRoute().Subrouter()
	authSendingEmail.Use(mw.RateLimit(rl.New(1.0/10, 1, 1*time.Hour), mw.GetEmailFromBody)) // 10 per sec by email
	authSendingEmail.Use(mw.RateLimit(rl.New(1.0/10.0, 1, 1*time.Hour), mw.GetIP))          // 10 per sec by IP
	authSendingEmail.Use(mw.GlobalRateLimit(rl.Rps100()))                                   // 100 global RPS
	authSendingEmail.HandleFunc("/register", h.Register).Methods("POST")

	// Confirmation code verification (stricter limits to prevent brute force)
	authConfirmation := auth.NewRoute().Subrouter()
	authConfirmation.Use(mw.RateLimit(rl.New(5.0/600.0, 5, 1*time.Hour), mw.GetEmailFromBody)) // 5 attempts per 10 minutes by email
	authConfirmation.Use(mw.RateLimit(rl.New(1, 1, 1*time.Hour), mw.GetIP))                    // 1 per second by IP (backup)
	authConfirmation.Use(mw.GlobalRateLimit(rl.Rps100()))                                      // 100 global RPS
	authConfirmation.HandleFunc("/check_confirmation_code", h.CheckConfirmationCode).Methods("POST")

	// Login endpoint (separate rate limiting)
	authLogin := auth.NewRoute().Subrouter()
	authLogin.Use(mw.RateLimit(rl.OnceInSecond(), mw.GetIP)) // 1 per second by IP
	authLogin.Use(mw.GlobalRateLimit(rl.Rps1000()))          // 1000 global RPS
	authLogin.HandleFunc("/login", h.Login).Methods("POST")

	// Logout (no rate limits)
	auth.HandleFunc("/logout", h.Logout).Methods("POST")

	// Logged-in user routes
	loggedIn := v1.NewRoute().Subrouter()
	loggedIn.Use(authMw.NeedAuth())                                 // Enforce JWT authentication with blacklist check
	loggedIn.Use(mw.RestrictBoardAccess(deps.AccessData))           // Restrict access based on board and email domain
	loggedIn.Use(mw.RateLimit(rl.Rps100(), mw.GetEmailFromContext)) // 100 RPS per user

	loggedIn.HandleFunc("/boards", h.GetBoards).Methods("GET")
	// GetBoard: 10 RPS per user
	loggedIn.Handle("/{board}", mw.RateLimit(rl.Rps10(), mw.GetEmailFromContext)(http.HandlerFunc(h.GetBoard))).Methods("GET")
	// CreateThread: 1 per minute per user
	loggedIn.Handle("/{board}", mw.RateLimit(rl.OnceInMinute(), mw.GetEmailFromContext)(http.HandlerFunc(h.CreateThread))).Methods("POST")

	loggedIn.HandleFunc("/{board}/{thread}", h.GetThread).Methods("GET")
	// CreateMessage: 1 per second per user (fixed rate limiter)
	loggedIn.Handle("/{board}/{thread}", mw.RateLimit(rl.New(1, 1, 1*time.Hour), mw.GetEmailFromContext)(http.HandlerFunc(h.CreateMessage))).Methods("POST")

	loggedIn.HandleFunc("/{board}/{thread}/{message}", h.GetMessage).Methods("GET")

	return r
}
