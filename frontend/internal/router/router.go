package router

import (
	"encoding/base64"
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
	sharedutils "github.com/itchan-dev/itchan/shared/utils"
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

	allowedRefs := sharedutils.NewAllowedSources(deps.Private.AllowedRefs)

	// Referral tracking config (shared between visit and action middleware)
	referralCfg := frontend_mw.ReferralConfig{
		SecureCookies: deps.Public.SecureCookies,
		AllowedRefs:   allowedRefs,
		RecordAction:  deps.Handler.APIClient.RecordReferralAction,
	}

	// Referral tracking middleware: captures ?ref= param into cookie on first visit
	r.Use(frontend_mw.ReferralTracking(referralCfg))

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
	r.With(frontend_mw.TrackReferralAction("get_login", referralCfg)).Get("/login", deps.Handler.LoginGetHandler)
	r.With(frontend_mw.TrackReferralAction("get_register", referralCfg)).Get("/register", deps.Handler.RegisterGetHandler)
	r.With(frontend_mw.TrackReferralAction("get_register_invite", referralCfg)).Get("/register_invite", deps.Handler.RegisterInviteGetHandler)
	r.With(frontend_mw.TrackReferralAction("get_check_confirmation_code", referralCfg)).Get("/check_confirmation_code", deps.Handler.ConfirmEmailGetHandler)

	// Create frontend auth middleware wrapper (needed for optional auth routes below)
	authMw := frontend_mw.NewAuth(deps.AuthMiddleware, deps.Public.SecureCookies)

	// Public routes with optional auth (shows user info if logged in)
	r.Group(func(optionalAuthRouter chi.Router) {
		optionalAuthRouter.Use(authMw.OptionalAuth())
		optionalAuthRouter.Get("/welcome", deps.Handler.WelcomeGetHandler)
		optionalAuthRouter.With(frontend_mw.TrackReferralAction("get_faq", referralCfg)).Get("/faq", deps.Handler.FAQGetHandler)
		optionalAuthRouter.Get("/about", deps.Handler.AboutGetHandler)
		optionalAuthRouter.Get("/terms", deps.Handler.TermsGetHandler)
		optionalAuthRouter.Get("/privacy", deps.Handler.PrivacyGetHandler)
		optionalAuthRouter.Get("/contacts", deps.Handler.ContactsGetHandler)
	})

	// Public board reading routes (optional auth, board access restricted to public boards for anon users)
	r.Group(func(publicBoard chi.Router) {
		publicBoard.Use(authMw.OptionalAuth())
		publicBoard.Use(mw.RestrictBoardAccess(deps.AccessData))
		publicBoard.Use(mw.RateLimit(rl.Rps10(), mw.GetIP))

		mediaPath := deps.Handler.MediaPath
		publicBoard.Handle("/media/{board}/*", http.StripPrefix("/media/", noDirectoryListing(http.FileServer(http.Dir(mediaPath)))))

		publicBoard.Get("/", deps.Handler.IndexGetHandler)
		publicBoard.Get("/{board}", deps.Handler.BoardGetHandler)
		publicBoard.With(frontend_mw.TrackReferralAction("get_thread", referralCfg)).Get("/{board}/{thread}", deps.Handler.ThreadGetHandler)

		// API proxy for message preview (JSON and HTML)
		publicBoard.Get("/api-proxy/v1/{board}/{thread}/{message}", deps.Handler.MessagePreviewHandler)
		publicBoard.Get("/api-proxy/v1/{board}/{thread}/{message}/html", deps.Handler.MessagePreviewHTMLHandler)
	})

	// Flash redirect handler for rate-limited POST routes
	onRateLimitExceeded := rateLimitExceededRedirect(deps.Public.SecureCookies)

	// Public POST routes (rate limited to prevent abuse)
	r.Group(func(publicPosts chi.Router) {
		publicPosts.Use(mw.GlobalRateLimit(rl.Rps100()))
		publicPosts.Use(mw.RateLimitWithHandler(rl.New(10.0/60.0, 10, 1*time.Hour), mw.GetIP, onRateLimitExceeded)) // 10 per minute by IP (backup)

		// Using email field
		publicPosts.Group(func(publicPostsEmail chi.Router) {
			publicPostsEmail.Use(mw.RateLimitWithHandler(rl.New(5.0/60.0, 5, 1*time.Hour), mw.GetEmailFromForm, onRateLimitExceeded)) // 5 attempts per minute by email
			publicPostsEmail.Post("/login", deps.Handler.LoginPostHandler)
			publicPostsEmail.Post("/register", deps.Handler.RegisterPostHandler)
			publicPostsEmail.With(frontend_mw.TrackReferralAction("registration", referralCfg)).Post("/check_confirmation_code", deps.Handler.ConfirmEmailPostHandler)
		})

		// Using invite_code field
		publicPosts.Group(func(publicPostsInvite chi.Router) {
			publicPostsInvite.Use(mw.RateLimitWithHandler(rl.New(5.0/60.0, 5, 1*time.Hour), mw.GetFieldFromForm("invite_code"), onRateLimitExceeded)) // 5 attempts per minute by each invite code
			publicPostsInvite.With(frontend_mw.TrackReferralAction("registration", referralCfg)).Post("/register_invite", deps.Handler.RegisterInvitePostHandler)
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

		adminRouter.Get("/admin", deps.Handler.AdminGetHandler)
		adminRouter.Post("/admin/unblacklist", deps.Handler.UnblacklistUserHandler)
		adminRouter.Post("/blacklist/user", deps.Handler.BlacklistUserHandler)
		adminRouter.Post("/{board}/delete", deps.Handler.BoardDeleteHandler)
		adminRouter.Post("/{board}/{thread}/delete", deps.Handler.ThreadDeleteHandler)
		adminRouter.Post("/{board}/{thread}/pin", deps.Handler.ThreadTogglePinnedHandler)
		adminRouter.Post("/{board}/{thread}/{message}/delete", deps.Handler.MessageDeleteHandler)
	})

	// Authenticated routes (write operations and user-specific pages)
	r.Group(func(authRouter chi.Router) {
		authRouter.Use(authMw.NeedAuth())
		authRouter.Use(mw.RestrictBoardAccess(deps.AccessData)) // Enforce board access restrictions
		authRouter.Use(mw.RateLimit(rl.Rps100(), mw.GetUserIDFromContext))

		if deps.Public.CSRFEnabled {
			authRouter.Use(frontend_mw.ValidateCSRFToken())
		}

		authRouter.Get("/settings/disable-media", deps.Handler.ToggleDisableMedia)

		authRouter.Post("/", deps.Handler.IndexPostHandler)
		authRouter.HandleFunc("/logout", deps.Handler.LogoutHandler)

		// Invite routes (must be registered before /{board} to avoid route conflicts)
		authRouter.Get("/invites", deps.Handler.InvitesGetHandler)
		// GenerateInvite: 1 per minute per user (same as CreateThread)
		authRouter.With(mw.RateLimitWithHandler(rl.OncePerMinute(), mw.GetUserIDFromContext, onRateLimitExceeded)).Post("/invites/generate", deps.Handler.GenerateInvitePostHandler)
		authRouter.Post("/invites/revoke", deps.Handler.RevokeInvitePostHandler)

		// Account page
		authRouter.Get("/account", deps.Handler.AccountGetHandler)

		// Board write routes
		authRouter.With(mw.RateLimitWithHandler(rl.OncePerMinute(), mw.GetUserIDFromContext, onRateLimitExceeded)).Post("/{board}", deps.Handler.BoardPostHandler)
		authRouter.With(mw.RateLimitWithHandler(rl.OncePerSecond(), mw.GetUserIDFromContext, onRateLimitExceeded)).Post("/{board}/{thread}", deps.Handler.ThreadPostHandler)
	})

	return r
}

// rateLimitExceededRedirect returns a handler that sets a flash error cookie and redirects back.
// Used for POST rate limits so users see a friendly error instead of a plain text 429 page.
func rateLimitExceededRedirect(secureCookies bool) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		message := base64.StdEncoding.EncodeToString([]byte("Слишком много запросов. Подождите немного."))
		http.SetCookie(w, &http.Cookie{
			Name:     "flash_error",
			Value:    message,
			Path:     "/",
			MaxAge:   300,
			HttpOnly: true,
			Secure:   secureCookies,
			SameSite: http.SameSiteLaxMode,
		})
		target := r.Referer()
		if target == "" {
			target = "/"
		}
		http.Redirect(w, r, target, http.StatusSeeOther)
	}
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
