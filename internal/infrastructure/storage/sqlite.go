package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

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

// DB returns the underlying database connection for advanced queries.
// Use with caution - prefer using the Repository interface methods.
func (s *Storage) DB() *sql.DB {
	return s.db
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
	var (
		transactionID     sql.NullString
		errorMessage      sql.NullString
		itemsJSON         sql.NullString
		splitsJSON        sql.NullString
		multiDeliveryData sql.NullString
	)
	err := s.db.QueryRow(query, orderID).Scan(
		&record.ID,
		&record.OrderID,
		&record.Provider,
		&transactionID,
		&record.OrderDate,
		&record.ProcessedAt,
		&record.OrderTotal,
		&record.OrderSubtotal,
		&record.OrderTax,
		&record.OrderTip,
		&record.TransactionAmount,
		&record.SplitCount,
		&record.Status,
		&errorMessage,
		&record.ItemCount,
		&record.MatchConfidence,
		&record.DryRun,
		&itemsJSON,
		&splitsJSON,
		&multiDeliveryData,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	// Handle nullable string fields
	if transactionID.Valid {
		record.TransactionID = transactionID.String
	}
	if errorMessage.Valid {
		record.ErrorMessage = errorMessage.String
	}
	if itemsJSON.Valid {
		record.ItemsJSON = itemsJSON.String
	}
	if splitsJSON.Valid {
		record.SplitsJSON = splitsJSON.String
	}
	if multiDeliveryData.Valid {
		record.MultiDeliveryData = multiDeliveryData.String
	}

	// Unmarshal JSON fields (errors ignored as these are optional enrichment fields)
	if record.ItemsJSON != "" {
		_ = json.Unmarshal([]byte(record.ItemsJSON), &record.Items)
	}
	if record.SplitsJSON != "" {
		_ = json.Unmarshal([]byte(record.SplitsJSON), &record.Splits)
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

// ListOrders returns orders matching the given filters with pagination
func (s *Storage) ListOrders(filters OrderFilters) (*OrderListResult, error) {
	// Set defaults
	if filters.Limit <= 0 {
		filters.Limit = 50
	}
	if filters.Limit > 500 {
		filters.Limit = 500
	}
	if filters.OrderBy == "" {
		filters.OrderBy = "processed_at"
	}

	// Build WHERE clause
	where := "WHERE 1=1"
	args := []interface{}{}

	if filters.Provider != "" {
		where += " AND provider = ?"
		args = append(args, filters.Provider)
	}
	if filters.Status != "" {
		where += " AND status = ?"
		args = append(args, filters.Status)
	}
	if filters.DaysBack > 0 {
		where += " AND order_date > datetime('now', ?)"
		args = append(args, fmt.Sprintf("-%d days", filters.DaysBack))
	}
	if filters.Search != "" {
		where += " AND order_id LIKE ?"
		args = append(args, "%"+filters.Search+"%")
	}

	// Validate and set ORDER BY
	orderBy := "processed_at"
	switch filters.OrderBy {
	case "date":
		orderBy = "order_date"
	case "total":
		orderBy = "order_total"
	case "processed_at":
		orderBy = "processed_at"
	}
	direction := "DESC"
	if !filters.OrderDesc {
		direction = "ASC"
	}

	// Get total count
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM processing_records %s", where)
	var totalCount int
	if err := s.db.QueryRow(countQuery, args...).Scan(&totalCount); err != nil {
		return nil, err
	}

	// Get paginated results
	query := fmt.Sprintf(`
		SELECT id, order_id, provider, transaction_id, order_date, processed_at,
		       order_total, order_subtotal, order_tax, order_tip, transaction_amount,
		       split_count, status, error_message, item_count, match_confidence,
		       dry_run, items_json, splits_json, multi_delivery_data
		FROM processing_records
		%s
		ORDER BY %s %s
		LIMIT ? OFFSET ?
	`, where, orderBy, direction)

	args = append(args, filters.Limit, filters.Offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var orders []*ProcessingRecord
	for rows.Next() {
		record := &ProcessingRecord{}
		var (
			transactionID     sql.NullString
			errorMessage      sql.NullString
			itemsJSON         sql.NullString
			splitsJSON        sql.NullString
			multiDeliveryData sql.NullString
		)
		err := rows.Scan(
			&record.ID,
			&record.OrderID,
			&record.Provider,
			&transactionID,
			&record.OrderDate,
			&record.ProcessedAt,
			&record.OrderTotal,
			&record.OrderSubtotal,
			&record.OrderTax,
			&record.OrderTip,
			&record.TransactionAmount,
			&record.SplitCount,
			&record.Status,
			&errorMessage,
			&record.ItemCount,
			&record.MatchConfidence,
			&record.DryRun,
			&itemsJSON,
			&splitsJSON,
			&multiDeliveryData,
		)
		if err != nil {
			return nil, err
		}

		// Handle nullable string fields
		if transactionID.Valid {
			record.TransactionID = transactionID.String
		}
		if errorMessage.Valid {
			record.ErrorMessage = errorMessage.String
		}
		if itemsJSON.Valid {
			record.ItemsJSON = itemsJSON.String
		}
		if splitsJSON.Valid {
			record.SplitsJSON = splitsJSON.String
		}
		if multiDeliveryData.Valid {
			record.MultiDeliveryData = multiDeliveryData.String
		}

		// Unmarshal JSON fields
		if record.ItemsJSON != "" {
			_ = json.Unmarshal([]byte(record.ItemsJSON), &record.Items)
		}
		if record.SplitsJSON != "" {
			_ = json.Unmarshal([]byte(record.SplitsJSON), &record.Splits)
		}

		orders = append(orders, record)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &OrderListResult{
		Orders:     orders,
		TotalCount: totalCount,
		Limit:      filters.Limit,
		Offset:     filters.Offset,
	}, nil
}

// SearchItems searches for items across all orders using SQLite JSON functions
func (s *Storage) SearchItems(query string, limit int) ([]ItemSearchResult, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	// Use SQLite's json_each to expand items array and search
	sqlQuery := `
		SELECT
			p.order_id,
			p.provider,
			date(p.order_date) as order_date,
			json_extract(item.value, '$.name') as item_name,
			json_extract(item.value, '$.total_price') as item_price,
			json_extract(item.value, '$.category') as category
		FROM processing_records p, json_each(p.items_json) as item
		WHERE p.items_json IS NOT NULL
		  AND p.items_json != 'null'
		  AND json_extract(item.value, '$.name') LIKE ?
		ORDER BY p.order_date DESC
		LIMIT ?
	`

	searchPattern := "%" + query + "%"
	rows, err := s.db.Query(sqlQuery, searchPattern, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []ItemSearchResult
	for rows.Next() {
		var r ItemSearchResult
		var category sql.NullString
		err := rows.Scan(
			&r.OrderID,
			&r.Provider,
			&r.OrderDate,
			&r.ItemName,
			&r.ItemPrice,
			&category,
		)
		if err != nil {
			return nil, err
		}
		if category.Valid {
			r.Category = category.String
		}
		results = append(results, r)
	}

	return results, rows.Err()
}

// ListSyncRuns returns recent sync runs
func (s *Storage) ListSyncRuns(limit int) ([]SyncRun, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	query := `
		SELECT id, provider, started_at, completed_at, lookback_days, dry_run,
		       orders_found, orders_processed, orders_skipped, orders_errored, status
		FROM sync_runs
		ORDER BY started_at DESC
		LIMIT ?
	`

	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var runs []SyncRun
	for rows.Next() {
		var r SyncRun
		var completedAt sql.NullString
		err := rows.Scan(
			&r.ID,
			&r.Provider,
			&r.StartedAt,
			&completedAt,
			&r.LookbackDays,
			&r.DryRun,
			&r.OrdersFound,
			&r.OrdersProcessed,
			&r.OrdersSkipped,
			&r.OrdersErrored,
			&r.Status,
		)
		if err != nil {
			return nil, err
		}
		if completedAt.Valid {
			r.CompletedAt = completedAt.String
		}
		runs = append(runs, r)
	}

	return runs, rows.Err()
}

// GetSyncRun retrieves a sync run by ID
func (s *Storage) GetSyncRun(runID int64) (*SyncRun, error) {
	query := `
		SELECT id, provider, started_at, completed_at, lookback_days, dry_run,
		       orders_found, orders_processed, orders_skipped, orders_errored, status
		FROM sync_runs
		WHERE id = ?
	`

	var r SyncRun
	var completedAt sql.NullString
	err := s.db.QueryRow(query, runID).Scan(
		&r.ID,
		&r.Provider,
		&r.StartedAt,
		&completedAt,
		&r.LookbackDays,
		&r.DryRun,
		&r.OrdersFound,
		&r.OrdersProcessed,
		&r.OrdersSkipped,
		&r.OrdersErrored,
		&r.Status,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if completedAt.Valid {
		r.CompletedAt = completedAt.String
	}

	return &r, nil
}

// ================================================================
// LEDGER REPOSITORY IMPLEMENTATION
// ================================================================

// SaveLedger saves a ledger snapshot with its charges in a transaction
func (s *Storage) SaveLedger(ledger *OrderLedger) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Determine the next ledger version for this order
	var currentVersion int
	err = tx.QueryRow(`
		SELECT COALESCE(MAX(ledger_version), 0)
		FROM order_ledgers
		WHERE order_id = ? AND provider = ?
	`, ledger.OrderID, ledger.Provider).Scan(&currentVersion)
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}
	ledger.LedgerVersion = currentVersion + 1

	// Insert the ledger
	result, err := tx.Exec(`
		INSERT INTO order_ledgers
		(order_id, sync_run_id, provider, ledger_state, ledger_version,
		 ledger_json, total_charged, charge_count, payment_method_types,
		 has_refunds, is_valid, validation_notes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		ledger.OrderID,
		nullInt64(ledger.SyncRunID),
		ledger.Provider,
		ledger.LedgerState,
		ledger.LedgerVersion,
		ledger.LedgerJSON,
		ledger.TotalCharged,
		ledger.ChargeCount,
		ledger.PaymentMethodTypes,
		ledger.HasRefunds,
		ledger.IsValid,
		ledger.ValidationNotes,
	)
	if err != nil {
		return fmt.Errorf("failed to insert ledger: %w", err)
	}

	ledgerID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get ledger ID: %w", err)
	}
	ledger.ID = ledgerID

	// Insert charges
	for i := range ledger.Charges {
		charge := &ledger.Charges[i]
		charge.OrderLedgerID = ledgerID
		charge.OrderID = ledger.OrderID
		charge.SyncRunID = ledger.SyncRunID

		result, err := tx.Exec(`
			INSERT INTO ledger_charges
			(order_ledger_id, order_id, sync_run_id, charge_sequence,
			 charge_amount, charge_type, payment_method, card_type, card_last_four,
			 charged_at, monarch_transaction_id, is_matched, match_confidence, matched_at, split_count)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			charge.OrderLedgerID,
			charge.OrderID,
			nullInt64(charge.SyncRunID),
			charge.ChargeSequence,
			charge.ChargeAmount,
			charge.ChargeType,
			charge.PaymentMethod,
			charge.CardType,
			charge.CardLastFour,
			nullTime(charge.ChargedAt),
			nullString(charge.MonarchTransactionID),
			charge.IsMatched,
			charge.MatchConfidence,
			nullTime(charge.MatchedAt),
			charge.SplitCount,
		)
		if err != nil {
			return fmt.Errorf("failed to insert charge: %w", err)
		}

		chargeID, _ := result.LastInsertId()
		charge.ID = chargeID
	}

	return tx.Commit()
}

// GetLatestLedger retrieves the most recent ledger for an order
func (s *Storage) GetLatestLedger(orderID string) (*OrderLedger, error) {
	query := `
		SELECT id, order_id, sync_run_id, provider, fetched_at, ledger_state,
		       ledger_version, ledger_json, total_charged, charge_count,
		       payment_method_types, has_refunds, is_valid, validation_notes
		FROM order_ledgers
		WHERE order_id = ?
		ORDER BY ledger_version DESC
		LIMIT 1
	`

	ledger := &OrderLedger{}
	var syncRunID sql.NullInt64
	var validationNotes sql.NullString

	err := s.db.QueryRow(query, orderID).Scan(
		&ledger.ID,
		&ledger.OrderID,
		&syncRunID,
		&ledger.Provider,
		&ledger.FetchedAt,
		&ledger.LedgerState,
		&ledger.LedgerVersion,
		&ledger.LedgerJSON,
		&ledger.TotalCharged,
		&ledger.ChargeCount,
		&ledger.PaymentMethodTypes,
		&ledger.HasRefunds,
		&ledger.IsValid,
		&validationNotes,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if syncRunID.Valid {
		ledger.SyncRunID = syncRunID.Int64
	}
	if validationNotes.Valid {
		ledger.ValidationNotes = validationNotes.String
	}

	// Load charges
	charges, err := s.getChargesForLedger(ledger.ID)
	if err != nil {
		return nil, err
	}
	ledger.Charges = charges

	return ledger, nil
}

// GetLedgerHistory retrieves all ledger snapshots for an order (newest first)
func (s *Storage) GetLedgerHistory(orderID string) ([]*OrderLedger, error) {
	query := `
		SELECT id, order_id, sync_run_id, provider, fetched_at, ledger_state,
		       ledger_version, ledger_json, total_charged, charge_count,
		       payment_method_types, has_refunds, is_valid, validation_notes
		FROM order_ledgers
		WHERE order_id = ?
		ORDER BY ledger_version DESC
	`

	rows, err := s.db.Query(query, orderID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var ledgers []*OrderLedger
	for rows.Next() {
		ledger := &OrderLedger{}
		var syncRunID sql.NullInt64
		var validationNotes sql.NullString

		err := rows.Scan(
			&ledger.ID,
			&ledger.OrderID,
			&syncRunID,
			&ledger.Provider,
			&ledger.FetchedAt,
			&ledger.LedgerState,
			&ledger.LedgerVersion,
			&ledger.LedgerJSON,
			&ledger.TotalCharged,
			&ledger.ChargeCount,
			&ledger.PaymentMethodTypes,
			&ledger.HasRefunds,
			&ledger.IsValid,
			&validationNotes,
		)
		if err != nil {
			return nil, err
		}

		if syncRunID.Valid {
			ledger.SyncRunID = syncRunID.Int64
		}
		if validationNotes.Valid {
			ledger.ValidationNotes = validationNotes.String
		}

		// Load charges for each ledger
		charges, err := s.getChargesForLedger(ledger.ID)
		if err != nil {
			return nil, err
		}
		ledger.Charges = charges

		ledgers = append(ledgers, ledger)
	}

	return ledgers, rows.Err()
}

// GetLedgerByID retrieves a specific ledger by ID
func (s *Storage) GetLedgerByID(id int64) (*OrderLedger, error) {
	query := `
		SELECT id, order_id, sync_run_id, provider, fetched_at, ledger_state,
		       ledger_version, ledger_json, total_charged, charge_count,
		       payment_method_types, has_refunds, is_valid, validation_notes
		FROM order_ledgers
		WHERE id = ?
	`

	ledger := &OrderLedger{}
	var syncRunID sql.NullInt64
	var validationNotes sql.NullString

	err := s.db.QueryRow(query, id).Scan(
		&ledger.ID,
		&ledger.OrderID,
		&syncRunID,
		&ledger.Provider,
		&ledger.FetchedAt,
		&ledger.LedgerState,
		&ledger.LedgerVersion,
		&ledger.LedgerJSON,
		&ledger.TotalCharged,
		&ledger.ChargeCount,
		&ledger.PaymentMethodTypes,
		&ledger.HasRefunds,
		&ledger.IsValid,
		&validationNotes,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if syncRunID.Valid {
		ledger.SyncRunID = syncRunID.Int64
	}
	if validationNotes.Valid {
		ledger.ValidationNotes = validationNotes.String
	}

	// Load charges
	charges, err := s.getChargesForLedger(ledger.ID)
	if err != nil {
		return nil, err
	}
	ledger.Charges = charges

	return ledger, nil
}

// ListLedgers returns ledgers matching the given filters with pagination
func (s *Storage) ListLedgers(filters LedgerFilters) (*LedgerListResult, error) {
	// Set defaults
	if filters.Limit <= 0 {
		filters.Limit = 50
	}
	if filters.Limit > 500 {
		filters.Limit = 500
	}

	// Build WHERE clause
	where := "WHERE 1=1"
	args := []interface{}{}

	if filters.OrderID != "" {
		where += " AND order_id = ?"
		args = append(args, filters.OrderID)
	}
	if filters.Provider != "" {
		where += " AND provider = ?"
		args = append(args, filters.Provider)
	}
	if filters.State != "" {
		where += " AND ledger_state = ?"
		args = append(args, filters.State)
	}

	// Get total count
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM order_ledgers %s", where)
	var totalCount int
	if err := s.db.QueryRow(countQuery, args...).Scan(&totalCount); err != nil {
		return nil, err
	}

	// Get paginated results
	query := fmt.Sprintf(`
		SELECT id, order_id, sync_run_id, provider, fetched_at, ledger_state,
		       ledger_version, ledger_json, total_charged, charge_count,
		       payment_method_types, has_refunds, is_valid, validation_notes
		FROM order_ledgers
		%s
		ORDER BY fetched_at DESC
		LIMIT ? OFFSET ?
	`, where)

	args = append(args, filters.Limit, filters.Offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var ledgers []*OrderLedger
	for rows.Next() {
		ledger := &OrderLedger{}
		var syncRunID sql.NullInt64
		var validationNotes sql.NullString

		err := rows.Scan(
			&ledger.ID,
			&ledger.OrderID,
			&syncRunID,
			&ledger.Provider,
			&ledger.FetchedAt,
			&ledger.LedgerState,
			&ledger.LedgerVersion,
			&ledger.LedgerJSON,
			&ledger.TotalCharged,
			&ledger.ChargeCount,
			&ledger.PaymentMethodTypes,
			&ledger.HasRefunds,
			&ledger.IsValid,
			&validationNotes,
		)
		if err != nil {
			return nil, err
		}

		if syncRunID.Valid {
			ledger.SyncRunID = syncRunID.Int64
		}
		if validationNotes.Valid {
			ledger.ValidationNotes = validationNotes.String
		}

		ledgers = append(ledgers, ledger)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &LedgerListResult{
		Ledgers:    ledgers,
		TotalCount: totalCount,
		Limit:      filters.Limit,
		Offset:     filters.Offset,
	}, nil
}

// UpdateChargeMatch updates a ledger charge's match status
func (s *Storage) UpdateChargeMatch(chargeID int64, transactionID string, confidence float64, splitCount int) error {
	query := `
		UPDATE ledger_charges
		SET monarch_transaction_id = ?,
		    is_matched = 1,
		    match_confidence = ?,
		    matched_at = CURRENT_TIMESTAMP,
		    split_count = ?
		WHERE id = ?
	`

	_, err := s.db.Exec(query, transactionID, confidence, splitCount, chargeID)
	return err
}

// GetUnmatchedCharges returns charges that haven't been matched to Monarch transactions
func (s *Storage) GetUnmatchedCharges(provider string, limit int) ([]LedgerCharge, error) {
	if limit <= 0 {
		limit = 100
	}

	var query string
	var args []interface{}

	if provider != "" {
		query = `
			SELECT c.id, c.order_ledger_id, c.order_id, c.sync_run_id, c.charge_sequence,
			       c.charge_amount, c.charge_type, c.payment_method, c.card_type, c.card_last_four,
			       c.monarch_transaction_id, c.is_matched, c.match_confidence, c.matched_at, c.split_count
			FROM ledger_charges c
			JOIN order_ledgers l ON c.order_ledger_id = l.id
			WHERE c.is_matched = 0
			  AND c.charge_type = 'payment'
			  AND l.provider = ?
			ORDER BY l.fetched_at DESC
			LIMIT ?
		`
		args = []interface{}{provider, limit}
	} else {
		query = `
			SELECT c.id, c.order_ledger_id, c.order_id, c.sync_run_id, c.charge_sequence,
			       c.charge_amount, c.charge_type, c.payment_method, c.card_type, c.card_last_four,
			       c.monarch_transaction_id, c.is_matched, c.match_confidence, c.matched_at, c.split_count
			FROM ledger_charges c
			JOIN order_ledgers l ON c.order_ledger_id = l.id
			WHERE c.is_matched = 0
			  AND c.charge_type = 'payment'
			ORDER BY l.fetched_at DESC
			LIMIT ?
		`
		args = []interface{}{limit}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var charges []LedgerCharge
	for rows.Next() {
		var charge LedgerCharge
		var syncRunID sql.NullInt64
		var txID sql.NullString
		var cardType, cardLastFour sql.NullString
		var matchedAt sql.NullTime

		err := rows.Scan(
			&charge.ID,
			&charge.OrderLedgerID,
			&charge.OrderID,
			&syncRunID,
			&charge.ChargeSequence,
			&charge.ChargeAmount,
			&charge.ChargeType,
			&charge.PaymentMethod,
			&cardType,
			&cardLastFour,
			&txID,
			&charge.IsMatched,
			&charge.MatchConfidence,
			&matchedAt,
			&charge.SplitCount,
		)
		if err != nil {
			return nil, err
		}

		if syncRunID.Valid {
			charge.SyncRunID = syncRunID.Int64
		}
		if txID.Valid {
			charge.MonarchTransactionID = txID.String
		}
		if cardType.Valid {
			charge.CardType = cardType.String
		}
		if cardLastFour.Valid {
			charge.CardLastFour = cardLastFour.String
		}
		if matchedAt.Valid {
			charge.MatchedAt = matchedAt.Time
		}

		charges = append(charges, charge)
	}

	return charges, rows.Err()
}

// getChargesForLedger retrieves all charges for a ledger
func (s *Storage) getChargesForLedger(ledgerID int64) ([]LedgerCharge, error) {
	query := `
		SELECT id, order_ledger_id, order_id, sync_run_id, charge_sequence,
		       charge_amount, charge_type, payment_method, card_type, card_last_four,
		       charged_at, monarch_transaction_id, is_matched, match_confidence, matched_at, split_count
		FROM ledger_charges
		WHERE order_ledger_id = ?
		ORDER BY charge_sequence
	`

	rows, err := s.db.Query(query, ledgerID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var charges []LedgerCharge
	for rows.Next() {
		var charge LedgerCharge
		var syncRunID sql.NullInt64
		var txID sql.NullString
		var cardType, cardLastFour sql.NullString
		var chargedAt, matchedAt sql.NullTime

		err := rows.Scan(
			&charge.ID,
			&charge.OrderLedgerID,
			&charge.OrderID,
			&syncRunID,
			&charge.ChargeSequence,
			&charge.ChargeAmount,
			&charge.ChargeType,
			&charge.PaymentMethod,
			&cardType,
			&cardLastFour,
			&chargedAt,
			&txID,
			&charge.IsMatched,
			&charge.MatchConfidence,
			&matchedAt,
			&charge.SplitCount,
		)
		if err != nil {
			return nil, err
		}

		if syncRunID.Valid {
			charge.SyncRunID = syncRunID.Int64
		}
		if txID.Valid {
			charge.MonarchTransactionID = txID.String
		}
		if cardType.Valid {
			charge.CardType = cardType.String
		}
		if cardLastFour.Valid {
			charge.CardLastFour = cardLastFour.String
		}
		if chargedAt.Valid {
			charge.ChargedAt = chargedAt.Time
		}
		if matchedAt.Valid {
			charge.MatchedAt = matchedAt.Time
		}

		charges = append(charges, charge)
	}

	return charges, rows.Err()
}

// Helper functions for nullable values
func nullInt64(v int64) interface{} {
	if v == 0 {
		return nil
	}
	return v
}

func nullString(v string) interface{} {
	if v == "" {
		return nil
	}
	return v
}

func nullTime(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t
}
