package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/itchan-dev/itchan/frontend/internal/router"
	"github.com/itchan-dev/itchan/frontend/internal/setup"
)

const (
	defaultPort            = "8081"
	templateReloadInterval = 5 * time.Second
	apiBaseURL             = "http://api:8080/v1"
	cookieName             = "accessToken"
	readTimeout            = 5 * time.Second
	writeTimeout           = 10 * time.Second
)

func main() {
	log.SetFlags(log.Lshortfile)
	deps := setup.SetupDependencies()
	router := router.SetupRouter(deps)

	server := configureServer(router)
	log.Printf("Starting frontend on :%s", server.Addr)
	log.Fatal(server.ListenAndServe())
}

func configureServer(handler http.Handler) *http.Server {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	return &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}
}
