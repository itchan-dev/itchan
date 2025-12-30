package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/itchan-dev/itchan/frontend/internal/router"
	"github.com/itchan-dev/itchan/frontend/internal/setup"
	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/logger"
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
	var configFolder string
	flag.StringVar(&configFolder, "config_folder", "config", "path to folder with configs")
	flag.Parse()

	cfg := config.MustLoad(configFolder)

	// Initialize logger with config settings
	useJSON := cfg.Public.LogFormat == "json"
	logger.Initialize(cfg.Public.LogLevel, useJSON)

	deps, err := setup.SetupDependencies(cfg)
	if err != nil {
		logger.Log.Error("failed to initialize dependencies", "error", err)
		os.Exit(1)
	}
	defer deps.Storage.Cleanup()

	router := router.SetupRouter(deps)
	server := configureServer(router)

	// Channel to listen for interrupt or termination signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		logger.Log.Info("starting frontend", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Block until a signal is received
	<-sigChan
	logger.Log.Info("shutdown signal received, initiating graceful shutdown")

	// Cancel the root context, triggering cleanup in dependencies
	deps.CancelFunc()

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error("frontend server shutdown error", "error", err)
	} else {
		logger.Log.Info("frontend server gracefully stopped")
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
