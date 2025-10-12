package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/storage"
)

// This tool enriches historical data by fetching split information from Monarch
func main() {
	// Initialize Monarch client
	token := os.Getenv("MONARCH_TOKEN")
	if token == "" {
		log.Fatal("MONARCH_TOKEN not set")
	}

	client, err := monarch.NewClientWithToken(token)
	if err != nil {
		log.Fatalf("Failed to create Monarch client: %v", err)
	}

	// Initialize storage
	// Use config for database path, fallback to default
	dbPath := cfg.Storage.DatabasePath
	if dbPath == "" {
		dbPath = "processing.db"
	}
	store, err := storage.NewStorage(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()

	// Create enhanced tables
	if err := store.CreateTablesV2(); err != nil {
		log.Printf("Warning: Failed to create v2 tables: %v", err)
	}

	// Get recent records that have transaction IDs
	records, err := store.GetRecentRecords(100)
	if err != nil {
		log.Fatalf("Failed to get records: %v", err)
	}

	fmt.Printf("Found %d records to potentially enrich\n", len(records))

	ctx := context.Background()
	enrichedCount := 0

	for _, record := range records {
		// Skip if no transaction ID
		if record.TransactionID == "" {
			continue
		}

		// Skip failed records
		if record.Status == "failed" {
			continue
		}

		fmt.Printf("\nProcessing order %s (Transaction: %s)\n", record.OrderID, record.TransactionID)

		// Fetch transaction with splits from Monarch
		// Note: The monarch client doesn't have a GetTransaction by ID method
		// We need to fetch all transactions and find the one we want
		fmt.Printf("  ⚠️  Fetching transaction splits (this may take a moment)...\n")

		// For now, skip enrichment since we don't have a direct way to fetch by ID
		// This would need to be implemented in the monarch client
		continue

		// TODO: Implement when monarch client supports GetTransaction(id)

		// Create category map
		categoryMap := make(map[string]string)
		for _, cat := range categories {
			categoryMap[cat.ID] = cat.Name
		}

		// Convert to enhanced record
		v2Record := &storage.ProcessingRecordV2{
			OrderID:           record.OrderID,
			Provider:          "walmart",
			TransactionID:     record.TransactionID,
			OrderDate:         record.OrderDate,
			ProcessedAt:       record.ProcessedAt,
			OrderTotal:        record.OrderAmount,
			TransactionAmount: tx.Amount,
			SplitCount:        len(tx.TransactionSplits),
			Status:            record.Status,
			ErrorMessage:      record.ErrorMessage,
			ItemCount:         record.ItemCount,
			MatchConfidence:   record.MatchConfidence,
			DryRun:            record.Status == "dry-run",
		}

		// Process splits
		var splits []storage.SplitDetail
		var orderSubtotal, orderTax, orderTip float64

		for _, split := range tx.TransactionSplits {
			categoryName := "Unknown"
			if name, exists := categoryMap[split.CategoryID]; exists {
				categoryName = name
			}

			// Check if this is a tip split
			if categoryName == "Shopping" || categoryName == "Delivery & Shipping" {
				// Likely a tip
				orderTip = math.Abs(split.Amount)
			} else {
				orderSubtotal += math.Abs(split.Amount)
			}

			splitDetail := storage.SplitDetail{
				CategoryID:   split.CategoryID,
				CategoryName: categoryName,
				Amount:       split.Amount,
				Notes:        split.Notes,
			}

			// Try to parse items from notes if available
			if split.Notes != "" {
				// Notes might contain item list
				splitDetail.Items = parseItemsFromNotes(split.Notes)
			}

			splits = append(splits, splitDetail)
		}

		// Estimate tax (if we don't have tip, the difference is probably tax)
		if orderTip == 0 {
			orderTax = math.Abs(tx.Amount) - orderSubtotal
		} else {
			// With tip, we need to estimate
			orderTax = (math.Abs(tx.Amount) - orderTip) - orderSubtotal
		}

		v2Record.OrderSubtotal = orderSubtotal
		v2Record.OrderTax = orderTax
		v2Record.OrderTip = orderTip
		v2Record.Splits = splits

		// Save enhanced record
		if err := store.SaveRecordV2(v2Record); err != nil {
			fmt.Printf("  ❌ Failed to save enhanced record: %v\n", err)
		} else {
			fmt.Printf("  💾 Saved enhanced record\n")
			fmt.Printf("     Subtotal: $%.2f, Tax: $%.2f, Tip: $%.2f\n",
				orderSubtotal, orderTax, orderTip)

			for _, split := range splits {
				fmt.Printf("     - %s: $%.2f\n", split.CategoryName, math.Abs(split.Amount))
			}

			enrichedCount++
		}
	}

	fmt.Printf("\n✅ Enriched %d records with split data from Monarch\n", enrichedCount)
}

// parseItemsFromNotes attempts to extract item information from split notes
func parseItemsFromNotes(notes string) []storage.OrderItem {
	// This is a simple parser - could be enhanced
	// Notes format might be like: "Item1 (x2) - $10.00, Item2 - $5.00"

	var items []storage.OrderItem

	// For now, just store the whole note as a single "item"
	if notes != "" {
		items = append(items, storage.OrderItem{
			Name: notes,
		})
	}

	return items
}

// Helper to pretty print JSON
func prettyPrint(v interface{}) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}
