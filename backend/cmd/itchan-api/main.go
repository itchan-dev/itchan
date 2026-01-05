package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/itchan-dev/itchan/backend/internal/router"
	"github.com/itchan-dev/itchan/backend/internal/service"
	"github.com/itchan-dev/itchan/backend/internal/setup"
	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/logger"
)

func main() {
	var configFolder string
	flag.StringVar(&configFolder, "config_folder", "config", "path to folder with configs")
	flag.Parse()

	cfg := config.MustLoad(configFolder)

	// Initialize logger with config settings
	useJSON := cfg.Public.LogFormat == "json"
	logger.Initialize(cfg.Public.LogLevel, useJSON)

	// Check ffmpeg availability for video sanitization
	if err := service.CheckFFmpegAvailable(); err != nil {
		logger.Log.Error("ffmpeg is required but not available", "error", err)
		fmt.Fprintln(os.Stderr, "ERROR: ffmpeg is required for video metadata stripping")
		fmt.Fprintln(os.Stderr, "Install: apk add ffmpeg (Alpine), apt install ffmpeg (Debian), brew install ffmpeg (macOS)")
		os.Exit(1)
	}
	logger.Log.Info("ffmpeg available for video processing")

	deps, err := setup.SetupDependencies(cfg)
	if err != nil {
		logger.Log.Error("failed to initialize dependencies", "error", err)
		os.Exit(1)
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
		logger.Log.Info("server starting", "port", httpPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	// Attempt graceful shutdown
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error("http server shutdown error", "error", err)
	} else {
		logger.Log.Info("http server gracefully stopped")
	}
}
