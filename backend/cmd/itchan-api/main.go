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

	storage, err := pg.New(cfg.Public)
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

	log.Print("Server started")
	httpPort := os.Getenv("PORT")
	if httpPort == "" {
		httpPort = "8080"
	}

	log.Fatal(http.ListenAndServe(":"+httpPort, r))
}
