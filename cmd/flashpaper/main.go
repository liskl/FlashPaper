// Package main is the entry point for FlashPaper, a zero-knowledge encrypted
// pastebin service. This file handles command-line argument parsing, configuration
// loading, and orchestrates the startup of all application components.
//
// FlashPaper is a Go implementation of PrivateBin, providing the same API
// compatibility and encryption model where all encryption/decryption happens
// client-side in the browser, ensuring the server never sees plaintext content.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/liskl/flashpaper/internal/config"
	"github.com/liskl/flashpaper/internal/server"
	"github.com/liskl/flashpaper/internal/storage"
)

// Version information set at build time via ldflags:
// go build -ldflags "-X main.version=1.0.0 -X main.commit=abc123"
var (
	version = "dev"
	commit  = "none"
)

func main() {
	// Parse command-line flags
	configPath := flag.String("config", "config.ini", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	// Handle version flag
	if *showVersion {
		fmt.Printf("FlashPaper %s (commit: %s)\n", version, commit)
		os.Exit(0)
	}

	// Load configuration from INI file and environment variables
	// Environment variables override file settings (12-factor app pattern)
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize the storage backend based on configuration
	// Supports: sqlite, postgres, mysql, filesystem
	store, err := storage.New(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()

	// Create and configure the HTTP server
	// The server handles all PrivateBin-compatible API endpoints
	srv, err := server.New(cfg, store)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Start the server in a goroutine so we can handle shutdown gracefully
	go func() {
		addr := fmt.Sprintf("%s:%d", cfg.Main.Host, cfg.Main.Port)
		log.Printf("FlashPaper %s starting on %s", version, addr)
		if err := srv.ListenAndServe(); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal (SIGINT or SIGTERM) for graceful shutdown
	// This ensures in-flight requests complete and resources are cleaned up
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Give outstanding requests up to 30 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped gracefully")
}
