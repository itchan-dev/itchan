package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/itchan-dev/itchan/internal/config"
	"github.com/itchan-dev/itchan/internal/handler"
	"github.com/itchan-dev/itchan/internal/logic/auth"
	"github.com/itchan-dev/itchan/internal/middleware"
	"github.com/itchan-dev/itchan/internal/scripts/email"
	"github.com/itchan-dev/itchan/internal/scripts/jwt"
	"github.com/itchan-dev/itchan/internal/storage/pg"

	"github.com/gorilla/mux"
)

func main() {
	log.SetFlags(log.Lshortfile)

	var configFolder string
	flag.StringVar(&configFolder, "config_folder", "config", "path to folder with configs")
	flag.Parse()
	// to do, move all init to different place
	cfg := config.MustLoad(configFolder)

	storage, err := pg.New(cfg.Public.Pg)
	if err != nil {
		log.Fatal(err)
	}
	defer storage.Cleanup()

	email := email.New()

	jwt := jwt.New(cfg.JwtKey(), cfg.JwtTTL())

	auth := auth.New(storage, email, jwt)

	r := mux.NewRouter()
	h := handler.New(auth, cfg)

	r.HandleFunc("/auth/signup", h.Signup).Methods("POST")
	r.HandleFunc("/auth/login", h.Login).Methods("POST")
	r.Handle("/auth/logout", middleware.NewAuth(http.HandlerFunc(h.Logout), *cfg, *jwt)).Methods("POST")

	r.Handle("/auth/test_auth", middleware.NewAuth(http.HandlerFunc(h.Test), *cfg, *jwt)).Methods("GET")
	r.Handle("/test_auth", middleware.NewAuth(http.HandlerFunc(h.Test), *cfg, *jwt)).Methods("GET")

	// r.HandleFunc("/create_board", middleware.AdminOnly(handler.CreateBoard)).Methods("POST")
	// r.HandleFunc("/{board}", middleware.Auth(handler.GetBoard)).Methods("GET")
	// r.HandleFunc("/{board}", middleware.AdminOnly(handler.DeleteBoard)).Methods("DELETE")

	// r.HandleFunc("/{board}/create_thread", middleware.Auth(handler.CreateThread)).Methods("POST")
	// r.HandleFunc("/{board}/thread/{thread}", middleware.Auth(handler.GetThread)).Methods("GET")
	// r.HandleFunc("/{board}/thread/{thread}", middleware.AdminOnly(handler.DeleteThread)).Methods("DELETE")

	// r.HandleFunc("/{board}/{thread}/reply", middleware.Auth(handler.CreateMessage)).Methods("POST")
	// r.HandleFunc("/{board}/{thread}/{message}", middleware.Auth(handler.GetMessage)).Methods("GET")
	// r.HandleFunc("/{board}/{thread}/{message}", middleware.AdminOnly(handler.DeleteMessage)).Methods("DELETE")

	log.Print("Server started")
	httpPort := os.Getenv("PORT")
	if httpPort == "" {
		httpPort = "8080"
	}

	log.Fatal(http.ListenAndServe(":"+httpPort, r))
}
