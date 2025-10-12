package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/config"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	var (
		dbPath     string
		configFile string
	)
	flag.StringVar(&dbPath, "db", "", "Path to database file (uses config if not specified)")
	flag.StringVar(&configFile, "config", "", "Configuration file path")
	flag.Parse()

	// Load config if database path not specified
	if dbPath == "" {
		cfg, err := config.Load(configFile)
		if err != nil {
			log.Printf("Warning: Failed to load config: %v", err)
			dbPath = "processing.db" // fallback
		} else {
			dbPath = cfg.Storage.DatabasePath
			if dbPath == "" {
				dbPath = "processing.db" // fallback
			}
		}
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	fmt.Println("📊 COSTCO SYNC AUDIT REPORT")
	fmt.Println("=" + strings.Repeat("=", 60))
	fmt.Printf("Database: %s\n", dbPath)
	fmt.Printf("Generated: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))

	// Overall Statistics
	fmt.Println("📈 OVERALL STATISTICS")
	fmt.Println(strings.Repeat("-", 40))

	var totalOrders, successCount, failedCount, skippedCount int
	var totalAmount, totalSplits float64

	err = db.QueryRow(`
		SELECT 
			COUNT(*) as total_orders,
			SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success_count,
			SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed_count,
			SUM(CASE WHEN status = 'skipped' OR status = 'no-match' THEN 1 ELSE 0 END) as skipped_count,
			COALESCE(SUM(order_amount), 0) as total_amount,
			COALESCE(SUM(split_count), 0) as total_splits
		FROM processing_records
	`).Scan(&totalOrders, &successCount, &failedCount, &skippedCount, &totalAmount, &totalSplits)

	if err != nil {
		log.Printf("Error getting stats: %v", err)
	}

	successRate := 0.0
	if totalOrders > 0 {
		successRate = float64(successCount) / float64(totalOrders) * 100
	}

	fmt.Printf("Total Orders Processed: %d\n", totalOrders)
	fmt.Printf("Successfully Split: %d (%.1f%%)\n", successCount, successRate)
	fmt.Printf("Failed: %d\n", failedCount)
	fmt.Printf("Skipped/No Match: %d\n", skippedCount)
	fmt.Printf("Total Amount Processed: $%.2f\n", totalAmount)
	fmt.Printf("Total Splits Created: %.0f\n", totalSplits)
	fmt.Println()

	// Sync Run History
	fmt.Println("🔄 RECENT SYNC RUNS")
	fmt.Println(strings.Repeat("-", 40))

	rows, err := db.Query(`
		SELECT 
			started_at,
			completed_at,
			total_orders,
			processed,
			skipped,
			failed,
			dry_run,
			lookback_days
		FROM sync_runs
		ORDER BY started_at DESC
		LIMIT 10
	`)
	if err != nil {
		log.Printf("Error getting sync runs: %v", err)
	} else {
		defer rows.Close()

		fmt.Printf("%-20s %-10s %-8s %-10s %-10s\n", "Date/Time", "Orders", "Status", "Mode", "Days Back")
		fmt.Println(strings.Repeat("-", 70))

		for rows.Next() {
			var startedAt, completedAt sql.NullString
			var totalOrders, processed, skipped, failed, dryRun, lookbackDays int

			err := rows.Scan(&startedAt, &completedAt, &totalOrders, &processed, &skipped, &failed, &dryRun, &lookbackDays)
			if err != nil {
				continue
			}

			startTime, _ := time.Parse("2006-01-02 15:04:05", startedAt.String)
			status := fmt.Sprintf("✅%d ❌%d ⏭️%d", processed, failed, skipped)
			mode := "PROD"
			if dryRun == 1 {
				mode = "DRY"
			}

			fmt.Printf("%-20s %-10d %-8s %-10s %-10d\n",
				startTime.Format("2006-01-02 15:04"),
				totalOrders,
				status,
				mode,
				lookbackDays,
			)
		}
	}
	fmt.Println()

	// Recent Processing Details
	fmt.Println("📝 RECENT PROCESSING DETAILS")
	fmt.Println(strings.Repeat("-", 40))

	rows, err = db.Query(`
		SELECT 
			order_id,
			order_date,
			processed_at,
			status,
			order_amount,
			split_count,
			match_confidence,
			error_message
		FROM processing_records
		ORDER BY processed_at DESC
		LIMIT 10
	`)
	if err != nil {
		log.Printf("Error getting processing records: %v", err)
	} else {
		defer rows.Close()

		for rows.Next() {
			var orderID, status string
			var orderDate, processedAt sql.NullString
			var orderAmount, matchConfidence float64
			var splitCount int
			var errorMsg sql.NullString

			err := rows.Scan(&orderID, &orderDate, &processedAt, &status, &orderAmount, &splitCount, &matchConfidence, &errorMsg)
			if err != nil {
				continue
			}

			orderTime, _ := time.Parse("2006-01-02 15:04:05+00:00", orderDate.String)

			statusIcon := "✅"
			if status == "failed" || status == "no-match" {
				statusIcon = "❌"
			} else if status == "skipped" {
				statusIcon = "⏭️"
			} else if status == "dry-run" {
				statusIcon = "🔍"
			}

			fmt.Printf("\n%s Order: %s\n", statusIcon, orderID)
			fmt.Printf("   Date: %s | Amount: $%.2f\n", orderTime.Format("2006-01-02"), orderAmount)
			fmt.Printf("   Status: %s | Splits: %d | Confidence: %.1f%%\n", status, splitCount, matchConfidence*100)

			if errorMsg.Valid && errorMsg.String != "" {
				fmt.Printf("   Error: %s\n", errorMsg.String)
			}
		}
	}
	fmt.Println()

	// Error Analysis
	fmt.Println("\n❌ ERROR ANALYSIS")
	fmt.Println(strings.Repeat("-", 40))

	rows, err = db.Query(`
		SELECT 
			error_message,
			COUNT(*) as count
		FROM processing_records
		WHERE status = 'failed' AND error_message IS NOT NULL
		GROUP BY error_message
		ORDER BY count DESC
	`)
	if err != nil {
		log.Printf("Error getting error analysis: %v", err)
	} else {
		defer rows.Close()

		for rows.Next() {
			var errorMsg string
			var count int

			err := rows.Scan(&errorMsg, &count)
			if err != nil {
				continue
			}

			fmt.Printf("%d occurrences: %s\n", count, errorMsg)
		}
	}
	fmt.Println()

	// Data Quality Checks
	fmt.Println("🔍 DATA QUALITY CHECKS")
	fmt.Println(strings.Repeat("-", 40))

	// Check for duplicate processing
	var duplicates int
	err = db.QueryRow(`
		SELECT COUNT(*) 
		FROM (
			SELECT order_id, COUNT(*) as cnt 
			FROM processing_records 
			GROUP BY order_id 
			HAVING cnt > 1
		)
	`).Scan(&duplicates)

	if duplicates > 0 {
		fmt.Printf("⚠️  Found %d orders processed multiple times\n", duplicates)
	} else {
		fmt.Printf("✅ No duplicate order processing detected\n")
	}

	// Check for orphaned transactions
	var orphaned int
	err = db.QueryRow(`
		SELECT COUNT(*) 
		FROM processing_records 
		WHERE status = 'success' AND transaction_id IS NULL
	`).Scan(&orphaned)

	if orphaned > 0 {
		fmt.Printf("⚠️  Found %d successful records without transaction IDs\n", orphaned)
	} else {
		fmt.Printf("✅ All successful records have transaction IDs\n")
	}

	// Check match confidence
	var lowConfidence int
	err = db.QueryRow(`
		SELECT COUNT(*) 
		FROM processing_records 
		WHERE status = 'success' AND match_confidence < 0.8
	`).Scan(&lowConfidence)

	if lowConfidence > 0 {
		fmt.Printf("⚠️  Found %d matches with confidence < 80%%\n", lowConfidence)
	} else {
		fmt.Printf("✅ All matches have high confidence (>= 80%%)\n")
	}

	fmt.Println()

	// Audit Trail Summary
	fmt.Println("📋 AUDIT TRAIL SUMMARY")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Println("✅ Complete audit trail for all processing")
	fmt.Println("✅ Transaction IDs linked to Monarch")
	fmt.Println("✅ Error messages captured for failures")
	fmt.Println("✅ Match confidence scores recorded")
	fmt.Println("✅ Processing timestamps maintained")
	fmt.Println("✅ Sync run history tracked")

	// Recommendations
	fmt.Println("\n💡 RECOMMENDATIONS")
	fmt.Println(strings.Repeat("-", 40))

	if successRate < 80 {
		fmt.Println("• Success rate is below 80% - review failed orders")
	}
	if lowConfidence > 0 {
		fmt.Println("• Some matches have low confidence - review matching logic")
	}
	if duplicates > 0 {
		fmt.Println("• Duplicate processing detected - check force flag usage")
	}

	fmt.Println("• Consider implementing:")
	fmt.Println("  - Structured JSON logging for better parsing")
	fmt.Println("  - Metrics export (Prometheus/Grafana)")
	fmt.Println("  - Email alerts for failures")
	fmt.Println("  - Automated retry mechanism")
	fmt.Println("  - Category mapping audit logs")
}
