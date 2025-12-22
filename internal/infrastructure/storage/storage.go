package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Storage provides database access for processing records
type Storage struct {
	db *sql.DB
}

// NewStorage creates a new storage instance with SQLite database
func NewStorage(dbPath string) (*Storage, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Enable foreign key constraints (SQLite-specific)
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	s := &Storage{db: db}

	// Run all pending migrations
	if err := s.runMigrations(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return s, nil
}

// Close closes the database connection
func (s *Storage) Close() error {
	return s.db.Close()
}

// ProcessingRecord includes detailed split information
type ProcessingRecord struct {
	ID                int64     `json:"id"`
	OrderID           string    `json:"order_id"`
	Provider          string    `json:"provider"`
	TransactionID     string    `json:"transaction_id"`
	OrderDate         time.Time `json:"order_date"`
	ProcessedAt       time.Time `json:"processed_at"`
	OrderTotal        float64   `json:"order_total"`
	OrderSubtotal     float64   `json:"order_subtotal"`
	OrderTax          float64   `json:"order_tax"`
	OrderTip          float64   `json:"order_tip"`
	TransactionAmount float64   `json:"transaction_amount"`
	SplitCount        int       `json:"split_count"`
	Status            string    `json:"status"`
	ErrorMessage      string    `json:"error_message,omitempty"`
	ItemCount         int       `json:"item_count"`
	MatchConfidence   float64   `json:"match_confidence"`
	DryRun            bool      `json:"dry_run"`

	// Detailed data stored as JSON
	Items      []OrderItem   `json:"items"`
	Splits     []SplitDetail `json:"splits"`
	ItemsJSON  string        `json:"-"` // For DB storage
	SplitsJSON string        `json:"-"` // For DB storage

	// Multi-delivery tracking (JSON)
	MultiDeliveryData string `json:"multi_delivery_data,omitempty"` // For DB storage
}

// OrderItem represents an item in the order
type OrderItem struct {
	Name       string  `json:"name"`
	Quantity   float64 `json:"quantity"`
	UnitPrice  float64 `json:"unit_price"`
	TotalPrice float64 `json:"total_price"`
	Category   string  `json:"category,omitempty"`
}

// SplitDetail represents how the transaction was split
type SplitDetail struct {
	CategoryID   string      `json:"category_id"`
	CategoryName string      `json:"category_name"`
	Amount       float64     `json:"amount"`
	Items        []OrderItem `json:"items"`
	Notes        string      `json:"notes"`
}

// MultiDeliveryInfo tracks multi-delivery consolidation metadata
type MultiDeliveryInfo struct {
	IsMultiDelivery           bool      `json:"is_multi_delivery"`
	ChargeCount               int       `json:"charge_count"`
	OriginalTransactionIDs    []string  `json:"original_transaction_ids"`
	ChargeAmounts             []float64 `json:"charge_amounts"`
	ConsolidatedTransactionID string    `json:"consolidated_transaction_id"`
}

// SetMultiDeliveryInfo serializes multi-delivery metadata to JSON for storage
func (r *ProcessingRecord) SetMultiDeliveryInfo(info *MultiDeliveryInfo) error {
	if info == nil {
		r.MultiDeliveryData = ""
		return nil
	}

	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	r.MultiDeliveryData = string(data)
	return nil
}

// GetMultiDeliveryInfo deserializes multi-delivery metadata from JSON
func (r *ProcessingRecord) GetMultiDeliveryInfo() (*MultiDeliveryInfo, error) {
	if r.MultiDeliveryData == "" {
		return nil, nil
	}

	var info MultiDeliveryInfo
	err := json.Unmarshal([]byte(r.MultiDeliveryData), &info)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

// SaveRecord saves an enhanced processing record
func (s *Storage) SaveRecord(record *ProcessingRecord) error {
	itemsJSON, _ := json.Marshal(record.Items)
	splitsJSON, _ := json.Marshal(record.Splits)

	query := `
	INSERT OR REPLACE INTO processing_records
	(order_id, provider, transaction_id, order_date, processed_at,
	 order_total, order_subtotal, order_tax, order_tip, transaction_amount,
	 split_count, status, error_message, item_count, match_confidence,
	 dry_run, items_json, splits_json, multi_delivery_data)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query,
		record.OrderID,
		record.Provider,
		record.TransactionID,
		record.OrderDate,
		record.ProcessedAt,
		record.OrderTotal,
		record.OrderSubtotal,
		record.OrderTax,
		record.OrderTip,
		record.TransactionAmount,
		record.SplitCount,
		record.Status,
		record.ErrorMessage,
		record.ItemCount,
		record.MatchConfidence,
		record.DryRun,
		string(itemsJSON),
		string(splitsJSON),
		record.MultiDeliveryData,
	)

	return err
}

// GetRecord retrieves an enhanced record by order ID
func (s *Storage) GetRecord(orderID string) (*ProcessingRecord, error) {
	query := `
	SELECT id, order_id, provider, transaction_id, order_date, processed_at,
	       order_total, order_subtotal, order_tax, order_tip, transaction_amount,
	       split_count, status, error_message, item_count, match_confidence,
	       dry_run, items_json, splits_json, multi_delivery_data
	FROM processing_records WHERE order_id = ?
	`

	record := &ProcessingRecord{}
	var multiDeliveryData sql.NullString
	err := s.db.QueryRow(query, orderID).Scan(
		&record.ID,
		&record.OrderID,
		&record.Provider,
		&record.TransactionID,
		&record.OrderDate,
		&record.ProcessedAt,
		&record.OrderTotal,
		&record.OrderSubtotal,
		&record.OrderTax,
		&record.OrderTip,
		&record.TransactionAmount,
		&record.SplitCount,
		&record.Status,
		&record.ErrorMessage,
		&record.ItemCount,
		&record.MatchConfidence,
		&record.DryRun,
		&record.ItemsJSON,
		&record.SplitsJSON,
		&multiDeliveryData,
	)

	if err != nil {
		return nil, err
	}

	// Unmarshal JSON fields (errors ignored as these are optional enrichment fields)
	if record.ItemsJSON != "" {
		_ = json.Unmarshal([]byte(record.ItemsJSON), &record.Items)
	}
	if record.SplitsJSON != "" {
		_ = json.Unmarshal([]byte(record.SplitsJSON), &record.Splits)
	}
	if multiDeliveryData.Valid {
		record.MultiDeliveryData = multiDeliveryData.String
	}

	return record, nil
}

// GetStats returns enhanced statistics
func (s *Storage) GetStats() (*Stats, error) {
	stats := &Stats{
		ProviderStats: make(map[string]ProviderStats),
	}

	// Overall stats
	query := `
	SELECT 
		COUNT(*) as total,
		COUNT(CASE WHEN status = 'success' THEN 1 END) as success,
		COUNT(CASE WHEN status = 'failed' THEN 1 END) as failed,
		COUNT(CASE WHEN status = 'skipped' THEN 1 END) as skipped,
		COUNT(CASE WHEN dry_run = 1 THEN 1 END) as dry_run,
		COALESCE(SUM(order_total), 0) as total_amount,
		COALESCE(AVG(order_total), 0) as avg_order,
		COALESCE(SUM(split_count), 0) as total_splits
	FROM processing_records
	WHERE processed_at > datetime('now', '-30 days')
	`

	err := s.db.QueryRow(query).Scan(
		&stats.TotalProcessed,
		&stats.SuccessCount,
		&stats.FailedCount,
		&stats.SkippedCount,
		&stats.DryRunCount,
		&stats.TotalAmount,
		&stats.AverageOrderAmount,
		&stats.TotalSplits,
	)
	if err != nil {
		return nil, err
	}

	// Provider breakdown
	provQuery := `
	SELECT 
		provider,
		COUNT(*) as count,
		COALESCE(SUM(order_total), 0) as total,
		COUNT(CASE WHEN status = 'success' THEN 1 END) as success
	FROM processing_records
	GROUP BY provider
	`

	rows, err := s.db.Query(provQuery)
	if err == nil {
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var provider string
			var ps ProviderStats
			if err := rows.Scan(&provider, &ps.Count, &ps.TotalAmount, &ps.SuccessCount); err == nil {
				stats.ProviderStats[provider] = ps
			}
		}
	}

	return stats, nil
}

// Stats contains enhanced statistics
type Stats struct {
	TotalProcessed     int                      `json:"total_processed"`
	SuccessCount       int                      `json:"success_count"`
	FailedCount        int                      `json:"failed_count"`
	SkippedCount       int                      `json:"skipped_count"`
	DryRunCount        int                      `json:"dry_run_count"`
	TotalAmount        float64                  `json:"total_amount"`
	AverageOrderAmount float64                  `json:"average_order_amount"`
	TotalSplits        int                      `json:"total_splits"`
	ProviderStats      map[string]ProviderStats `json:"provider_stats"`
}

// ProviderStats contains per-provider statistics
type ProviderStats struct {
	Count        int     `json:"count"`
	SuccessCount int     `json:"success_count"`
	TotalAmount  float64 `json:"total_amount"`
}

// IsProcessed checks if an order has already been successfully processed (non-dry-run)
func (s *Storage) IsProcessed(orderID string) bool {
	var count int
	query := `SELECT COUNT(*) FROM processing_records WHERE order_id = ? AND dry_run = 0 AND status = 'success'`
	err := s.db.QueryRow(query, orderID).Scan(&count)
	return err == nil && count > 0
}

// StartSyncRun records the start of a sync run
func (s *Storage) StartSyncRun(provider string, lookbackDays int, dryRun bool) (int64, error) {
	query := `
		INSERT INTO sync_runs (provider, lookback_days, dry_run, status)
		VALUES (?, ?, ?, 'running')
	`

	result, err := s.db.Exec(query, provider, lookbackDays, dryRun)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

// CompleteSyncRun records the completion of a sync run
func (s *Storage) CompleteSyncRun(runID int64, ordersFound, processed, skipped, errors int) error {
	query := `
		UPDATE sync_runs
		SET completed_at = CURRENT_TIMESTAMP,
		    orders_found = ?,
		    orders_processed = ?,
		    orders_skipped = ?,
		    orders_errored = ?,
		    status = CASE WHEN ? > 0 THEN 'completed_with_errors' ELSE 'completed' END
		WHERE id = ?
	`

	_, err := s.db.Exec(query, ordersFound, processed, skipped, errors, errors, runID)
	return err
}

// APICall represents a logged API call
type APICall struct {
	RunID        int64
	OrderID      string
	Method       string
	RequestJSON  string
	ResponseJSON string
	Error        string
	DurationMs   int64
}

// LogAPICall logs an API call to the database
func (s *Storage) LogAPICall(call *APICall) error {
	query := `
		INSERT INTO api_calls
		(run_id, order_id, method, request_json, response_json, error, duration_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query,
		call.RunID,
		call.OrderID,
		call.Method,
		call.RequestJSON,
		call.ResponseJSON,
		call.Error,
		call.DurationMs,
	)

	return err
}

// GetAPICallsByOrderID retrieves all API calls for a specific order
func (s *Storage) GetAPICallsByOrderID(orderID string) ([]APICall, error) {
	query := `
		SELECT run_id, order_id, method, request_json, response_json, error, duration_ms, timestamp
		FROM api_calls
		WHERE order_id = ?
		ORDER BY timestamp ASC
	`

	rows, err := s.db.Query(query, orderID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var calls []APICall
	for rows.Next() {
		var call APICall
		var timestamp string
		err := rows.Scan(
			&call.RunID,
			&call.OrderID,
			&call.Method,
			&call.RequestJSON,
			&call.ResponseJSON,
			&call.Error,
			&call.DurationMs,
			&timestamp,
		)
		if err != nil {
			return nil, err
		}
		calls = append(calls, call)
	}

	return calls, rows.Err()
}

// GetAPICallsByRunID retrieves all API calls for a specific sync run
func (s *Storage) GetAPICallsByRunID(runID int64) ([]APICall, error) {
	query := `
		SELECT run_id, order_id, method, request_json, response_json, error, duration_ms, timestamp
		FROM api_calls
		WHERE run_id = ?
		ORDER BY timestamp ASC
	`

	rows, err := s.db.Query(query, runID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var calls []APICall
	for rows.Next() {
		var call APICall
		var timestamp string
		err := rows.Scan(
			&call.RunID,
			&call.OrderID,
			&call.Method,
			&call.RequestJSON,
			&call.ResponseJSON,
			&call.Error,
			&call.DurationMs,
			&timestamp,
		)
		if err != nil {
			return nil, err
		}
		calls = append(calls, call)
	}

	return calls, rows.Err()
}
