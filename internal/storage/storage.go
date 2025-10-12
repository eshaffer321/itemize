package storage

import (
	"database/sql"
	"encoding/json"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Storage handles persistence for processing history
type Storage struct {
	db *sql.DB
}

// ProcessingRecord represents a processed order
type ProcessingRecord struct {
	ID             int64     `json:"id"`
	OrderID        string    `json:"order_id"`
	TransactionID  string    `json:"transaction_id"`
	OrderDate      time.Time `json:"order_date"`
	ProcessedAt    time.Time `json:"processed_at"`
	OrderAmount    float64   `json:"order_amount"`
	SplitCount     int       `json:"split_count"`
	Categories     []string  `json:"categories"`
	CategoriesJSON string    `json:"-"` // For DB storage
	Status         string    `json:"status"` // success, failed, skipped, dry-run
	ErrorMessage   string    `json:"error_message,omitempty"`
	ItemCount      int       `json:"item_count"`
	Notes          string    `json:"notes,omitempty"`
	ItemDetails    string    `json:"item_details,omitempty"` // JSON of items with prices
	MatchConfidence float64  `json:"match_confidence"`
}

// Stats represents processing statistics
type Stats struct {
	TotalProcessed   int     `json:"total_processed"`
	TotalSplits      int     `json:"total_splits"`
	TotalAmount      float64 `json:"total_amount"`
	SuccessRate      float64 `json:"success_rate"`
	LastProcessedAt  *time.Time `json:"last_processed_at"`
	MostUsedCategory string  `json:"most_used_category"`
	AverageSplits    float64 `json:"average_splits"`
}

// NewStorage creates a new storage instance
func NewStorage(dbPath string) (*Storage, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	s := &Storage{db: db}
	if err := s.createTables(); err != nil {
		return nil, err
	}

	return s, nil
}

// createTables creates the necessary database tables
func (s *Storage) createTables() error {
	schema := `
	CREATE TABLE IF NOT EXISTS processing_records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		order_id TEXT UNIQUE NOT NULL,
		transaction_id TEXT,
		order_date DATETIME,
		processed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		order_amount REAL,
		split_count INTEGER DEFAULT 0,
		categories TEXT, -- JSON array
		status TEXT DEFAULT 'pending',
		error_message TEXT,
		item_count INTEGER DEFAULT 0,
		notes TEXT,
		item_details TEXT, -- JSON of items
		match_confidence REAL DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS sync_runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		completed_at DATETIME,
		total_orders INTEGER DEFAULT 0,
		processed INTEGER DEFAULT 0,
		skipped INTEGER DEFAULT 0,
		failed INTEGER DEFAULT 0,
		dry_run BOOLEAN DEFAULT 0,
		lookback_days INTEGER DEFAULT 14
	);

	CREATE INDEX IF NOT EXISTS idx_order_date ON processing_records(order_date);
	CREATE INDEX IF NOT EXISTS idx_processed_at ON processing_records(processed_at);
	CREATE INDEX IF NOT EXISTS idx_status ON processing_records(status);
	`

	_, err := s.db.Exec(schema)
	return err
}

// SaveRecord saves a processing record
func (s *Storage) SaveRecord(record *ProcessingRecord) error {
	categoriesJSON, _ := json.Marshal(record.Categories)
	
	query := `
	INSERT OR REPLACE INTO processing_records 
	(order_id, transaction_id, order_date, processed_at, order_amount, 
	 split_count, categories, status, error_message, item_count, notes)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query,
		record.OrderID,
		record.TransactionID,
		record.OrderDate,
		record.ProcessedAt,
		record.OrderAmount,
		record.SplitCount,
		string(categoriesJSON),
		record.Status,
		record.ErrorMessage,
		record.ItemCount,
		record.Notes,
	)

	return err
}

// GetRecord retrieves a single record by order ID
func (s *Storage) GetRecord(orderID string) (*ProcessingRecord, error) {
	query := `
	SELECT id, order_id, transaction_id, order_date, processed_at, 
	       order_amount, split_count, categories, status, error_message, 
	       item_count, notes
	FROM processing_records WHERE order_id = ?
	`

	record := &ProcessingRecord{}
	err := s.db.QueryRow(query, orderID).Scan(
		&record.ID,
		&record.OrderID,
		&record.TransactionID,
		&record.OrderDate,
		&record.ProcessedAt,
		&record.OrderAmount,
		&record.SplitCount,
		&record.CategoriesJSON,
		&record.Status,
		&record.ErrorMessage,
		&record.ItemCount,
		&record.Notes,
	)

	if err == nil && record.CategoriesJSON != "" {
		json.Unmarshal([]byte(record.CategoriesJSON), &record.Categories)
	}

	return record, err
}

// GetRecentRecords retrieves recent processing records
func (s *Storage) GetRecentRecords(limit int) ([]*ProcessingRecord, error) {
	query := `
	SELECT id, order_id, transaction_id, order_date, processed_at, 
	       order_amount, split_count, categories, status, error_message,
	       item_count, notes
	FROM processing_records 
	ORDER BY processed_at DESC 
	LIMIT ?
	`

	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*ProcessingRecord
	for rows.Next() {
		record := &ProcessingRecord{}
		err := rows.Scan(
			&record.ID,
			&record.OrderID,
			&record.TransactionID,
			&record.OrderDate,
			&record.ProcessedAt,
			&record.OrderAmount,
			&record.SplitCount,
			&record.CategoriesJSON,
			&record.Status,
			&record.ErrorMessage,
			&record.ItemCount,
			&record.Notes,
		)
		if err != nil {
			continue
		}

		if record.CategoriesJSON != "" {
			json.Unmarshal([]byte(record.CategoriesJSON), &record.Categories)
		}

		records = append(records, record)
	}

	return records, nil
}

// GetStats retrieves processing statistics
func (s *Storage) GetStats() (*Stats, error) {
	stats := &Stats{}

	// Get basic counts and sums
	query := `
	SELECT 
		COUNT(*) as total,
		COUNT(CASE WHEN status = 'success' THEN 1 END) as success_count,
		COALESCE(SUM(split_count), 0) as total_splits,
		COALESCE(SUM(order_amount), 0) as total_amount,
		COALESCE(AVG(split_count), 0) as avg_splits,
		MAX(processed_at) as last_processed
	FROM processing_records
	`

	var successCount int
	var lastProcessed sql.NullTime
	
	err := s.db.QueryRow(query).Scan(
		&stats.TotalProcessed,
		&successCount,
		&stats.TotalSplits,
		&stats.TotalAmount,
		&stats.AverageSplits,
		&lastProcessed,
	)
	
	if err != nil {
		return stats, err
	}

	if lastProcessed.Valid {
		stats.LastProcessedAt = &lastProcessed.Time
	}

	if stats.TotalProcessed > 0 {
		stats.SuccessRate = float64(successCount) / float64(stats.TotalProcessed) * 100
	}

	// Get most used category
	categoryQuery := `
	SELECT category, COUNT(*) as count
	FROM (
		SELECT json_each.value as category
		FROM processing_records, json_each(categories)
		WHERE status = 'success'
	)
	GROUP BY category
	ORDER BY count DESC
	LIMIT 1
	`

	var category sql.NullString
	var count int
	err = s.db.QueryRow(categoryQuery).Scan(&category, &count)
	if err == nil && category.Valid {
		stats.MostUsedCategory = category.String
	}

	return stats, nil
}

// GetCategoryBreakdown gets category usage statistics
func (s *Storage) GetCategoryBreakdown() (map[string]int, error) {
	query := `
	SELECT category, COUNT(*) as count
	FROM (
		SELECT json_each.value as category
		FROM processing_records, json_each(categories)
		WHERE status = 'success'
	)
	GROUP BY category
	ORDER BY count DESC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	breakdown := make(map[string]int)
	for rows.Next() {
		var category string
		var count int
		if err := rows.Scan(&category, &count); err == nil {
			breakdown[category] = count
		}
	}

	return breakdown, nil
}

// IsProcessed checks if an order has been processed
func (s *Storage) IsProcessed(orderID string) bool {
	query := `SELECT 1 FROM processing_records WHERE order_id = ? AND status = 'success'`
	var exists int
	err := s.db.QueryRow(query, orderID).Scan(&exists)
	return err == nil
}

// SyncRun represents a sync run session
type SyncRun struct {
	ID           int64      `json:"id"`
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at"`
	TotalOrders  int        `json:"total_orders"`
	Processed    int        `json:"processed"`
	Skipped      int        `json:"skipped"`
	Failed       int        `json:"failed"`
	DryRun       bool       `json:"dry_run"`
	LookbackDays int        `json:"lookback_days"`
}

// StartSyncRun creates a new sync run record
func (s *Storage) StartSyncRun(totalOrders int, dryRun bool, lookbackDays int) (int64, error) {
	query := `
	INSERT INTO sync_runs (total_orders, dry_run, lookback_days)
	VALUES (?, ?, ?)
	`
	result, err := s.db.Exec(query, totalOrders, dryRun, lookbackDays)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// CompleteSyncRun updates the sync run with final statistics
func (s *Storage) CompleteSyncRun(runID int64, processed, skipped, failed int) error {
	query := `
	UPDATE sync_runs 
	SET completed_at = CURRENT_TIMESTAMP,
	    processed = ?,
	    skipped = ?,
	    failed = ?
	WHERE id = ?
	`
	_, err := s.db.Exec(query, processed, skipped, failed, runID)
	return err
}

// GetLastSyncRun retrieves the most recent sync run
func (s *Storage) GetLastSyncRun() (*SyncRun, error) {
	query := `
	SELECT id, started_at, completed_at, total_orders, processed, 
	       skipped, failed, dry_run, lookback_days
	FROM sync_runs
	ORDER BY started_at DESC
	LIMIT 1
	`
	run := &SyncRun{}
	var completedAt sql.NullTime
	err := s.db.QueryRow(query).Scan(
		&run.ID,
		&run.StartedAt,
		&completedAt,
		&run.TotalOrders,
		&run.Processed,
		&run.Skipped,
		&run.Failed,
		&run.DryRun,
		&run.LookbackDays,
	)
	if err != nil {
		return nil, err
	}
	if completedAt.Valid {
		run.CompletedAt = &completedAt.Time
	}
	return run, nil
}

// GetRecentSyncRuns gets the last N sync runs
func (s *Storage) GetRecentSyncRuns(limit int) ([]*SyncRun, error) {
	query := `
	SELECT id, started_at, completed_at, total_orders, processed, 
	       skipped, failed, dry_run, lookback_days
	FROM sync_runs
	ORDER BY started_at DESC
	LIMIT ?
	`
	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []*SyncRun
	for rows.Next() {
		run := &SyncRun{}
		var completedAt sql.NullTime
		err := rows.Scan(
			&run.ID,
			&run.StartedAt,
			&completedAt,
			&run.TotalOrders,
			&run.Processed,
			&run.Skipped,
			&run.Failed,
			&run.DryRun,
			&run.LookbackDays,
		)
		if err != nil {
			continue
		}
		if completedAt.Valid {
			run.CompletedAt = &completedAt.Time
		}
		runs = append(runs, run)
	}
	return runs, nil
}

// Close closes the database connection
func (s *Storage) Close() error {
	return s.db.Close()
}