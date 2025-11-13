package router

import (
	"net/http"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/itchan-dev/itchan/frontend/internal/handler"
	"github.com/itchan-dev/itchan/frontend/internal/setup"
	mw "github.com/itchan-dev/itchan/shared/middleware"
	rl "github.com/itchan-dev/itchan/shared/middleware/ratelimiter"
)

func SetupRouter(deps *setup.Dependencies) *mux.Router {
	r := mux.NewRouter()
	r.StrictSlash(true)

	// Enable gzip compression for all responses (HTML, CSS, JS)
	r.Use(handlers.CompressHandler)

	// Public routes
	r.HandleFunc("/favicon.ico", handler.FaviconHandler)
	r.HandleFunc("/alino4ka", deps.Handler.ProposalHandler).Methods("GET")
	r.HandleFunc("/login", deps.Handler.LoginGetHandler).Methods("GET")
	r.HandleFunc("/login", deps.Handler.LoginPostHandler).Methods("POST")
	r.HandleFunc("/register", deps.Handler.RegisterGetHandler).Methods("GET")
	r.HandleFunc("/register", deps.Handler.RegisterPostHandler).Methods("POST")

	r.HandleFunc("/check_confirmation_code", deps.Handler.ConfirmEmailGetHandler).Methods("GET")
	r.HandleFunc("/check_confirmation_code", deps.Handler.ConfirmEmailPostHandler).Methods("POST")

	r.PathPrefix("/static/").Handler(
		http.StripPrefix("/static/", http.FileServer(http.Dir("static"))),
	)

	// Authenticated routes
	authRouter := r.NewRoute().Subrouter()
	authRouter.Use(mw.NeedAuth(deps.Jwt))
	authRouter.Use(mw.RestrictBoardAccess(deps.AccessData))           // Enforce board access restrictions
	authRouter.Use(mw.RateLimit(rl.Rps100(), mw.GetEmailFromContext)) // 100 RPS per user

	// Media file server - serve files from shared media directory
	mediaPath := deps.Handler.MediaPath
	authRouter.PathPrefix("/media/{board}/").Handler(
		http.StripPrefix("/media/", http.FileServer(http.Dir(mediaPath))),
	)

	authRouter.HandleFunc("/", deps.Handler.IndexGetHandler).Methods("GET")
	authRouter.HandleFunc("/", deps.Handler.IndexPostHandler).Methods("POST")
	authRouter.HandleFunc("/logout", deps.Handler.LogoutHandler)

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

	// Admin-only routes
	adminRouter := r.NewRoute().Subrouter()
	adminRouter.Use(mw.AdminOnly(deps.Jwt))
	adminRouter.HandleFunc("/{board}/delete", deps.Handler.BoardDeleteHandler).Methods("POST")
	adminRouter.HandleFunc("/{board}/{thread}/delete", deps.Handler.ThreadDeleteHandler).Methods("POST")
	adminRouter.HandleFunc("/{board}/{thread}/{message}/delete", deps.Handler.MessageDeleteHandler).Methods("POST")

	return r
}
