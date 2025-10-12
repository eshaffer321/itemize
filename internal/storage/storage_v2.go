package storage

import (
	"encoding/json"
	"time"
)

// ProcessingRecordV2 includes detailed split information
type ProcessingRecordV2 struct {
	ID               int64           `json:"id"`
	OrderID          string          `json:"order_id"`
	Provider         string          `json:"provider"`
	TransactionID    string          `json:"transaction_id"`
	OrderDate        time.Time       `json:"order_date"`
	ProcessedAt      time.Time       `json:"processed_at"`
	OrderTotal       float64         `json:"order_total"`
	OrderSubtotal    float64         `json:"order_subtotal"`
	OrderTax         float64         `json:"order_tax"`
	OrderTip         float64         `json:"order_tip"`
	TransactionAmount float64        `json:"transaction_amount"`
	SplitCount       int             `json:"split_count"`
	Status           string          `json:"status"`
	ErrorMessage     string          `json:"error_message,omitempty"`
	ItemCount        int             `json:"item_count"`
	MatchConfidence  float64         `json:"match_confidence"`
	DryRun           bool            `json:"dry_run"`
	
	// Detailed data stored as JSON
	Items            []OrderItem     `json:"items"`
	Splits           []SplitDetail   `json:"splits"`
	ItemsJSON        string          `json:"-"` // For DB storage
	SplitsJSON       string          `json:"-"` // For DB storage
}

// OrderItem represents an item in the order
type OrderItem struct {
	Name        string  `json:"name"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	TotalPrice  float64 `json:"total_price"`
	Category    string  `json:"category,omitempty"`
}

// SplitDetail represents how the transaction was split
type SplitDetail struct {
	CategoryID   string      `json:"category_id"`
	CategoryName string      `json:"category_name"`
	Amount       float64     `json:"amount"`
	Items        []OrderItem `json:"items"`
	Notes        string      `json:"notes"`
}

// CreateTablesV2 creates enhanced tables with more detail
func (s *Storage) CreateTablesV2() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS processing_records_v2 (
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
		`CREATE INDEX IF NOT EXISTS idx_order_id_v2 ON processing_records_v2(order_id)`,
		`CREATE INDEX IF NOT EXISTS idx_processed_at_v2 ON processing_records_v2(processed_at)`,
		`CREATE INDEX IF NOT EXISTS idx_status_v2 ON processing_records_v2(status)`,
		`CREATE INDEX IF NOT EXISTS idx_provider_v2 ON processing_records_v2(provider)`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return err
		}
	}

	return nil
}

// SaveRecordV2 saves an enhanced processing record
func (s *Storage) SaveRecordV2(record *ProcessingRecordV2) error {
	itemsJSON, _ := json.Marshal(record.Items)
	splitsJSON, _ := json.Marshal(record.Splits)
	
	query := `
	INSERT OR REPLACE INTO processing_records_v2 
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

// GetRecordV2 retrieves an enhanced record by order ID
func (s *Storage) GetRecordV2(orderID string) (*ProcessingRecordV2, error) {
	query := `
	SELECT id, order_id, provider, transaction_id, order_date, processed_at, 
	       order_total, order_subtotal, order_tax, order_tip, transaction_amount,
	       split_count, status, error_message, item_count, match_confidence,
	       dry_run, items_json, splits_json
	FROM processing_records_v2 WHERE order_id = ?
	`

	record := &ProcessingRecordV2{}
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

// GetRecentRecordsV2 retrieves recent enhanced records
func (s *Storage) GetRecentRecordsV2(limit int) ([]*ProcessingRecordV2, error) {
	query := `
	SELECT id, order_id, provider, transaction_id, order_date, processed_at, 
	       order_total, order_subtotal, order_tax, order_tip, transaction_amount,
	       split_count, status, error_message, item_count, match_confidence,
	       dry_run, items_json, splits_json
	FROM processing_records_v2 
	ORDER BY processed_at DESC
	LIMIT ?
	`

	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*ProcessingRecordV2
	for rows.Next() {
		record := &ProcessingRecordV2{}
		err := rows.Scan(
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
			continue
		}

		// Unmarshal JSON fields
		if record.ItemsJSON != "" {
			json.Unmarshal([]byte(record.ItemsJSON), &record.Items)
		}
		if record.SplitsJSON != "" {
			json.Unmarshal([]byte(record.SplitsJSON), &record.Splits)
		}

		records = append(records, record)
	}

	return records, nil
}

// GetStatsV2 returns enhanced statistics
func (s *Storage) GetStatsV2() (*StatsV2, error) {
	stats := &StatsV2{
		CategoryBreakdown: make(map[string]CategoryStats),
		ProviderStats:     make(map[string]ProviderStats),
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
	FROM processing_records_v2
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
	FROM processing_records_v2
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

// StatsV2 contains enhanced statistics
type StatsV2 struct {
	TotalProcessed     int                       `json:"total_processed"`
	SuccessCount       int                       `json:"success_count"`
	FailedCount        int                       `json:"failed_count"`
	SkippedCount       int                       `json:"skipped_count"`
	DryRunCount        int                       `json:"dry_run_count"`
	TotalAmount        float64                   `json:"total_amount"`
	AverageOrderAmount float64                   `json:"average_order_amount"`
	TotalSplits        int                       `json:"total_splits"`
	CategoryBreakdown  map[string]CategoryStats  `json:"category_breakdown"`
	ProviderStats      map[string]ProviderStats  `json:"provider_stats"`
}

// CategoryStats contains per-category statistics
type CategoryStats struct {
	Count       int     `json:"count"`
	TotalAmount float64 `json:"total_amount"`
}

// ProviderStats contains per-provider statistics
type ProviderStats struct {
	Count        int     `json:"count"`
	SuccessCount int     `json:"success_count"`
	TotalAmount  float64 `json:"total_amount"`
}

// MigrateFromV1 migrates data from the old schema to the new one
func (s *Storage) MigrateFromV1() error {
	// Check if we have any v1 records
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM processing_records").Scan(&count)
	if err != nil || count == 0 {
		return nil // Nothing to migrate
	}

	// Create v2 tables
	if err := s.CreateTablesV2(); err != nil {
		return err
	}

	// Migrate records
	query := `
	INSERT INTO processing_records_v2 
	(order_id, provider, transaction_id, order_date, processed_at,
	 order_total, split_count, status, error_message, item_count,
	 match_confidence, items_json)
	SELECT 
		order_id, 'walmart', transaction_id, order_date, processed_at,
		order_amount, split_count, status, error_message, item_count,
		match_confidence, item_details
	FROM processing_records
	`

	_, err = s.db.Exec(query)
	return err
}