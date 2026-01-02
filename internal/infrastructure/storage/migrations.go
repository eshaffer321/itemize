package storage

import (
	"database/sql"
	"fmt"
	"log"
)

// Migration represents a database schema migration
type Migration struct {
	Version int
	Name    string
	Up      func(*sql.Tx) error
}

// allMigrations defines all migrations in order
var allMigrations = []Migration{
	{
		Version: 1,
		Name:    "initial_schema",
		Up:      migration001InitialSchema,
	},
	{
		Version: 2,
		Name:    "add_sync_runs_table",
		Up:      migration002AddSyncRunsTable,
	},
	{
		Version: 3,
		Name:    "add_api_calls_table",
		Up:      migration003AddAPICallsTable,
	},
	{
		Version: 4,
		Name:    "backfill_null_values",
		Up:      migration004BackfillNullValues,
	},
	{
		Version: 5,
		Name:    "add_ledger_tables",
		Up:      migration005AddLedgerTables,
	},
	{
		Version: 6,
		Name:    "add_charged_at_column",
		Up:      migration006AddChargedAtColumn,
	},
}

// runMigrations executes all pending migrations
func (s *Storage) runMigrations() error {
	// Ensure migrations table exists
	if err := s.ensureMigrationsTable(); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get applied migrations
	applied, err := s.getAppliedMigrations()
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Run pending migrations
	for _, migration := range allMigrations {
		if applied[migration.Version] {
			continue // Already applied
		}

		log.Printf("Running migration %d: %s", migration.Version, migration.Name)

		// Run migration in transaction
		tx, err := s.db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction for migration %d: %w", migration.Version, err)
		}

		// Execute migration
		if err := migration.Up(tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %d (%s) failed: %w", migration.Version, migration.Name, err)
		}

		// Record migration
		_, err = tx.Exec(`
			INSERT INTO schema_migrations (version, name) VALUES (?, ?)
		`, migration.Version, migration.Name)
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to record migration %d: %w", migration.Version, err)
		}

		// Commit
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %d: %w", migration.Version, err)
		}

		log.Printf("✅ Migration %d complete", migration.Version)
	}

	return nil
}

// ensureMigrationsTable creates the schema_migrations table
func (s *Storage) ensureMigrationsTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`

	_, err := s.db.Exec(query)
	return err
}

// getAppliedMigrations returns a set of applied migration versions
func (s *Storage) getAppliedMigrations() (map[int]bool, error) {
	applied := make(map[int]bool)

	rows, err := s.db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}

	return applied, rows.Err()
}

// ================================================================
// MIGRATION FUNCTIONS
// ================================================================

// migration001InitialSchema creates the initial processing_records table
func migration001InitialSchema(db *sql.Tx) error {
	queries := []string{
		// Main processing records table
		`CREATE TABLE IF NOT EXISTS processing_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			order_id TEXT UNIQUE NOT NULL,
			provider TEXT DEFAULT 'walmart',
			transaction_id TEXT,
			order_date TIMESTAMP,
			processed_at TIMESTAMP,
			order_total REAL,
			order_subtotal REAL,
			order_tax REAL,
			order_tip REAL,
			transaction_amount REAL,
			split_count INTEGER,
			status TEXT,
			error_message TEXT,
			item_count INTEGER,
			match_confidence REAL,
			dry_run BOOLEAN DEFAULT 0,
			items_json TEXT,
			splits_json TEXT,
			multi_delivery_data TEXT
		)`,

		// Indexes for common queries
		`CREATE INDEX IF NOT EXISTS idx_processing_records_provider
		 ON processing_records(provider)`,

		`CREATE INDEX IF NOT EXISTS idx_processing_records_order_date
		 ON processing_records(order_date)`,

		`CREATE INDEX IF NOT EXISTS idx_processing_records_status
		 ON processing_records(status)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query: %w", err)
		}
	}

	return nil
}

