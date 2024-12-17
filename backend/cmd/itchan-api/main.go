package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/itchan-dev/itchan/backend/internal/handler"
	"github.com/itchan-dev/itchan/backend/internal/middleware"
	"github.com/itchan-dev/itchan/backend/internal/service"
	"github.com/itchan-dev/itchan/backend/internal/storage/pg"
	"github.com/itchan-dev/itchan/backend/internal/utils"
	"github.com/itchan-dev/itchan/backend/internal/utils/email"
	"github.com/itchan-dev/itchan/backend/internal/utils/jwt"
	"github.com/itchan-dev/itchan/shared/config"

	"github.com/gorilla/mux"
)

func main() {
	log.SetFlags(log.Lshortfile)

	var configFolder string
	flag.StringVar(&configFolder, "config_folder", "backend/config", "path to folder with configs")
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

	auth := service.NewAuth(storage, email, jwt)
	board := service.NewBoard(storage, &utils.BoardNameValidator{})
	thread := service.NewThread(storage, &utils.ThreadTitleValidator{})
	message := service.NewMessage(storage, &utils.MessageValidator{})

	r := mux.NewRouter()
	h := handler.New(auth, board, thread, message, cfg, jwt)

	r.HandleFunc("/v1/auth/signup", h.Signup).Methods("POST")
	r.HandleFunc("/v1/auth/login", h.Login).Methods("POST")
	r.Handle("/v1/auth/logout", middleware.NeedAuth(h.Logout, *jwt)).Methods("POST")

	r.Handle("/v1/auth/test_auth", middleware.NeedAuth(h.Test, *jwt)).Methods("GET")
	r.Handle("/v1/test_auth", middleware.NeedAuth(h.Test, *jwt)).Methods("GET")
	r.Handle("/v1/test_admin", middleware.AdminOnly(h.Test, *jwt)).Methods("GET")

	r.HandleFunc("/v1/boards", middleware.AdminOnly(h.CreateBoard, *jwt)).Methods("POST")
	r.HandleFunc("/v1/{board}", middleware.NeedAuth(h.GetBoard, *jwt)).Methods("GET")
	r.HandleFunc("/v1/{board}", middleware.AdminOnly(h.DeleteBoard, *jwt)).Methods("DELETE")

	r.HandleFunc("/v1/{board}", middleware.NeedAuth(middleware.ExtractUserId(h.CreateThread, *jwt), *jwt)).Methods("POST")
	r.HandleFunc("/v1/{board}/{thread}", middleware.NeedAuth(h.GetThread, *jwt)).Methods("GET")
	r.HandleFunc("/v1/{board}/{thread}", middleware.AdminOnly(h.DeleteThread, *jwt)).Methods("DELETE")

	r.HandleFunc("/v1/{board}/{thread}", middleware.NeedAuth(middleware.ExtractUserId(h.CreateMessage, *jwt), *jwt)).Methods("POST")
	r.HandleFunc("/v1/{board}/{thread}/{message}", middleware.NeedAuth(h.GetMessage, *jwt)).Methods("GET")
	r.HandleFunc("/v1/{board}/{thread}/{message}", middleware.AdminOnly(h.DeleteMessage, *jwt)).Methods("DELETE")

	log.Print("Server started")
	httpPort := os.Getenv("PORT")
	if httpPort == "" {
		httpPort = "8080"
	}

	log.Fatal(http.ListenAndServe(":"+httpPort, r))
}
