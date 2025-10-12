package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	var (
		targetDB = flag.String("target", "monarch_sync.db", "Target consolidated database file")
		dryRun   = flag.Bool("dry-run", true, "Preview changes without applying")
	)
	flag.Parse()

	// Find all .db files in current directory
	dbFiles, err := filepath.Glob("*.db")
	if err != nil {
		log.Fatalf("Failed to find database files: %v", err)
	}

	if len(dbFiles) == 0 {
		log.Println("No database files found")
		return
	}

	fmt.Printf("Found %d database files:\n", len(dbFiles))
	for _, db := range dbFiles {
		fmt.Printf("  - %s\n", db)
	}

	if *dryRun {
		fmt.Println("\n=== DRY RUN MODE ===")
		fmt.Printf("Would consolidate all databases into: %s\n", *targetDB)
		return
	}

	// Create target database
	targetConn, err := sql.Open("sqlite3", *targetDB)
	if err != nil {
		log.Fatalf("Failed to create target database: %v", err)
	}
	defer targetConn.Close()

	// Create tables in target database
	if err := createTables(targetConn); err != nil {
		log.Fatalf("Failed to create tables: %v", err)
	}

	// Consolidate data from each database
	totalRecords := 0
	for _, dbFile := range dbFiles {
		if dbFile == *targetDB {
			continue // Skip target database
		}

		fmt.Printf("\nProcessing %s...\n", dbFile)

		sourceConn, err := sql.Open("sqlite3", dbFile)
		if err != nil {
			fmt.Printf("Warning: Failed to open %s: %v\n", dbFile, err)
			continue
		}

		records, err := consolidateDatabase(sourceConn, targetConn, dbFile)
		if err != nil {
			fmt.Printf("Warning: Failed to consolidate %s: %v\n", dbFile, err)
		} else {
			fmt.Printf("  Migrated %d records from %s\n", records, dbFile)
			totalRecords += records
		}

		sourceConn.Close()
	}

	fmt.Printf("\n=== CONSOLIDATION COMPLETE ===\n")
	fmt.Printf("Total records consolidated: %d\n", totalRecords)
	fmt.Printf("Target database: %s\n", *targetDB)
}

func createTables(db *sql.DB) error {
	// Create processing_records table
	processingRecordsSchema := `
	CREATE TABLE IF NOT EXISTS processing_records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		order_id TEXT NOT NULL,
		transaction_id TEXT,
		order_date DATETIME NOT NULL,
		processed_at DATETIME NOT NULL,
		order_amount REAL NOT NULL,
		split_count INTEGER NOT NULL,
		categories TEXT NOT NULL,
		status TEXT NOT NULL,
		error_message TEXT,
		item_count INTEGER NOT NULL,
		notes TEXT,
		item_details TEXT,
		match_confidence REAL,
		source_db TEXT
	);`

	// Create sync_runs table
	syncRunsSchema := `
	CREATE TABLE IF NOT EXISTS sync_runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		started_at DATETIME NOT NULL,
		completed_at DATETIME,
		total_orders INTEGER NOT NULL,
		dry_run BOOLEAN NOT NULL,
		lookback_days INTEGER NOT NULL,
		processed INTEGER NOT NULL,
		skipped INTEGER NOT NULL,
		failed INTEGER NOT NULL,
		run_id TEXT
	);`

	if _, err := db.Exec(processingRecordsSchema); err != nil {
		return fmt.Errorf("failed to create processing_records table: %w", err)
	}

	if _, err := db.Exec(syncRunsSchema); err != nil {
		return fmt.Errorf("failed to create sync_runs table: %w", err)
	}

	return nil
}

func consolidateDatabase(sourceDB, targetDB *sql.DB, sourceFile string) (int, error) {
	// Check if source database has processing_records table
	var tableExists bool
	err := sourceDB.QueryRow(`
		SELECT COUNT(*) > 0 
		FROM sqlite_master 
		WHERE type='table' AND name='processing_records'
	`).Scan(&tableExists)

	if err != nil || !tableExists {
		return 0, nil // No data to migrate
	}

	// Get all records from source database
	rows, err := sourceDB.Query(`
		SELECT order_id, transaction_id, order_date, processed_at, 
		       order_amount, split_count, categories, status, error_message,
		       item_count, notes, item_details, match_confidence
		FROM processing_records
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	recordCount := 0
	for rows.Next() {
		var orderID, transactionID, categories, status, errorMessage, notes, itemDetails sql.NullString
		var orderDate, processedAt string
		var orderAmount, matchConfidence sql.NullFloat64
		var splitCount, itemCount sql.NullInt64

		err := rows.Scan(&orderID, &transactionID, &orderDate, &processedAt,
			&orderAmount, &splitCount, &categories, &status, &errorMessage,
			&itemCount, &notes, &itemDetails, &matchConfidence)
		if err != nil {
			return recordCount, err
		}

		// Check if record already exists in target database
		var exists bool
		err = targetDB.QueryRow(`
			SELECT COUNT(*) > 0 
			FROM processing_records 
			WHERE order_id = ?
		`, orderID.String).Scan(&exists)

		if err != nil {
			return recordCount, err
		}

		if exists {
			continue // Skip duplicate records
		}

		// Insert record into target database
		_, err = targetDB.Exec(`
			INSERT INTO processing_records 
			(order_id, transaction_id, order_date, processed_at, order_amount, 
			 split_count, categories, status, error_message, item_count, 
			 notes, item_details, match_confidence, source_db)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, orderID.String, transactionID.String, orderDate, processedAt,
			orderAmount.Float64, splitCount.Int64, categories.String, status.String,
			errorMessage.String, itemCount.Int64, notes.String, itemDetails.String,
			matchConfidence.Float64, sourceFile)

		if err != nil {
			return recordCount, err
		}

		recordCount++
	}

	return recordCount, nil
}
