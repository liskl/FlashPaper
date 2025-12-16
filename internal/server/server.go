// Package server provides the HTTP server for FlashPaper.
// It configures routing, middleware, and serves the PrivateBin-compatible API
// along with the web frontend.
package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/liskl/flashpaper/internal/config"
	"github.com/liskl/flashpaper/internal/handler"
	fpMiddleware "github.com/liskl/flashpaper/internal/middleware"
	"github.com/liskl/flashpaper/internal/storage"
)

// Server wraps the HTTP server with FlashPaper configuration.
type Server struct {
	httpServer *http.Server
	config     *config.Config
	store      storage.Storage
}

// New creates a new FlashPaper HTTP server.
func New(cfg *config.Config, store storage.Storage) (*Server, error) {
	// Create the main router
	r := chi.NewRouter()

	// Apply middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Security headers
	r.Use(fpMiddleware.SecurityHeaders(cfg))

	// Create the main handler
	h := handler.New(cfg, store)

	// Mount routes
	r.Mount("/", h.Routes())

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Main.Host, cfg.Main.Port)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return &Server{
		httpServer: httpServer,
		config:     cfg,
		store:      store,
	}, nil
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// Addr returns the server's address.
func (s *Server) Addr() string {
	return s.httpServer.Addr
}
