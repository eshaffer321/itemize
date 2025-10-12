package main

import (
	// "embed"
	// "encoding/json"
	// "io/fs"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/storage"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// For production builds, uncomment this to embed the dashboard
// //go:embed all:dashboard/dist
// var dashboardFS embed.FS

type APIServer struct {
	storage *storage.Storage
	logger  *slog.Logger
}

func NewAPIServer(storage *storage.Storage, logger *slog.Logger) *APIServer {
	return &APIServer{
		storage: storage,
		logger:  logger,
	}
}

// Stats response
type StatsResponse struct {
	TotalTraces      int                `json:"total_traces"`
	SuccessCount     int                `json:"success_count"`
	FailureCount     int                `json:"failure_count"`
	SkippedCount     int                `json:"skipped_count"`
	ProcessingCount  int                `json:"processing_count"`
	TotalAmount      float64            `json:"total_amount"`
	AvgDuration      int64              `json:"avg_duration"`
	CategoryBreakdown map[string]int    `json:"category_breakdown"`
}

// Order response with all details
type OrderResponse struct {
	ID                    string             `json:"id"`
	OrderID               string             `json:"order_id"`
	Provider              string             `json:"provider"`
	Status                string             `json:"status"`
	OrderDate             string             `json:"order_date"`
	OrderTotal            float64            `json:"order_total"`
	ItemCount             int                `json:"item_count"`
	SplitCount            int                `json:"split_count"`
	Categories            []string           `json:"categories,omitempty"`
	Error                 string             `json:"error,omitempty"`
	Duration              int64              `json:"duration,omitempty"`
	CreatedAt             string             `json:"created_at"`
	MonarchTransactionID  string             `json:"monarch_transaction_id,omitempty"`
	DryRun                bool               `json:"dry_run"`
	Items                 []OrderItem        `json:"items,omitempty"`
	Splits                []TransactionSplit `json:"splits,omitempty"`
}

type OrderItem struct {
	Name       string  `json:"name"`
	Quantity   int     `json:"quantity"`
	UnitPrice  float64 `json:"unit_price"`
	TotalPrice float64 `json:"total_price"`
	Category   string  `json:"category,omitempty"`
}

type TransactionSplit struct {
	Category     string  `json:"category"`
	MerchantName string  `json:"merchant_name"`
	Amount       float64 `json:"amount"`
	Notes        string  `json:"notes,omitempty"`
}

func (s *APIServer) getStats(c *gin.Context) {
	stats, err := s.storage.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch stats"})
		return
	}

	categoryBreakdown, _ := s.storage.GetCategoryBreakdown()

	// Get counts from recent records to determine statuses
	records, _ := s.storage.GetRecentRecords(1000)
	var successCount, failureCount, skippedCount int
	for _, r := range records {
		switch r.Status {
		case "success":
			successCount++
		case "failed":
			failureCount++
		case "skipped":
			skippedCount++
		}
	}

	response := StatsResponse{
		TotalTraces:       stats.TotalProcessed,
		SuccessCount:      successCount,
		FailureCount:      failureCount,
		SkippedCount:      skippedCount,
		TotalAmount:       stats.TotalAmount,
		AvgDuration:       1000000, // Default to 1ms
		CategoryBreakdown: categoryBreakdown,
	}

	c.JSON(http.StatusOK, response)
}

func (s *APIServer) getRecentOrders(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 10
	}

	records, err := s.storage.GetRecentRecords(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch recent orders"})
		return
	}

	orders := make([]OrderResponse, 0, len(records))
	for _, record := range records {
		orders = append(orders, s.recordToOrderResponse(record))
	}

	c.JSON(http.StatusOK, orders)
}

