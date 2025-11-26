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
			tx.Rollback()
			return fmt.Errorf("migration %d (%s) failed: %w", migration.Version, migration.Name, err)
		}

		// Record migration
		_, err = tx.Exec(`
			INSERT INTO schema_migrations (version, name) VALUES (?, ?)
		`, migration.Version, migration.Name)
		if err != nil {
			tx.Rollback()
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
	defer rows.Close()

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
