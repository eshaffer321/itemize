// Package main is the entry point for the Walmart-Monarch sync backend server.
package main

import (
	"log"
	"time"

	"monarchmoney-sync-backend/config"
	"monarchmoney-sync-backend/handlers"

	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Initialize Sentry if DSN is provided
	if cfg.IsSentryEnabled() {
		if err := sentry.Init(sentry.ClientOptions{
			Dsn:              cfg.SentryDSN,
			TracesSampleRate: 1.0,
			Environment:      cfg.GinMode,
			BeforeSend: func(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
				// Filter out sensitive information
				if event.Request != nil {
					event.Request.Headers = nil
					event.Request.Cookies = ""
				}
				return event
			},
		}); err != nil {
			log.Printf("Sentry initialization failed: %v\n", err)
		} else {
			defer sentry.Flush(2 * time.Second)
			log.Println("Sentry error tracking initialized")
		}
	}

	// Set Gin mode
	if cfg.GinMode == "release" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	// Create router with config
	router := setupRouter(cfg)

	// Start server
	log.Printf("Starting Walmart-Monarch Sync Backend on port %s\n", cfg.Port)
	if err := router.Run(":" + cfg.Port); err != nil {
		sentry.CaptureException(err)
		log.Fatalf("Failed to start server: %v", err)
	}
}

func setupRouter(cfg *config.Config) *gin.Engine {
	router := gin.New()

	// Add logging middleware
	router.Use(gin.Logger())

	// Add recovery middleware that works with Sentry
	router.Use(gin.Recovery())

	// Add Sentry middleware if enabled
	if cfg.IsSentryEnabled() {
		router.Use(sentrygin.New(sentrygin.Options{
			Repanic:         true,
			WaitForDelivery: false,
			Timeout:         3 * time.Second,
		}))
	}

	// Health check endpoint (no auth required)
	router.GET("/health", handlers.HealthCheck)

	// API routes group with authentication
	api := router.Group("/api")
	api.Use(handlers.AuthMiddleware())
	{
		// Walmart endpoints
		walmart := api.Group("/walmart")
		{
			walmart.POST("/orders", handlers.ReceiveOrders)
			walmart.POST("/orders/batch", handlers.ReceiveBatchOrders)
			walmart.GET("/sync-status", handlers.GetSyncStatus)
		}

		// Test endpoint for Sentry (only in debug mode)
		if cfg.GinMode == "debug" {
			api.GET("/test-error", func(_ *gin.Context) {
				panic("Test error for Sentry")
			})
		}

		// Future endpoints
		// categories := api.Group("/categories")
		// {
		//     categories.GET("", handlers.ListCategories)
		// }
		//
		// transactions := api.Group("/transactions")
		// {
		//     transactions.POST("/split", handlers.SplitTransaction)
		//     transactions.GET("/:id/audit", handlers.GetAuditTrail)
		// }
	}

	return router
}
