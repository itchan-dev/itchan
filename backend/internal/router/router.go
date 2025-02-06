package router

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"

	mw "github.com/itchan-dev/itchan/backend/internal/middleware"
	rl "github.com/itchan-dev/itchan/backend/internal/middleware/ratelimiter"
	"github.com/itchan-dev/itchan/backend/internal/setup"
)

// New creates and configures a new mux router with all the routes.
// IMPORTANT! ratelimiters set with .Use limit request for all endpoints combined in that subrouter
func New(deps *setup.Dependencies) *mux.Router {
	r := mux.NewRouter()

	h := deps.Handler
	jwt := deps.Jwt

	v1 := r.PathPrefix("/v1").Subrouter()
	v1.Use(mw.RestrictBoardAccess(deps.AccessData)) // if path has {board}, restrict access to certain email domain

	admin := v1.PathPrefix("/admin").Subrouter()
	admin.Use(mw.AdminOnly(jwt))
	admin.HandleFunc("/boards", h.CreateBoard).Methods("POST")
	admin.HandleFunc("/{board}", h.DeleteBoard).Methods("DELETE")
	admin.HandleFunc("/{board}/{thread}", h.DeleteThread).Methods("DELETE")
	admin.HandleFunc("/{board}/{thread}/{message}", h.DeleteMessage).Methods("DELETE")

	auth := v1.PathPrefix("/auth")
	// register func send msg via email
	// so rate limit it hard to prevent spam
	authSendingEmail := auth.Subrouter()
	authSendingEmail.Use(mw.RateLimit(rl.OnceInMinute(), mw.GetEmailFromBody)) // rate limit by email
	authSendingEmail.Use(mw.RateLimit(rl.OnceInMinute(), mw.GetIP))            // rate limit by ip
	authSendingEmail.Use(mw.GlobalRateLimit(rl.Rps100()))                      // rate limit to 100 rps total from all users
	authSendingEmail.HandleFunc("/register", h.Register).Methods("POST")
	// сheckConfirmationCode and login do database lookup
	// so we can afford higher rate limits
	authDbLookup := auth.Subrouter()
	authDbLookup.Use(mw.RateLimit(rl.OnceInSecond(), mw.GetEmailFromBody)) // rate limit by email
	authDbLookup.Use(mw.RateLimit(rl.OnceInSecond(), mw.GetIP))            // rate limit by ip
	authDbLookup.Use(mw.GlobalRateLimit(rl.Rps1000()))                     // rate limit to 1000 rps total from all users
	authDbLookup.HandleFunc("/check_confirmation_code", h.CheckConfirmationCode).Methods("POST")
	authDbLookup.HandleFunc("/login", h.Login).Methods("POST")
	// logout just deletes cookie, no need rate limits
	auth.Subrouter().HandleFunc("/logout", h.Logout).Methods("POST")

	loggedIn := v1.NewRoute().Subrouter()
	loggedIn.Use(mw.NeedAuth(jwt)) // check user token from cookie and add user to request context
	// loggedIn.Use(mw.RateLimit(rl.New(100, 100, time.Hour), mw.GetIP)) // maybe limit all actions fron single ip?
	loggedIn.Use(mw.RateLimit(rl.Rps100(), mw.GetEmailFromContext)) // 100 rps for single user to all loggedIn endpoints

	// costly operation, limit to 10 rps
	loggedIn.Handle("/{board}", mw.RateLimit(rl.Rps10(), mw.GetEmailFromContext)(http.HandlerFunc(h.GetBoard))).Methods("GET")
	// 1 thread per minute at max
	loggedIn.Handle("/{board}", mw.RateLimit(rl.OnceInMinute(), mw.GetEmailFromContext)(http.HandlerFunc(h.CreateThread))).Methods("POST")
	loggedIn.HandleFunc("/{board}/{thread}", h.GetThread).Methods("GET")
	// 1 msg per sec for single user
	loggedIn.Handle("/{board}/{thread}", mw.RateLimit(rl.New(1/15, 1, 1*time.Hour), mw.GetEmailFromContext)(http.HandlerFunc(h.CreateMessage))).Methods("POST")
	loggedIn.HandleFunc("/{board}/{thread}/{message}", h.GetMessage).Methods("GET")

	return r
}
