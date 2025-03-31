package router

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/itchan-dev/itchan/frontend/internal/handler"
	"github.com/itchan-dev/itchan/frontend/internal/setup"
	mw "github.com/itchan-dev/itchan/shared/middleware"
)

func SetupRouter(deps *setup.Dependencies) *mux.Router {
	r := mux.NewRouter()

	// Public routes
	r.HandleFunc("/favicon.ico", handler.FaviconHandler)
	r.HandleFunc("/login", deps.Handler.LoginGetHandler).Methods("GET")
	r.HandleFunc("/login", deps.Handler.LoginPostHandler).Methods("POST")
	r.HandleFunc("/register", deps.Handler.RegisterGetHandler).Methods("GET")
	r.HandleFunc("/register", deps.Handler.RegisterPostHandler).Methods("POST")

	r.HandleFunc("/check_confirmation_code", deps.Handler.ConfirmEmailGetHandler).Methods("GET")
	r.HandleFunc("/check_confirmation_code", deps.Handler.ConfirmEmailPostHandler).Methods("POST")

	// Authenticated routes
	authRouter := r.NewRoute().Subrouter()
	authRouter.Use(mw.NeedAuth(deps.Jwt))
	authRouter.PathPrefix("/static/").Handler(
		http.StripPrefix("/static/", http.FileServer(http.Dir("static"))),
	)
	authRouter.HandleFunc("/", deps.Handler.IndexGetHandler).Methods("GET")
	authRouter.HandleFunc("/", deps.Handler.IndexPostHandler).Methods("POST")
	authRouter.HandleFunc("/logout", handler.LogoutHandler)
	authRouter.HandleFunc("/{board}/delete", handler.BoardDeleteHandler).Methods("POST")

	authRouter.HandleFunc("/{board}", deps.Handler.BoardGetHandler).Methods("GET")
	authRouter.HandleFunc("/{board}", deps.Handler.BoardPostHandler).Methods("POST")

	authRouter.HandleFunc("/{board}/{thread}", deps.Handler.ThreadGetHandler).Methods("GET")

	return r
}
