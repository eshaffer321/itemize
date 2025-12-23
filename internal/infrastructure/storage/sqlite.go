package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

// Storage provides SQLite database access for processing records.
// It implements the Repository interface.
type Storage struct {
	db *sql.DB
}

// Compile-time check that Storage implements Repository
var _ Repository = (*Storage)(nil)

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
