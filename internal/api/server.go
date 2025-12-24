package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/api/handlers"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/api/middleware"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/application/service"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/storage"
)

// Config holds API server configuration.
type Config struct {
	Port           int
	AllowedOrigins []string
}

// DefaultConfig returns sensible defaults for the API server.
func DefaultConfig() Config {
	return Config{
		Port:           8080,
		AllowedOrigins: []string{"http://localhost:3000", "http://localhost:5173"},
	}
}

// Server is the HTTP API server.
type Server struct {
	config      Config
	router      chi.Router
	httpServer  *http.Server
	logger      *slog.Logger
	repo        storage.Repository
	syncService *service.SyncService
}

// NewServer creates a new API server.
// If syncService is nil, sync endpoints will not be available.
func NewServer(cfg Config, repo storage.Repository, syncService *service.SyncService, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	s := &Server{
		config:      cfg,
		router:      chi.NewRouter(),
		logger:      logger,
		repo:        repo,
		syncService: syncService,
	}

	s.setupMiddleware()
	s.setupRoutes()

	return s
}

// setupMiddleware configures global middleware.
func (s *Server) setupMiddleware() {
	// CORS
	corsConfig := middleware.CORSConfig{
		AllowedOrigins: s.config.AllowedOrigins,
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type"},
	}
	s.router.Use(middleware.CORS(corsConfig))

	// Request logging
	s.router.Use(middleware.Logging(s.logger))
}

// setupRoutes configures all API routes.
func (s *Server) setupRoutes() {
	// Health check (no /api prefix - for load balancers)
	healthHandler := handlers.NewHealthHandler()
	s.router.Get("/health", healthHandler.ServeHTTP)

	// API routes
	s.router.Route("/api", func(r chi.Router) {
		// Orders
		ordersHandler := handlers.NewOrdersHandler(s.repo)
		r.Get("/orders", ordersHandler.List)
		r.Get("/orders/{id}", ordersHandler.Get)

		// Items
		itemsHandler := handlers.NewItemsHandler(s.repo)
		r.Get("/items/search", itemsHandler.Search)

		// Sync runs (historical)
		runsHandler := handlers.NewRunsHandler(s.repo)
		r.Get("/runs", runsHandler.List)
		r.Get("/runs/{id}", runsHandler.Get)

		// Stats
		statsHandler := handlers.NewStatsHandler(s.repo)
		r.Get("/stats", statsHandler.Get)

		// Ledgers
		ledgersHandler := handlers.NewLedgersHandler(s.repo)
		r.Get("/ledgers", ledgersHandler.List)
		r.Get("/ledgers/{id}", ledgersHandler.Get)
		r.Get("/orders/{orderID}/ledger", ledgersHandler.GetByOrderID)
		r.Get("/orders/{orderID}/ledgers", ledgersHandler.GetHistoryByOrderID)

		// Sync operations (live sync jobs)
		if s.syncService != nil {
			syncHandler := handlers.NewSyncHandler(s.syncService)
			r.Post("/sync", syncHandler.StartSync)
			r.Get("/sync", syncHandler.ListAllSyncs)
			r.Get("/sync/active", syncHandler.ListActiveSyncs)
			r.Get("/sync/{jobId}", syncHandler.GetSyncStatus)
			r.Delete("/sync/{jobId}", syncHandler.CancelSync)
		}
	})
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.config.Port)

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	s.logger.Info("starting API server", "addr", addr)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down API server")

	if s.httpServer == nil {
		return nil
	}

	return s.httpServer.Shutdown(ctx)
}

// Router returns the chi router for testing.
func (s *Server) Router() chi.Router {
	return s.router
}