func (s *APIServer) getOrders(c *gin.Context) {
	// Get filter parameters
	search := c.Query("search")
	status := c.Query("status")
	provider := c.Query("provider") 
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	limitStr := c.DefaultQuery("limit", "100")
	limit, _ := strconv.Atoi(limitStr)

	// For now, use recent records as we don't have a GetAllRecords method
	records, err := s.storage.GetRecentRecords(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch orders"})
		return
	}

	// Apply filters
	filtered := make([]*storage.ProcessingRecord, 0)
	for _, record := range records {
		// Search filter
		if search != "" && !strings.Contains(strings.ToLower(record.OrderID), strings.ToLower(search)) {
			continue
		}

		// Status filter
		if status != "" && record.Status != status {
			continue
		}

		// Provider filter - skip for now as provider field doesn't exist in ProcessingRecord
		if provider != "" && provider != "walmart" {
			continue
		}

		// Date filters
		if startDate != "" || endDate != "" {
			if startDate != "" {
				start, _ := time.Parse("2006-01-02", startDate)
				if record.ProcessedAt.Before(start) {
					continue
				}
			}
			if endDate != "" {
				end, _ := time.Parse("2006-01-02", endDate)
				end = end.Add(24 * time.Hour) // Include entire day
				if record.ProcessedAt.After(end) {
					continue
				}
			}
		}

		filtered = append(filtered, record)
	}

	// Convert to response format
	orders := make([]OrderResponse, 0, len(filtered))
	for _, record := range filtered {
		orders = append(orders, s.recordToOrderResponse(record))
	}

	c.JSON(http.StatusOK, orders)
}

func (s *APIServer) getOrderDetail(c *gin.Context) {
	orderID := c.Param("orderId")

	// Find record by order ID
	record, err := s.storage.GetRecord(orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	order := s.recordToOrderResponse(record)
	
	// Add detailed information from splits data if available
	// For now, we'll use placeholder data as the actual metadata isn't stored
	if record.SplitCount > 0 {
		// Create placeholder splits based on categories
		order.Splits = make([]TransactionSplit, 0, len(record.Categories))
		for _, cat := range record.Categories {
			order.Splits = append(order.Splits, TransactionSplit{
				Category:     cat,
				MerchantName: "Walmart",
				Amount:       record.OrderAmount / float64(len(record.Categories)),
			})
		}
	}

	c.JSON(http.StatusOK, order)
}

func (s *APIServer) recordToOrderResponse(record *storage.ProcessingRecord) OrderResponse {
	order := OrderResponse{
		ID:        strconv.FormatInt(record.ID, 10),
		OrderID:   record.OrderID,
		Provider:  "walmart", // Default to walmart for now
		Status:    record.Status,
		CreatedAt: record.ProcessedAt.Format(time.RFC3339),
		OrderDate: record.OrderDate.Format("2006-01-02"),
		OrderTotal: record.OrderAmount,
		SplitCount: record.SplitCount,
		Categories: record.Categories,
		MonarchTransactionID: record.TransactionID,
	}

	// Set dry run based on status
	if record.Status == "dry-run" {
		order.DryRun = true
	}

	// Set error if failed
	if record.Status == "failed" {
		order.Error = "Order processing failed"
	}

	return order
}

// Helper functions
func getStringFromMap(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func getIntFromMap(m map[string]interface{}, key string) int {
	if val, ok := m[key].(float64); ok {
		return int(val)
	}
	return 0
}

func getFloatFromMap(m map[string]interface{}, key string) float64 {
	if val, ok := m[key].(float64); ok {
		return val
	}
	return 0
}

func main() {
	// Initialize logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Initialize storage
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "sync_traces.db"
	}

	storage, err := storage.NewStorage(dbPath)
	if err != nil {
		logger.Error("Failed to initialize storage", "error", err)
		os.Exit(1)
	}
	defer storage.Close()

	// Initialize API server
	server := NewAPIServer(storage, logger)

	// Setup Gin router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/health"},
	}))

	// CORS configuration
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "http://localhost:5173", "http://localhost:8080"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// API routes
	api := router.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "healthy"})
		})
		api.GET("/stats", server.getStats)
		api.GET("/orders/recent", server.getRecentOrders)
		api.GET("/orders", server.getOrders)
		api.GET("/orders/:orderId", server.getOrderDetail)
	}

	// Serve dashboard in production
	// Uncomment this block when building for production with embedded dashboard
	/*
	if os.Getenv("ENV") != "development" {
		// Extract the embedded dashboard
		dashboardDist, err := fs.Sub(dashboardFS, "dashboard/dist")
		if err != nil {
			logger.Error("Failed to extract dashboard", "error", err)
		} else {
			// Serve static files
			router.StaticFS("/assets", http.FS(dashboardDist))
			
			// Serve index.html for all non-API routes
			router.NoRoute(func(c *gin.Context) {
				if !strings.HasPrefix(c.Request.URL.Path, "/api") {
					c.FileFromFS("/", http.FS(dashboardDist))
				}
			})
		}
	}
	*/

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger.Info("Starting API server", "port", port)
	if err := router.Run(":" + port); err != nil {
		logger.Error("Failed to start server", "error", err)
		os.Exit(1)
	}
}