package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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

	// Channel to listen for interrupt or termination signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		log.Printf("Starting frontend on :%s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Block until a signal is received
	<-sigChan
	log.Println("Shutdown signal received, initiating graceful shutdown...")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Frontend server shutdown error: %v", err)
	} else {
		log.Println("Frontend server gracefully stopped")
	}
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
