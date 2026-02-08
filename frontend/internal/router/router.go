package router

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/itchan-dev/itchan/frontend/internal/handler"
	frontend_mw "github.com/itchan-dev/itchan/frontend/internal/middleware"
	"github.com/itchan-dev/itchan/frontend/internal/setup"
	mw "github.com/itchan-dev/itchan/shared/middleware"
	rl "github.com/itchan-dev/itchan/shared/middleware/ratelimiter"
)

func SetupRouter(deps *setup.Dependencies) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.StripSlashes)

	r.Use(middleware.Compress(5))

	frontendCSP := "default-src 'self'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data: blob:; " +
		"frame-ancestors 'none'; " +
		"base-uri 'self'; " +
		"form-action 'self'"
	r.Use(mw.SecurityHeadersWithCSP(deps.Public.SecureCookies, frontendCSP))

	if deps.Public.CSRFEnabled {
		r.Use(frontend_mw.GenerateCSRFToken(frontend_mw.CSRFConfig{
			SecureCookies: deps.Public.SecureCookies,
		}))
	}

	// Health check endpoint (no auth required)
	// Support both GET and HEAD for health checks (wget --spider uses HEAD)
	healthHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}
	r.Get("/health", healthHandler)
	r.Head("/health", healthHandler)

	// Public routes (GET endpoints - no rate limiting needed)
	r.Get("/favicon.ico", handler.FaviconHandler)
	r.Get("/login", deps.Handler.LoginGetHandler)
	r.Get("/register", deps.Handler.RegisterGetHandler)
	r.Get("/register_invite", deps.Handler.RegisterInviteGetHandler)
	r.Get("/check_confirmation_code", deps.Handler.ConfirmEmailGetHandler)

	// Create frontend auth middleware wrapper (needed for optional auth routes below)
	authMw := frontend_mw.NewAuth(deps.AuthMiddleware)

	// Public routes with optional auth (shows user info if logged in)
	r.Group(func(optionalAuthRouter chi.Router) {
		optionalAuthRouter.Use(authMw.OptionalAuth())
		optionalAuthRouter.Get("/faq", deps.Handler.FAQGetHandler)
		optionalAuthRouter.Get("/about", deps.Handler.AboutGetHandler)
		optionalAuthRouter.Get("/terms", deps.Handler.TermsGetHandler)
		optionalAuthRouter.Get("/privacy", deps.Handler.PrivacyGetHandler)
		optionalAuthRouter.Get("/contacts", deps.Handler.ContactsGetHandler)
	})

	// Public POST routes (rate limited to prevent abuse)
	r.Group(func(publicPosts chi.Router) {
		publicPosts.Use(mw.GlobalRateLimit(rl.Rps100()))
		publicPosts.Use(mw.RateLimit(rl.New(10.0/60.0, 10, 1*time.Hour), mw.GetIP)) // 10 per minute by IP (backup)

		// Using email field
		publicPosts.Group(func(publicPostsEmail chi.Router) {
			publicPostsEmail.Use(mw.RateLimit(rl.New(5.0/60.0, 5, 1*time.Hour), mw.GetEmailFromForm)) // 5 attempts per minute by email
			publicPostsEmail.Post("/login", deps.Handler.LoginPostHandler)
			publicPostsEmail.Post("/register", deps.Handler.RegisterPostHandler)
			publicPostsEmail.Post("/check_confirmation_code", deps.Handler.ConfirmEmailPostHandler)
		})

		// Using invite_code field
		publicPosts.Group(func(publicPostsInvite chi.Router) {
			publicPostsInvite.Use(mw.RateLimit(rl.New(5.0/60.0, 5, 1*time.Hour), mw.GetFieldFromForm("invite_code"))) // 5 attempts per minute by each invite code
			publicPostsInvite.Post("/register_invite", deps.Handler.RegisterInvitePostHandler)
		})
	})

	fileServer := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.StripPrefix("/static/", cacheStaticFiles(fileServer, deps.Public.StaticCacheMaxAge)))

	// Admin-only routes (register before generic path patterns to avoid conflicts)
	r.Group(func(adminRouter chi.Router) {
		adminRouter.Use(authMw.AdminOnly())

		if deps.Public.CSRFEnabled {
			adminRouter.Use(frontend_mw.ValidateCSRFToken())
		}

		adminRouter.Post("/blacklist/user", deps.Handler.BlacklistUserHandler)
		adminRouter.Post("/{board}/delete", deps.Handler.BoardDeleteHandler)
		adminRouter.Post("/{board}/{thread}/delete", deps.Handler.ThreadDeleteHandler)
		adminRouter.Post("/{board}/{thread}/pin", deps.Handler.ThreadTogglePinnedHandler)
		adminRouter.Post("/{board}/{thread}/{message}/delete", deps.Handler.MessageDeleteHandler)
	})

	// Authenticated routes
	r.Group(func(authRouter chi.Router) {
		authRouter.Use(authMw.NeedAuth())
		authRouter.Use(mw.RestrictBoardAccess(deps.AccessData)) // Enforce board access restrictions
		authRouter.Use(mw.RateLimit(rl.Rps100(), mw.GetUserIDFromContext))

		if deps.Public.CSRFEnabled {
			authRouter.Use(frontend_mw.ValidateCSRFToken())
		}

		mediaPath := deps.Handler.MediaPath
		authRouter.Handle("/media/{board}/*", http.StripPrefix("/media/", noDirectoryListing(http.FileServer(http.Dir(mediaPath)))))

		authRouter.Get("/", deps.Handler.IndexGetHandler)
		authRouter.Post("/", deps.Handler.IndexPostHandler)
		authRouter.HandleFunc("/logout", deps.Handler.LogoutHandler)

		// Invite routes (must be registered before /{board} to avoid route conflicts)
		authRouter.Get("/invites", deps.Handler.InvitesGetHandler)
		// GenerateInvite: 1 per minute per user (same as CreateThread)
		authRouter.With(mw.RateLimit(rl.OncePerMinute(), mw.GetUserIDFromContext)).Post("/invites/generate", deps.Handler.GenerateInvitePostHandler)
		authRouter.Post("/invites/revoke", deps.Handler.RevokeInvitePostHandler)

		// Account page
		authRouter.Get("/account", deps.Handler.AccountGetHandler)

		// Board routes with specific rate limits
		// GetBoard: 10 RPS per user
		authRouter.With(mw.RateLimit(rl.Rps10(), mw.GetUserIDFromContext)).Get("/{board}", deps.Handler.BoardGetHandler)
		authRouter.With(mw.RateLimit(rl.OncePerMinute(), mw.GetUserIDFromContext)).Post("/{board}", deps.Handler.BoardPostHandler)

		// Thread routes
		authRouter.Get("/{board}/{thread}", deps.Handler.ThreadGetHandler)
		authRouter.With(mw.RateLimit(rl.OncePerSecond(), mw.GetUserIDFromContext)).Post("/{board}/{thread}", deps.Handler.ThreadPostHandler)

		// API proxy for message preview (JSON)
		authRouter.Get("/api-proxy/v1/{board}/{thread}/{message}", deps.Handler.MessagePreviewHandler)
		// API endpoint for message preview (HTML)
		authRouter.Get("/api-proxy/v1/{board}/{thread}/{message}/html", deps.Handler.MessagePreviewHTMLHandler)
	})

	return r
}

// noDirectoryListing wraps a file server to return 404 for directory requests
func noDirectoryListing(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") || r.URL.Path == "" {
			http.NotFound(w, r)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// cacheStaticFiles wraps an http.Handler to add Cache-Control headers for static files
func cacheStaticFiles(h http.Handler, maxAge time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", int(maxAge.Seconds())))
		h.ServeHTTP(w, r)
	})
}
