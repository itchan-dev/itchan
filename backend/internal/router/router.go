package router

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/itchan-dev/itchan/backend/internal/setup"
	mw "github.com/itchan-dev/itchan/shared/middleware"
	"github.com/itchan-dev/itchan/shared/middleware/metrics"
	rl "github.com/itchan-dev/itchan/shared/middleware/ratelimiter"
)

// New creates and configures a new chi router with all the routes.
// IMPORTANT! ratelimiters set with .Use limit request for all endpoints combined in that subrouter
func New(deps *setup.Dependencies) *chi.Mux {
	r := chi.NewRouter()

	// Strip trailing slashes (replaces mux.StrictSlash)
	r.Use(middleware.StripSlashes)

	// Prometheus metrics middleware (must be early to capture all requests)
	r.Use(metrics.Middleware)

	// Enable gzip compression for all responses
	r.Use(middleware.Compress(5))

	// setup CORS for frontend
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:8081"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Add security headers
	// Backend CSP: strict policy (JSON API only, no scripts/styles needed)
	backendCSP := "default-src 'none'; frame-ancestors 'none'"
	r.Use(mw.SecurityHeadersWithCSP(deps.Config.Public.SecureCookies, backendCSP))

	// Add a wildcard OPTIONS handler to avoid 404s for preflight requests
	r.Options("/*", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h := deps.Handler
	authMw := deps.AuthMiddleware

	// Health check and metrics endpoints (no auth required)
	// Support both GET and HEAD for health checks (wget --spider uses HEAD)
	r.Get("/health", h.Health)
	r.Head("/health", h.Health)
	r.Get("/ready", h.Ready)
	r.Head("/ready", h.Ready)
	r.Handle("/metrics", promhttp.Handler())

	r.Route("/v1", func(v1 chi.Router) {
		// Public config endpoint
		v1.Get("/public_config", h.GetPublicConfig)

		// Admin routes
		v1.Route("/admin", func(admin chi.Router) {
			admin.Use(authMw.AdminOnly())

			admin.Post("/boards", h.CreateBoard)
			admin.Delete("/{board}", h.DeleteBoard)
			admin.Delete("/{board}/{thread}", h.DeleteThread)
			admin.Post("/{board}/{thread}/pin", h.TogglePinnedThread)
			admin.Delete("/{board}/{thread}/{message}", h.DeleteMessage)

			// Admin blacklist routes
			admin.Post("/users/{userId}/blacklist", h.BlacklistUser)
			admin.Delete("/users/{userId}/blacklist", h.UnblacklistUser)
			admin.Post("/blacklist/refresh", h.RefreshBlacklistCache)
			admin.Get("/blacklist", h.GetBlacklistedUsers)
		})

		// Auth routes
		v1.Route("/auth", func(auth chi.Router) {
			// Rate-limited email sending endpoints
			auth.Group(func(authSendingEmail chi.Router) {
				authSendingEmail.Use(mw.RateLimit(rl.New(1.0/10, 1, 1*time.Hour), mw.GetEmailFromBody)) // 10 per sec by email
				authSendingEmail.Use(mw.RateLimit(rl.New(1.0/10.0, 1, 1*time.Hour), mw.GetIP))          // 10 per sec by IP
				authSendingEmail.Use(mw.GlobalRateLimit(rl.Rps100()))                                   // 100 global RPS
				authSendingEmail.Post("/register", h.Register)
			})

			// Confirmation code verification (stricter limits to prevent brute force)
			auth.Group(func(authConfirmation chi.Router) {
				authConfirmation.Use(mw.RateLimit(rl.New(5.0/600.0, 5, 1*time.Hour), mw.GetEmailFromBody)) // 5 attempts per 10 minutes by email
				authConfirmation.Use(mw.RateLimit(rl.New(1, 1, 1*time.Hour), mw.GetIP))                    // 1 per second by IP (backup)
				authConfirmation.Use(mw.GlobalRateLimit(rl.Rps100()))                                      // 100 global RPS
				authConfirmation.Post("/check_confirmation_code", h.CheckConfirmationCode)
			})

			// Login endpoint (separate rate limiting)
			auth.Group(func(authLogin chi.Router) {
				authLogin.Use(mw.RateLimit(rl.OnceInSecond(), mw.GetIP)) // 1 per second by IP
				authLogin.Use(mw.GlobalRateLimit(rl.Rps1000()))          // 1000 global RPS
				authLogin.Post("/login", h.Login)
			})

			// Invite-based registration (public, rate limited)
			auth.Group(func(authRegisterInvite chi.Router) {
				authRegisterInvite.Use(mw.RateLimit(rl.OnceInSecond(), mw.GetIP)) // 1 per second by IP
				authRegisterInvite.Use(mw.GlobalRateLimit(rl.Rps100()))           // 100 global RPS
				authRegisterInvite.Post("/register_with_invite", h.RegisterWithInvite)
			})

			// Logout (no rate limits)
			auth.Post("/logout", h.Logout)
		})

		// Logged-in user routes
		v1.Group(func(loggedIn chi.Router) {
			loggedIn.Use(authMw.NeedAuth())                                  // Enforce JWT authentication with blacklist check
			loggedIn.Use(mw.RateLimit(rl.Rps100(), mw.GetUserIDFromContext)) // 100 RPS per user

			// User activity endpoint
			loggedIn.Get("/users/me/activity", h.GetUserActivity)

			// Invite management routes (authenticated users only)
			loggedIn.Route("/invites", func(invites chi.Router) {
				invites.Get("/", h.GetMyInvites)
				// Generate invite: 1 per minute per user to prevent spam
				invites.With(mw.RateLimit(rl.OnceInMinute(), mw.GetUserIDFromContext)).Post("/", h.GenerateInvite)
				invites.Delete("/{codeHash}", h.RevokeInvite)
			})

			loggedIn.Group(func(boards chi.Router) {
				boards.Use(mw.RestrictBoardAccess(deps.AccessData)) // Restrict access based on board and email domain

				boards.Get("/boards", h.GetBoards)
				// GetBoard: 10 RPS per user
				boards.With(mw.RateLimit(rl.Rps10(), mw.GetUserIDFromContext)).Get("/{board}", h.GetBoard)
				// CreateThread: 1 per minute per user
				boards.With(mw.RateLimit(rl.OnceInMinute(), mw.GetUserIDFromContext)).Post("/{board}", h.CreateThread)

				boards.Get("/{board}/{thread}", h.GetThread)
				// CreateMessage: 1 per second per user (fixed rate limiter)
				boards.With(mw.RateLimit(rl.New(1, 1, 1*time.Hour), mw.GetUserIDFromContext)).Post("/{board}/{thread}", h.CreateMessage)
				boards.Get("/{board}/{thread}/{message}", h.GetMessage)
			})
		})
	})

	return r
}