// migration002AddSyncRunsTable creates the sync_runs table
func migration002AddSyncRunsTable(db *sql.Tx) error {
	// Drop old sync_runs table if it exists (it was a placeholder with wrong schema)
	_, _ = db.Exec(`DROP TABLE IF EXISTS sync_runs`)

	queries := []string{
		`CREATE TABLE sync_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider TEXT NOT NULL,
			started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			completed_at TIMESTAMP,
			lookback_days INTEGER,
			dry_run BOOLEAN DEFAULT 0,
			orders_found INTEGER DEFAULT 0,
			orders_processed INTEGER DEFAULT 0,
			orders_skipped INTEGER DEFAULT 0,
			orders_errored INTEGER DEFAULT 0,
			status TEXT DEFAULT 'running'
		)`,

		`CREATE INDEX IF NOT EXISTS idx_sync_runs_provider
		 ON sync_runs(provider)`,

		`CREATE INDEX IF NOT EXISTS idx_sync_runs_started
		 ON sync_runs(started_at DESC)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return err
		}
	}

	return nil
}

// migration003AddAPICallsTable creates the api_calls table for logging
func migration003AddAPICallsTable(db *sql.Tx) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS api_calls (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id INTEGER,
			order_id TEXT,
			timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			method TEXT NOT NULL,
			request_json TEXT,
			response_json TEXT,
			error TEXT,
			duration_ms INTEGER,
			FOREIGN KEY (run_id) REFERENCES sync_runs(id)
		)`,

		`CREATE INDEX IF NOT EXISTS idx_api_calls_run_id
		 ON api_calls(run_id)`,

		`CREATE INDEX IF NOT EXISTS idx_api_calls_order_id
		 ON api_calls(order_id)`,

		`CREATE INDEX IF NOT EXISTS idx_api_calls_timestamp
		 ON api_calls(timestamp DESC)`,

		`CREATE INDEX IF NOT EXISTS idx_api_calls_method
		 ON api_calls(method)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return err
		}
	}

	return nil
}

// migration004BackfillNullValues converts NULL values to empty strings for consistency.
// This ensures the Go code can scan these columns without sql.NullString for most cases.
// Fields that are semantically nullable (error_message, transaction_id, multi_delivery_data)
// are intentionally left as-is since NULL has meaning there.
func migration004BackfillNullValues(db *sql.Tx) error {
	// Backfill empty strings for fields that should never be NULL
	queries := []string{
		// provider should always have a value
		`UPDATE processing_records SET provider = 'unknown' WHERE provider IS NULL`,

		// status should always have a value
		`UPDATE processing_records SET status = 'unknown' WHERE status IS NULL`,

		// JSON fields: empty array is better than NULL for items/splits
		`UPDATE processing_records SET items_json = '[]' WHERE items_json IS NULL`,
		`UPDATE processing_records SET splits_json = '[]' WHERE splits_json IS NULL`,

		// Numeric fields: 0 is better than NULL
		`UPDATE processing_records SET order_total = 0 WHERE order_total IS NULL`,
		`UPDATE processing_records SET order_subtotal = 0 WHERE order_subtotal IS NULL`,
		`UPDATE processing_records SET order_tax = 0 WHERE order_tax IS NULL`,
		`UPDATE processing_records SET order_tip = 0 WHERE order_tip IS NULL`,
		`UPDATE processing_records SET transaction_amount = 0 WHERE transaction_amount IS NULL`,
		`UPDATE processing_records SET split_count = 0 WHERE split_count IS NULL`,
		`UPDATE processing_records SET item_count = 0 WHERE item_count IS NULL`,
		`UPDATE processing_records SET match_confidence = 0 WHERE match_confidence IS NULL`,

		// Note: We intentionally leave these as potentially NULL since NULL has meaning:
		// - transaction_id: NULL means no match found
		// - error_message: NULL means no error (success)
		// - multi_delivery_data: NULL means not a multi-delivery order
		// - order_date, processed_at: Could be NULL for very old/corrupt records
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("backfill query failed: %w", err)
		}
	}

	return nil
}

// migration005AddLedgerTables creates tables for storing order ledger data and charges.
// This enables:
// - Tracking ledger state changes over time (payment_pending → charged → refunded)
// - Per-charge tracking for multi-delivery orders
// - Detecting when ledger changes require re-processing
// - Audit trail for debugging and refund matching
func migration005AddLedgerTables(db *sql.Tx) error {
	queries := []string{
		// order_ledgers: Store ledger snapshots with history
		`CREATE TABLE IF NOT EXISTS order_ledgers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			order_id TEXT NOT NULL,
			sync_run_id INTEGER,
			provider TEXT NOT NULL,
			fetched_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			ledger_state TEXT NOT NULL,
			ledger_version INTEGER DEFAULT 1,
			ledger_json TEXT NOT NULL,
			total_charged REAL,
			charge_count INTEGER,
			payment_method_types TEXT,
			has_refunds BOOLEAN DEFAULT 0,
			is_valid BOOLEAN DEFAULT 1,
			validation_notes TEXT,
			FOREIGN KEY (sync_run_id) REFERENCES sync_runs(id)
		)`,

		// Indexes for order_ledgers
		`CREATE INDEX IF NOT EXISTS idx_order_ledgers_order_id
		 ON order_ledgers(order_id)`,

		`CREATE INDEX IF NOT EXISTS idx_order_ledgers_provider
		 ON order_ledgers(provider)`,

		`CREATE INDEX IF NOT EXISTS idx_order_ledgers_state
		 ON order_ledgers(ledger_state)`,

		`CREATE INDEX IF NOT EXISTS idx_order_ledgers_fetched
		 ON order_ledgers(fetched_at DESC)`,

		// ledger_charges: Normalized charge entries for querying
		`CREATE TABLE IF NOT EXISTS ledger_charges (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			order_ledger_id INTEGER NOT NULL,
			order_id TEXT NOT NULL,
			sync_run_id INTEGER,
			charge_sequence INTEGER NOT NULL,
			charge_amount REAL NOT NULL,
			charge_type TEXT,
			payment_method TEXT,
			card_type TEXT,
			card_last_four TEXT,
			monarch_transaction_id TEXT,
			is_matched BOOLEAN DEFAULT 0,
			match_confidence REAL,
			matched_at TIMESTAMP,
			split_count INTEGER,
			FOREIGN KEY (order_ledger_id) REFERENCES order_ledgers(id),
			FOREIGN KEY (sync_run_id) REFERENCES sync_runs(id)
		)`,

		// Indexes for ledger_charges
		`CREATE INDEX IF NOT EXISTS idx_ledger_charges_order_id
		 ON ledger_charges(order_id)`,

		`CREATE INDEX IF NOT EXISTS idx_ledger_charges_ledger_id
		 ON ledger_charges(order_ledger_id)`,

		`CREATE INDEX IF NOT EXISTS idx_ledger_charges_monarch_tx
		 ON ledger_charges(monarch_transaction_id)`,

		`CREATE INDEX IF NOT EXISTS idx_ledger_charges_unmatched
		 ON ledger_charges(is_matched) WHERE is_matched = 0`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to create ledger tables: %w", err)
		}
	}

	return nil
}

// migration006AddChargedAtColumn adds the charged_at column to ledger_charges table.
// This allows tracking when each charge actually occurred for display in the UI.
func migration006AddChargedAtColumn(db *sql.Tx) error {
	query := `ALTER TABLE ledger_charges ADD COLUMN charged_at TIMESTAMP`

	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to add charged_at column: %w", err)
	}

	return nil
}
