package router

import (
	"net/http"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/itchan-dev/itchan/frontend/internal/handler"
	frontend_mw "github.com/itchan-dev/itchan/frontend/internal/middleware"
	"github.com/itchan-dev/itchan/frontend/internal/setup"
	mw "github.com/itchan-dev/itchan/shared/middleware"
	rl "github.com/itchan-dev/itchan/shared/middleware/ratelimiter"
)

func SetupRouter(deps *setup.Dependencies) *mux.Router {
	r := mux.NewRouter()
	r.StrictSlash(true)

	// Enable gzip compression for all responses (HTML, CSS, JS)
	r.Use(handlers.CompressHandler)

	// Add security headers
	frontendCSP := "default-src 'self'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data: blob:; " +
		"frame-ancestors 'none'; " +
		"base-uri 'self'; " +
		"form-action 'self'"
	r.Use(mw.SecurityHeadersWithCSP(deps.Public.SecureCookies, frontendCSP))

	// CSRF token generation for all routes (if enabled)
	if deps.Public.CSRFEnabled {
		r.Use(frontend_mw.GenerateCSRFToken(frontend_mw.CSRFConfig{
			SecureCookies: deps.Public.SecureCookies,
		}))
	}

	// Public routes (GET endpoints - no rate limiting needed)
	r.HandleFunc("/favicon.ico", handler.FaviconHandler)
	r.HandleFunc("/login", deps.Handler.LoginGetHandler).Methods("GET")
	r.HandleFunc("/register", deps.Handler.RegisterGetHandler).Methods("GET")
	r.HandleFunc("/register_invite", deps.Handler.RegisterInviteGetHandler).Methods("GET")
	r.HandleFunc("/check_confirmation_code", deps.Handler.ConfirmEmailGetHandler).Methods("GET")

	// Public POST routes (rate limited to prevent abuse)
	publicPosts := r.NewRoute().Subrouter()
	publicPosts.Use(mw.GlobalRateLimit(rl.Rps100()))                                          // 100 global RPS
	publicPosts.Use(mw.RateLimit(rl.New(10.0/60.0, 10, 1*time.Hour), mw.GetIP))               // 10 per minute by IP (backup)
	publicPostsEmail := publicPosts.NewRoute().Subrouter()                                    // Using email field
	publicPostsEmail.Use(mw.RateLimit(rl.New(5.0/60.0, 5, 1*time.Hour), mw.GetEmailFromForm)) // 5 attempts per minute by email
	publicPostsEmail.HandleFunc("/login", deps.Handler.LoginPostHandler).Methods("POST")
	publicPostsEmail.HandleFunc("/register", deps.Handler.RegisterPostHandler).Methods("POST")
	publicPostsEmail.HandleFunc("/check_confirmation_code", deps.Handler.ConfirmEmailPostHandler).Methods("POST")
	publicPostsInvite := publicPosts.NewRoute().Subrouter()                                                   // Using invite_code field
	publicPostsInvite.Use(mw.RateLimit(rl.New(5.0/60.0, 5, 1*time.Hour), mw.GetFieldFromForm("invite_code"))) // 5 attempts per minute by each invite code
	publicPostsInvite.HandleFunc("/register_invite", deps.Handler.RegisterInvitePostHandler).Methods("POST")

	r.PathPrefix("/static/").Handler(
		http.StripPrefix("/static/", http.FileServer(http.Dir("static"))),
	)

	// Create frontend auth middleware wrapper
	authMw := frontend_mw.NewAuth(deps.AuthMiddleware)

	// Admin-only routes (register before generic path patterns to avoid conflicts)
	adminRouter := r.NewRoute().Subrouter()
	adminRouter.Use(authMw.AdminOnly())

	// Add CSRF validation for admin operations
	if deps.Public.CSRFEnabled {
		adminRouter.Use(frontend_mw.ValidateCSRFToken())
	}

	adminRouter.HandleFunc("/blacklist/user", deps.Handler.BlacklistUserHandler).Methods("POST")
	adminRouter.HandleFunc("/{board}/delete", deps.Handler.BoardDeleteHandler).Methods("POST")
	adminRouter.HandleFunc("/{board}/{thread}/delete", deps.Handler.ThreadDeleteHandler).Methods("POST")
	adminRouter.HandleFunc("/{board}/{thread}/sticky", deps.Handler.ThreadToggleStickyHandler).Methods("POST")
	adminRouter.HandleFunc("/{board}/{thread}/{message}/delete", deps.Handler.MessageDeleteHandler).Methods("POST")

	// Authenticated routes
	authRouter := r.NewRoute().Subrouter()
	authRouter.Use(authMw.NeedAuth())
	authRouter.Use(mw.RestrictBoardAccess(deps.AccessData))           // Enforce board access restrictions
	authRouter.Use(mw.RateLimit(rl.Rps100(), mw.GetEmailFromContext)) // 100 RPS per user

	// Add CSRF validation for authenticated state-changing operations
	if deps.Public.CSRFEnabled {
		authRouter.Use(frontend_mw.ValidateCSRFToken())
	}

	// Media file server - serve files from shared media directory
	mediaPath := deps.Handler.MediaPath
	authRouter.PathPrefix("/media/{board}/").Handler(
		http.StripPrefix("/media/", http.FileServer(http.Dir(mediaPath))),
	)

	authRouter.HandleFunc("/", deps.Handler.IndexGetHandler).Methods("GET")
	authRouter.HandleFunc("/", deps.Handler.IndexPostHandler).Methods("POST")
	authRouter.HandleFunc("/logout", deps.Handler.LogoutHandler)

	// Invite routes (must be registered before /{board} to avoid route conflicts)
	authRouter.HandleFunc("/invites", deps.Handler.InvitesGetHandler).Methods("GET")
	// GenerateInvite: 1 per minute per user (same as CreateThread)
	authRouter.Handle("/invites/generate", mw.RateLimit(rl.OnceInMinute(), mw.GetEmailFromContext)(http.HandlerFunc(deps.Handler.GenerateInvitePostHandler))).Methods("POST")
	authRouter.HandleFunc("/invites/revoke", deps.Handler.RevokeInvitePostHandler).Methods("POST")

	// Account page
	authRouter.HandleFunc("/account", deps.Handler.AccountGetHandler).Methods("GET")

	// Board routes with specific rate limits
	// GetBoard: 10 RPS per user
	authRouter.Handle("/{board}", mw.RateLimit(rl.Rps10(), mw.GetEmailFromContext)(http.HandlerFunc(deps.Handler.BoardGetHandler))).Methods("GET")
	// CreateThread: 1 per minute per user
	authRouter.Handle("/{board}", mw.RateLimit(rl.OnceInMinute(), mw.GetEmailFromContext)(http.HandlerFunc(deps.Handler.BoardPostHandler))).Methods("POST")

	// Thread routes
	authRouter.HandleFunc("/{board}/{thread}", deps.Handler.ThreadGetHandler).Methods("GET")
	// CreateMessage: 1 per second per user
	authRouter.Handle("/{board}/{thread}", mw.RateLimit(rl.New(1, 1, 1*time.Hour), mw.GetEmailFromContext)(http.HandlerFunc(deps.Handler.ThreadPostHandler))).Methods("POST")

	// API proxy for message preview (JSON)
	authRouter.HandleFunc("/api/v1/{board}/{thread}/{message}", deps.Handler.MessagePreviewHandler).Methods("GET")
	// API endpoint for message preview (HTML)
	authRouter.HandleFunc("/api/v1/{board}/{thread}/{message}/html", deps.Handler.MessagePreviewHTMLHandler).Methods("GET")

	return r
}
