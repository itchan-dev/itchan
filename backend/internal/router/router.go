package router

import (
	"github.com/gorilla/mux"

	mw "github.com/itchan-dev/itchan/backend/internal/middleware"
	"github.com/itchan-dev/itchan/backend/internal/middleware/ratelimiter"
	"github.com/itchan-dev/itchan/backend/internal/setup"
)

// New creates and configures a new mux router with all the routes.
func New(deps *setup.Dependencies) *mux.Router {
	r := mux.NewRouter()

	h := deps.Handler
	jwt := deps.Jwt

	r.HandleFunc("/v1/auth/register", mw.LimitByIpAndEmail(ratelimiter.OnceInMinute, h.Register)).Methods("POST")
	r.HandleFunc("/v1/auth/check_confirmation_code", mw.LimitByIpAndEmail(ratelimiter.OnceInSecond, h.Register)).Methods("POST")
	r.HandleFunc("/v1/auth/login", mw.RateLimit(ratelimiter.OnceInSecond, mw.GetEmailFromBody)(h.Login)).Methods("POST")
	r.HandleFunc("/v1/auth/logout", mw.NeedAuth(jwt)(h.Logout)).Methods("POST")

	// r.HandleFunc("/v1/auth/test_auth", mw.NeedAuth(jwt)(h.Test)).Methods("GET")
	// r.HandleFunc("/v1/test_auth", mw.NeedAuth(jwt)(h.Test)).Methods("GET")
	// r.HandleFunc("/v1/test_admin", mw.AdminOnly(jwt)(h.Test)).Methods("GET")

	r.HandleFunc("/v1/boards", mw.AdminOnly(jwt)(h.CreateBoard)).Methods("POST")
	r.HandleFunc("/v1/{board}", mw.NeedAuth(jwt)(h.GetBoard)).Methods("GET")
	r.HandleFunc("/v1/{board}", mw.AdminOnly(jwt)(h.DeleteBoard)).Methods("DELETE")

	r.HandleFunc("/v1/{board}", mw.NeedAuth(jwt)(h.CreateThread)).Methods("POST")
	r.HandleFunc("/v1/{board}/{thread}", mw.NeedAuth(jwt)(h.GetThread)).Methods("GET")
	r.HandleFunc("/v1/{board}/{thread}", mw.AdminOnly(jwt)(h.DeleteThread)).Methods("DELETE")

	r.HandleFunc("/v1/{board}/{thread}", mw.NeedAuth(jwt)(h.CreateMessage)).Methods("POST")
	r.HandleFunc("/v1/{board}/{thread}/{message}", mw.NeedAuth(jwt)(h.GetMessage)).Methods("GET")
	r.HandleFunc("/v1/{board}/{thread}/{message}", mw.AdminOnly(jwt)(h.DeleteMessage)).Methods("DELETE")

	return r
}
