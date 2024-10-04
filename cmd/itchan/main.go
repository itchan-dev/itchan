package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/itchan-dev/itchan/internal/config"
	"github.com/itchan-dev/itchan/internal/handler"
	"github.com/itchan-dev/itchan/internal/middleware"
	"github.com/itchan-dev/itchan/internal/models/auth"
	"github.com/itchan-dev/itchan/internal/models/board"
	"github.com/itchan-dev/itchan/internal/scripts"
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
	board := board.New(storage, &scripts.BoardNameValidator{})

	r := mux.NewRouter()
	h := handler.New(auth, board, cfg)

	r.HandleFunc("/auth/signup/", h.Signup).Methods("POST")
	r.HandleFunc("/auth/login/", h.Login).Methods("POST")
	r.Handle("/auth/logout/", middleware.NeedAuth(h.Logout, *jwt)).Methods("POST")

	r.Handle("/auth/test_auth/", middleware.NeedAuth(h.Test, *jwt)).Methods("GET")
	r.Handle("/test_auth/", middleware.NeedAuth(h.Test, *jwt)).Methods("GET")
	r.Handle("/test_admin/", middleware.AdminOnly(h.Test, *jwt)).Methods("GET")

	r.HandleFunc("/create_board/", middleware.AdminOnly(h.CreateBoard, *jwt)).Methods("POST")
	r.HandleFunc("/{board}/", middleware.NeedAuth(h.GetBoard, *jwt)).Methods("GET")
	r.HandleFunc("/{board}/", middleware.AdminOnly(h.DeleteBoard, *jwt)).Methods("DELETE")

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
