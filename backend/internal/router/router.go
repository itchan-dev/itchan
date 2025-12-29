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

	// Add a wildcard OPTIONS handler to avoid 404s for preflight requests
	r.Methods("OPTIONS").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h := deps.Handler
	jwt := deps.Jwt
	blacklistCache := deps.BlacklistCache
	secureCookies := deps.Config.Public.SecureCookies

	v1 := r.PathPrefix("/v1").Subrouter()
	// Public config endpoint
	v1.HandleFunc("/public_config", h.GetPublicConfig).Methods("GET")

	// Admin routes
	admin := v1.PathPrefix("/admin").Subrouter()
	admin.Use(mw.AdminOnly(jwt, blacklistCache, secureCookies))
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
	// authSendingEmail.Use(mw.RateLimit(rl.New(1.0/10, 1, 1*time.Hour), mw.GetEmailFromBody)) // per 10 sec by email
	authSendingEmail.Use(mw.RateLimit(rl.New(1.0/10.0, 1, 1*time.Hour), mw.GetIP)) // per 10 sec by IP
	authSendingEmail.Use(mw.GlobalRateLimit(rl.Rps100()))                          // 100 global RPS
	authSendingEmail.HandleFunc("/register", h.Register).Methods("POST")

	// Database lookup endpoints (higher limits)
	authDbLookup := auth.NewRoute().Subrouter()
	// authDbLookup.Use(mw.RateLimit(rl.OnceInSecond(), mw.GetEmailFromBody)) // 1 per second by email
	authDbLookup.Use(mw.RateLimit(rl.OnceInSecond(), mw.GetIP)) // 1 per second by IP
	authDbLookup.Use(mw.GlobalRateLimit(rl.Rps1000()))          // 1000 global RPS
	authDbLookup.HandleFunc("/check_confirmation_code", h.CheckConfirmationCode).Methods("POST")
	authDbLookup.HandleFunc("/login", h.Login).Methods("POST")

	// Logout (no rate limits)
	auth.HandleFunc("/logout", h.Logout).Methods("POST")

	// Logged-in user routes
	loggedIn := v1.NewRoute().Subrouter()
	loggedIn.Use(mw.NeedAuth(jwt, blacklistCache, secureCookies))   // Enforce JWT authentication with blacklist check
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
