package storage

import (
	"database/sql"
	"encoding/json"
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

	s := &Storage{db: db}

	// Check if we need to migrate from old schema
	needsMigration := false
	var col string
	err = db.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name='processing_records'").Scan(&col)
	if err == nil {
		// Table exists - check if it has the 'provider' column
		_, err = db.Exec("SELECT provider FROM processing_records LIMIT 1")
		needsMigration = (err != nil)
	}

	if needsMigration {
		if err := s.migrateFromV1(); err != nil {
			return nil, err
		}
	} else {
		if err := s.CreateTables(); err != nil {
			return nil, err
		}
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

// CreateTables creates enhanced tables with more detail
func (s *Storage) CreateTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS processing_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			order_id TEXT UNIQUE NOT NULL,
			provider TEXT DEFAULT 'walmart',
			transaction_id TEXT,
			order_date DATETIME,
			processed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			order_total REAL,
			order_subtotal REAL,
			order_tax REAL,
			order_tip REAL,
			transaction_amount REAL,
			split_count INTEGER DEFAULT 0,
			status TEXT,
			error_message TEXT,
			item_count INTEGER DEFAULT 0,
			match_confidence REAL DEFAULT 0,
			dry_run BOOLEAN DEFAULT 0,
			items_json TEXT,
			splits_json TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_order_id ON processing_records(order_id)`,
		`CREATE INDEX IF NOT EXISTS idx_processed_at ON processing_records(processed_at)`,
		`CREATE INDEX IF NOT EXISTS idx_status ON processing_records(status)`,
		`CREATE INDEX IF NOT EXISTS idx_provider ON processing_records(provider)`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return err
		}
	}

	return nil
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
	 dry_run, items_json, splits_json)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
	)

	return err
}

// GetRecord retrieves an enhanced record by order ID
func (s *Storage) GetRecord(orderID string) (*ProcessingRecord, error) {
	query := `
	SELECT id, order_id, provider, transaction_id, order_date, processed_at, 
	       order_total, order_subtotal, order_tax, order_tip, transaction_amount,
	       split_count, status, error_message, item_count, match_confidence,
	       dry_run, items_json, splits_json
	FROM processing_records WHERE order_id = ?
	`

	record := &ProcessingRecord{}
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
	)

	if err != nil {
		return nil, err
	}

	// Unmarshal JSON fields
	if record.ItemsJSON != "" {
		json.Unmarshal([]byte(record.ItemsJSON), &record.Items)
	}
	if record.SplitsJSON != "" {
		json.Unmarshal([]byte(record.SplitsJSON), &record.Splits)
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
		defer rows.Close()
		for rows.Next() {
			var provider string
			var ps ProviderStats
			rows.Scan(&provider, &ps.Count, &ps.TotalAmount, &ps.SuccessCount)
			stats.ProviderStats[provider] = ps
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

// migrateFromV1 migrates data from the old schema to the new one
func (s *Storage) migrateFromV1() error {
	// Rename old table
	_, err := s.db.Exec("ALTER TABLE processing_records RENAME TO processing_records_old")
	if err != nil {
		return err
	}

	// Create new table with v2 schema
	if err := s.CreateTables(); err != nil {
		return err
	}

	// Migrate records from old table to new table (skip duplicates)
	query := `
	INSERT OR IGNORE INTO processing_records
	(order_id, provider, transaction_id, order_date, processed_at,
	 order_total, split_count, status, error_message, item_count,
	 match_confidence, items_json)
	SELECT
		order_id, 'walmart', transaction_id, order_date, processed_at,
		order_amount, split_count, status, COALESCE(error_message, ''), item_count,
		COALESCE(match_confidence, 0), COALESCE(item_details, '[]')
	FROM processing_records_old
	`

	_, err = s.db.Exec(query)
	if err != nil {
		return err
	}

	// Drop old table
	_, err = s.db.Exec("DROP TABLE processing_records_old")
	return err
}

// IsProcessed checks if an order has already been successfully processed (non-dry-run)
func (s *Storage) IsProcessed(orderID string) bool {
	var count int
	query := `SELECT COUNT(*) FROM processing_records WHERE order_id = ? AND dry_run = 0 AND status = 'success'`
	err := s.db.QueryRow(query, orderID).Scan(&count)
	return err == nil && count > 0
}

// StartSyncRun records the start of a sync run (optional feature)
func (s *Storage) StartSyncRun(orderCount int, dryRun bool, lookbackDays int) (int64, error) {
	// This is a placeholder - sync runs tracking is optional
	// For now, just return a fake ID
	return 1, nil
}

// CompleteSyncRun records the completion of a sync run (optional feature)
func (s *Storage) CompleteSyncRun(runID int64, processed, skipped, errors int) error {
	// This is a placeholder - sync runs tracking is optional
	return nil
}
