package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/itchan-dev/itchan/backend/internal/router"
	"github.com/itchan-dev/itchan/backend/internal/setup"
	"github.com/itchan-dev/itchan/shared/config"
)

func main() {
	log.SetFlags(log.Lshortfile)

	var configFolder string
	flag.StringVar(&configFolder, "config_folder", "backend/config", "path to folder with configs")
	flag.Parse()

	cfg := config.MustLoad(configFolder)

	deps, err := setup.SetupDependencies(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize dependencies: %v", err)
	}
	defer deps.Storage.Cleanup()

	r := router.New(deps)

	httpPort := os.Getenv("PORT")
	if httpPort == "" {
		httpPort = "8080"
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", httpPort),
		Handler: r,
	}

	// Channel to listen for interrupt or termination signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		log.Printf("Server starting on port %s", httpPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Block until a signal is received
	<-sigChan
	log.Println("Shutdown signal received, initiating graceful shutdown...")

	// Cancel the root context, triggering cleanup in dependencies
	deps.CancelFunc()

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	// Attempt graceful shutdown
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	} else {
		log.Println("HTTP server gracefully stopped")
	}
}
