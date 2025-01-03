package router

import (
	"github.com/gorilla/mux"

	"github.com/itchan-dev/itchan/backend/internal/middleware"
	"github.com/itchan-dev/itchan/backend/internal/setup"
)

// New creates and configures a new mux router with all the routes.
func New(deps *setup.Dependencies) *mux.Router {
	r := mux.NewRouter()

	h := deps.Handler
	jwt := deps.Jwt

	r.HandleFunc("/v1/auth/signup", h.Signup).Methods("POST")
	r.HandleFunc("/v1/auth/login", h.Login).Methods("POST")
	r.HandleFunc("/v1/auth/logout", middleware.NeedAuth(jwt)(h.Logout)).Methods("POST")

	r.HandleFunc("/v1/auth/test_auth", middleware.NeedAuth(jwt)(h.Test)).Methods("GET")
	r.HandleFunc("/v1/test_auth", middleware.NeedAuth(jwt)(h.Test)).Methods("GET")
	r.HandleFunc("/v1/test_admin", middleware.AdminOnly(jwt)(h.Test)).Methods("GET")

	r.HandleFunc("/v1/boards", middleware.AdminOnly(jwt)(h.CreateBoard)).Methods("POST")
	r.HandleFunc("/v1/{board}", middleware.NeedAuth(jwt)(h.GetBoard)).Methods("GET")
	r.HandleFunc("/v1/{board}", middleware.AdminOnly(jwt)(h.DeleteBoard)).Methods("DELETE")

	r.HandleFunc("/v1/{board}", middleware.NeedAuth(jwt)(h.CreateThread)).Methods("POST")
	r.HandleFunc("/v1/{board}/{thread}", middleware.NeedAuth(jwt)(h.GetThread)).Methods("GET")
	r.HandleFunc("/v1/{board}/{thread}", middleware.AdminOnly(jwt)(h.DeleteThread)).Methods("DELETE")

	r.HandleFunc("/v1/{board}/{thread}", middleware.NeedAuth(jwt)(h.CreateMessage)).Methods("POST")
	r.HandleFunc("/v1/{board}/{thread}/{message}", middleware.NeedAuth(jwt)(h.GetMessage)).Methods("GET")
	r.HandleFunc("/v1/{board}/{thread}/{message}", middleware.AdminOnly(jwt)(h.DeleteMessage)).Methods("DELETE")

	return r
}
