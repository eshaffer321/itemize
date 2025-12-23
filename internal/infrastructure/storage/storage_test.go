package storage

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Mock Repository Tests
// =============================================================================

func TestMockRepository_ImplementsInterface(t *testing.T) {
	// This test verifies the mock can be used anywhere Repository is expected
	var repo Repository = NewMockRepository()
	require.NotNil(t, repo)
}

func TestMockRepository_SaveAndGetRecord(t *testing.T) {
	mock := NewMockRepository()

	record := &ProcessingRecord{
		OrderID:   "ORDER-123",
		Provider:  "walmart",
		Status:    "success",
		ItemCount: 5,
		Items: []OrderItem{
			{Name: "Milk", TotalPrice: 3.99},
		},
	}

	err := mock.SaveRecord(record)
	require.NoError(t, err)
	assert.True(t, mock.SaveRecordCalled)
	assert.Equal(t, record, mock.LastSavedRecord)

	retrieved, err := mock.GetRecord("ORDER-123")
	require.NoError(t, err)
	assert.True(t, mock.GetRecordCalled)
	assert.Equal(t, "ORDER-123", retrieved.OrderID)
	assert.Len(t, retrieved.Items, 1)
}

func TestMockRepository_IsProcessed(t *testing.T) {
	mock := NewMockRepository()

	// Not processed initially
	assert.False(t, mock.IsProcessed("ORDER-1"))

	// Add a dry-run record (should not count as processed)
	mock.AddRecord(&ProcessingRecord{
		OrderID: "ORDER-1",
		Status:  "success",
		DryRun:  true,
	})
	assert.False(t, mock.IsProcessed("ORDER-1"))

	// Add a real success record
	mock.AddRecord(&ProcessingRecord{
		OrderID: "ORDER-2",
		Status:  "success",
		DryRun:  false,
	})
	assert.True(t, mock.IsProcessed("ORDER-2"))
}

func TestMockRepository_SyncRuns(t *testing.T) {
	mock := NewMockRepository()

	runID, err := mock.StartSyncRun("costco", 14, false)
	require.NoError(t, err)
	assert.Equal(t, int64(1), runID)
	assert.True(t, mock.StartSyncRunCalled)

	err = mock.CompleteSyncRun(runID, 10, 8, 1, 1)
	require.NoError(t, err)

	run := mock.GetMockSyncRun(runID)
	require.NotNil(t, run)
	assert.True(t, run.completed)
	assert.Equal(t, 10, run.ordersFound)
	assert.Equal(t, 8, run.processed)
}

func TestMockRepository_ErrorInjection(t *testing.T) {
	mock := NewMockRepository()
	mock.SaveRecordErr = assert.AnError

	err := mock.SaveRecord(&ProcessingRecord{OrderID: "test"})
	assert.Error(t, err)
	assert.Equal(t, assert.AnError, err)
}

func TestMockRepository_Reset(t *testing.T) {
	mock := NewMockRepository()

	mock.AddRecord(&ProcessingRecord{OrderID: "ORDER-1"})
	mock.SaveRecordCalled = true
	mock.SaveRecordErr = assert.AnError

	mock.Reset()

	assert.Empty(t, mock.GetAllRecords())
	assert.False(t, mock.SaveRecordCalled)
	assert.Nil(t, mock.SaveRecordErr)
}

// =============================================================================
// SQLite Storage Tests
// =============================================================================

func TestStorage_SaveAndGetRecord_WithItems(t *testing.T) {
	tmpDB := createTempDB(t)
	defer os.Remove(tmpDB)

	store, err := NewStorage(tmpDB)
	require.NoError(t, err)
	defer store.Close()

	// Create a record with items
	record := &ProcessingRecord{
		OrderID:         "ORDER-123",
		Provider:        "walmart",
		TransactionID:   "txn-456",
		OrderDate:       time.Now().Truncate(time.Second),
		ProcessedAt:     time.Now().Truncate(time.Second),
		OrderTotal:      150.00,
		OrderSubtotal:   140.00,
		OrderTax:        10.00,
		TransactionAmount: -150.00,
		SplitCount:      2,
		Status:          "success",
		ItemCount:       3,
		MatchConfidence: 0.95,
		DryRun:          false,
		Items: []OrderItem{
			{Name: "Milk", Quantity: 2, UnitPrice: 3.99, TotalPrice: 7.98, Category: "Groceries"},
			{Name: "Bread", Quantity: 1, UnitPrice: 2.50, TotalPrice: 2.50, Category: "Groceries"},
			{Name: "Shampoo", Quantity: 1, UnitPrice: 8.99, TotalPrice: 8.99, Category: "Personal Care"},
		},
		Splits: []SplitDetail{
			{
				CategoryID:   "cat-groceries",
				CategoryName: "Groceries",
				Amount:       10.48,
				Notes:        "Milk, Bread",
				Items: []OrderItem{
					{Name: "Milk", Quantity: 2, UnitPrice: 3.99, TotalPrice: 7.98},
					{Name: "Bread", Quantity: 1, UnitPrice: 2.50, TotalPrice: 2.50},
				},
			},
			{
				CategoryID:   "cat-personal",
				CategoryName: "Personal Care",
				Amount:       8.99,
				Notes:        "Shampoo",
				Items: []OrderItem{
					{Name: "Shampoo", Quantity: 1, UnitPrice: 8.99, TotalPrice: 8.99},
				},
			},
		},
	}

	// Save the record
	err = store.SaveRecord(record)
	require.NoError(t, err)

	// Retrieve the record
	retrieved, err := store.GetRecord("ORDER-123")
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	// Verify basic fields
	assert.Equal(t, "ORDER-123", retrieved.OrderID)
	assert.Equal(t, "walmart", retrieved.Provider)
	assert.Equal(t, 150.00, retrieved.OrderTotal)
	assert.Equal(t, "success", retrieved.Status)

	// Verify items were stored and retrieved
	require.Len(t, retrieved.Items, 3, "Should have 3 items")
	assert.Equal(t, "Milk", retrieved.Items[0].Name)
	assert.Equal(t, 2.0, retrieved.Items[0].Quantity)
	assert.Equal(t, 3.99, retrieved.Items[0].UnitPrice)
	assert.Equal(t, 7.98, retrieved.Items[0].TotalPrice)
	assert.Equal(t, "Groceries", retrieved.Items[0].Category)

	assert.Equal(t, "Bread", retrieved.Items[1].Name)
	assert.Equal(t, "Shampoo", retrieved.Items[2].Name)

	// Verify splits were stored and retrieved
	require.Len(t, retrieved.Splits, 2, "Should have 2 splits")
	assert.Equal(t, "cat-groceries", retrieved.Splits[0].CategoryID)
	assert.Equal(t, "Groceries", retrieved.Splits[0].CategoryName)
	assert.Equal(t, 10.48, retrieved.Splits[0].Amount)
	assert.Equal(t, "Milk, Bread", retrieved.Splits[0].Notes)
	require.Len(t, retrieved.Splits[0].Items, 2, "Groceries split should have 2 items")

	assert.Equal(t, "cat-personal", retrieved.Splits[1].CategoryID)
	assert.Equal(t, "Personal Care", retrieved.Splits[1].CategoryName)
	require.Len(t, retrieved.Splits[1].Items, 1, "Personal Care split should have 1 item")
}

func TestStorage_SaveRecord_EmptyItems(t *testing.T) {
	tmpDB := createTempDB(t)
	defer os.Remove(tmpDB)

	store, err := NewStorage(tmpDB)
	require.NoError(t, err)
	defer store.Close()

	// Create a record without items (edge case)
	record := &ProcessingRecord{
		OrderID:   "ORDER-EMPTY",
		Provider:  "costco",
		OrderDate: time.Now(),
		Status:    "success",
		Items:     nil,
		Splits:    nil,
	}

	err = store.SaveRecord(record)
	require.NoError(t, err)

	retrieved, err := store.GetRecord("ORDER-EMPTY")
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	// Empty slices should be handled gracefully
	assert.Empty(t, retrieved.Items)
	assert.Empty(t, retrieved.Splits)
}

func TestStorage_SaveRecord_UpdateExisting(t *testing.T) {
	tmpDB := createTempDB(t)
	defer os.Remove(tmpDB)

	store, err := NewStorage(tmpDB)
	require.NoError(t, err)
	defer store.Close()

	// Save initial record without items
	record := &ProcessingRecord{
		OrderID:   "ORDER-UPDATE",
		Provider:  "walmart",
		OrderDate: time.Now(),
		Status:    "dry-run",
		Items:     nil,
	}

	err = store.SaveRecord(record)
	require.NoError(t, err)

	// Update with items (simulates re-processing with -force)
	record.Status = "success"
	record.Items = []OrderItem{
		{Name: "Updated Item", Quantity: 1, UnitPrice: 10.00, TotalPrice: 10.00},
	}

	err = store.SaveRecord(record)
	require.NoError(t, err)

	// Verify update
	retrieved, err := store.GetRecord("ORDER-UPDATE")
	require.NoError(t, err)

	assert.Equal(t, "success", retrieved.Status)
	require.Len(t, retrieved.Items, 1)
	assert.Equal(t, "Updated Item", retrieved.Items[0].Name)
}

func TestStorage_IsProcessed(t *testing.T) {
	tmpDB := createTempDB(t)
	defer os.Remove(tmpDB)

	store, err := NewStorage(tmpDB)
	require.NoError(t, err)
	defer store.Close()

	// Not processed initially
	assert.False(t, store.IsProcessed("ORDER-NEW"))

	// Save a dry-run record (should not count as processed)
	record := &ProcessingRecord{
		OrderID:   "ORDER-DRYRUN",
		Provider:  "walmart",
		OrderDate: time.Now(),
		Status:    "dry-run",
		DryRun:    true,
	}
	err = store.SaveRecord(record)
	require.NoError(t, err)

	assert.False(t, store.IsProcessed("ORDER-DRYRUN"), "Dry-run should not count as processed")

	// Save a success record
	record = &ProcessingRecord{
		OrderID:   "ORDER-SUCCESS",
		Provider:  "walmart",
		OrderDate: time.Now(),
		Status:    "success",
		DryRun:    false,
	}
	err = store.SaveRecord(record)
	require.NoError(t, err)

	assert.True(t, store.IsProcessed("ORDER-SUCCESS"), "Success should count as processed")
}

// =============================================================================
// API Query Method Tests
// =============================================================================

func TestStorage_ListOrders(t *testing.T) {
	tmpDB := createTempDB(t)
	defer os.Remove(tmpDB)

	store, err := NewStorage(tmpDB)
	require.NoError(t, err)
	defer store.Close()

	// Setup: Create several records
	now := time.Now()
	records := []*ProcessingRecord{
		{OrderID: "ORDER-1", Provider: "walmart", Status: "success", OrderTotal: 100.00, OrderDate: now.AddDate(0, 0, -1), ProcessedAt: now},
		{OrderID: "ORDER-2", Provider: "costco", Status: "success", OrderTotal: 200.00, OrderDate: now.AddDate(0, 0, -2), ProcessedAt: now.Add(-time.Hour)},
		{OrderID: "ORDER-3", Provider: "walmart", Status: "failed", OrderTotal: 50.00, OrderDate: now.AddDate(0, 0, -3), ProcessedAt: now.Add(-2 * time.Hour)},
		{OrderID: "ORDER-4", Provider: "amazon", Status: "success", OrderTotal: 75.00, OrderDate: now.AddDate(0, 0, -4), ProcessedAt: now.Add(-3 * time.Hour)},
	}

	for _, r := range records {
		err := store.SaveRecord(r)
		require.NoError(t, err)
	}

	t.Run("list all orders", func(t *testing.T) {
		result, err := store.ListOrders(OrderFilters{})
		require.NoError(t, err)
		assert.Equal(t, 4, result.TotalCount)
		assert.Len(t, result.Orders, 4)
	})

	t.Run("filter by provider", func(t *testing.T) {
		result, err := store.ListOrders(OrderFilters{Provider: "walmart"})
		require.NoError(t, err)
		assert.Equal(t, 2, result.TotalCount)
		for _, order := range result.Orders {
			assert.Equal(t, "walmart", order.Provider)
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		result, err := store.ListOrders(OrderFilters{Status: "success"})
		require.NoError(t, err)
		assert.Equal(t, 3, result.TotalCount)
		for _, order := range result.Orders {
			assert.Equal(t, "success", order.Status)
		}
	})

	t.Run("pagination", func(t *testing.T) {
		result, err := store.ListOrders(OrderFilters{Limit: 2, Offset: 0})
		require.NoError(t, err)
		assert.Equal(t, 4, result.TotalCount)
		assert.Len(t, result.Orders, 2)
		assert.Equal(t, 2, result.Limit)
		assert.Equal(t, 0, result.Offset)

		// Get second page
		result2, err := store.ListOrders(OrderFilters{Limit: 2, Offset: 2})
		require.NoError(t, err)
		assert.Len(t, result2.Orders, 2)
		assert.Equal(t, 2, result2.Offset)
	})

	t.Run("combined filters", func(t *testing.T) {
		result, err := store.ListOrders(OrderFilters{
			Provider: "walmart",
			Status:   "success",
		})
		require.NoError(t, err)
		assert.Equal(t, 1, result.TotalCount)
		assert.Equal(t, "ORDER-1", result.Orders[0].OrderID)
	})
}

func TestStorage_SearchItems(t *testing.T) {
	tmpDB := createTempDB(t)
	defer os.Remove(tmpDB)

	store, err := NewStorage(tmpDB)
	require.NoError(t, err)
	defer store.Close()

	// Setup: Create records with items
	record1 := &ProcessingRecord{
		OrderID:   "ORDER-1",
		Provider:  "walmart",
		OrderDate: time.Now(),
		Status:    "success",
		Items: []OrderItem{
			{Name: "Organic Milk", TotalPrice: 5.99, Category: "Groceries"},
			{Name: "Whole Wheat Bread", TotalPrice: 3.49, Category: "Groceries"},
		},
	}
	record2 := &ProcessingRecord{
		OrderID:   "ORDER-2",
		Provider:  "costco",
		OrderDate: time.Now().AddDate(0, 0, -1),
		Status:    "success",
		Items: []OrderItem{
			{Name: "Almond Milk", TotalPrice: 4.99, Category: "Groceries"},
			{Name: "Shampoo", TotalPrice: 8.99, Category: "Personal Care"},
		},
	}

	err = store.SaveRecord(record1)
	require.NoError(t, err)
	err = store.SaveRecord(record2)
	require.NoError(t, err)

	t.Run("search for milk", func(t *testing.T) {
		results, err := store.SearchItems("milk", 10)
		require.NoError(t, err)
		assert.Len(t, results, 2)

		// Check that both milk items are found
		names := make([]string, len(results))
		for i, r := range results {
			names[i] = r.ItemName
		}
		assert.Contains(t, names, "Organic Milk")
		assert.Contains(t, names, "Almond Milk")
	})

	t.Run("case insensitive search", func(t *testing.T) {
		results, err := store.SearchItems("BREAD", 10)
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "Whole Wheat Bread", results[0].ItemName)
	})

	t.Run("search with limit", func(t *testing.T) {
		results, err := store.SearchItems("milk", 1)
		require.NoError(t, err)
		assert.Len(t, results, 1)
	})

	t.Run("no matches", func(t *testing.T) {
		results, err := store.SearchItems("nonexistent", 10)
		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("result contains order info", func(t *testing.T) {
		results, err := store.SearchItems("shampoo", 10)
		require.NoError(t, err)
		require.Len(t, results, 1)

		result := results[0]
		assert.Equal(t, "ORDER-2", result.OrderID)
		assert.Equal(t, "costco", result.Provider)
		assert.Equal(t, "Shampoo", result.ItemName)
		assert.Equal(t, 8.99, result.ItemPrice)
		assert.Equal(t, "Personal Care", result.Category)
	})
}

func TestStorage_ListSyncRuns(t *testing.T) {
	tmpDB := createTempDB(t)
	defer os.Remove(tmpDB)

	store, err := NewStorage(tmpDB)
	require.NoError(t, err)
	defer store.Close()

	// Create several sync runs
	runID1, err := store.StartSyncRun("walmart", 14, false)
	require.NoError(t, err)

	runID2, err := store.StartSyncRun("costco", 7, true)
	require.NoError(t, err)

	// Complete the first run (no errors = "completed" status)
	err = store.CompleteSyncRun(runID1, 10, 9, 1, 0)
	require.NoError(t, err)

	t.Run("list all runs", func(t *testing.T) {
		runs, err := store.ListSyncRuns(10)
		require.NoError(t, err)
		assert.Len(t, runs, 2)
	})

	t.Run("runs contain expected data", func(t *testing.T) {
		runs, err := store.ListSyncRuns(10)
		require.NoError(t, err)

		// Find the completed walmart run
		var walmartRun *SyncRun
		for i := range runs {
			if runs[i].Provider == "walmart" {
				walmartRun = &runs[i]
				break
			}
		}
		require.NotNil(t, walmartRun)
		assert.Equal(t, runID1, walmartRun.ID)
		assert.Equal(t, 14, walmartRun.LookbackDays)
		assert.False(t, walmartRun.DryRun)
		assert.Equal(t, "completed", walmartRun.Status) // No errors = "completed"
		assert.Equal(t, 10, walmartRun.OrdersFound)
		assert.Equal(t, 9, walmartRun.OrdersProcessed)
	})

	t.Run("list with limit", func(t *testing.T) {
		runs, err := store.ListSyncRuns(1)
		require.NoError(t, err)
		assert.Len(t, runs, 1)
	})

	t.Run("running status for incomplete run", func(t *testing.T) {
		runs, err := store.ListSyncRuns(10)
		require.NoError(t, err)

		var costcoRun *SyncRun
		for i := range runs {
			if runs[i].Provider == "costco" {
				costcoRun = &runs[i]
				break
			}
		}
		require.NotNil(t, costcoRun)
		assert.Equal(t, runID2, costcoRun.ID)
		assert.Equal(t, "running", costcoRun.Status)
		assert.True(t, costcoRun.DryRun)
	})
}

func TestStorage_GetSyncRun(t *testing.T) {
	tmpDB := createTempDB(t)
	defer os.Remove(tmpDB)

	store, err := NewStorage(tmpDB)
	require.NoError(t, err)
	defer store.Close()

	// Create a sync run
	runID, err := store.StartSyncRun("walmart", 14, false)
	require.NoError(t, err)
	err = store.CompleteSyncRun(runID, 5, 4, 1, 0)
	require.NoError(t, err)

	t.Run("get existing run", func(t *testing.T) {
		run, err := store.GetSyncRun(runID)
		require.NoError(t, err)
		require.NotNil(t, run)
		assert.Equal(t, runID, run.ID)
		assert.Equal(t, "walmart", run.Provider)
		assert.Equal(t, 14, run.LookbackDays)
		assert.Equal(t, 5, run.OrdersFound)
		assert.Equal(t, 4, run.OrdersProcessed)
		assert.Equal(t, 1, run.OrdersSkipped)
		assert.Equal(t, 0, run.OrdersErrored)
		assert.Equal(t, "completed", run.Status)
	})

	t.Run("get non-existent run returns nil", func(t *testing.T) {
		run, err := store.GetSyncRun(9999)
		require.NoError(t, err)
		assert.Nil(t, run)
	})
}

// =============================================================================
// Mock Repository API Query Tests
// =============================================================================

func TestMockRepository_ListOrders(t *testing.T) {
	mock := NewMockRepository()

	mock.AddRecord(&ProcessingRecord{OrderID: "ORDER-1", Provider: "walmart", Status: "success"})
	mock.AddRecord(&ProcessingRecord{OrderID: "ORDER-2", Provider: "costco", Status: "success"})
	mock.AddRecord(&ProcessingRecord{OrderID: "ORDER-3", Provider: "walmart", Status: "failed"})

	t.Run("list all", func(t *testing.T) {
		result, err := mock.ListOrders(OrderFilters{})
		require.NoError(t, err)
		assert.Equal(t, 3, result.TotalCount)
	})

	t.Run("filter by provider", func(t *testing.T) {
		result, err := mock.ListOrders(OrderFilters{Provider: "walmart"})
		require.NoError(t, err)
		assert.Equal(t, 2, result.TotalCount)
	})

	t.Run("filter by status", func(t *testing.T) {
		result, err := mock.ListOrders(OrderFilters{Status: "success"})
		require.NoError(t, err)
		assert.Equal(t, 2, result.TotalCount)
	})
}

func TestMockRepository_SearchItems(t *testing.T) {
	mock := NewMockRepository()

	mock.AddRecord(&ProcessingRecord{
		OrderID:   "ORDER-1",
		Provider:  "walmart",
		OrderDate: time.Now(),
		Items: []OrderItem{
			{Name: "Milk", TotalPrice: 5.99, Category: "Groceries"},
			{Name: "Bread", TotalPrice: 2.99, Category: "Groceries"},
		},
	})

	t.Run("find item", func(t *testing.T) {
		results, err := mock.SearchItems("Milk", 10)
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "Milk", results[0].ItemName)
	})

	t.Run("case insensitive", func(t *testing.T) {
		results, err := mock.SearchItems("milk", 10)
		require.NoError(t, err)
		assert.Len(t, results, 1)
	})
}

func TestMockRepository_GetSyncRun(t *testing.T) {
	mock := NewMockRepository()

	runID, err := mock.StartSyncRun("walmart", 14, false)
	require.NoError(t, err)

	err = mock.CompleteSyncRun(runID, 10, 8, 1, 1)
	require.NoError(t, err)

	run, err := mock.GetSyncRun(runID)
	require.NoError(t, err)
	require.NotNil(t, run)
	assert.Equal(t, "walmart", run.Provider)
	assert.Equal(t, "completed", run.Status)
	assert.Equal(t, 10, run.OrdersFound)
}

// =============================================================================
// NULL Value Handling Tests
// =============================================================================
// These tests verify that records with NULL values in the database are handled
// correctly. This is critical because:
// 1. Legacy records may have NULL values from before we backfilled defaults
// 2. Some fields are intentionally nullable (error_message, transaction_id)
// 3. Go's sql.Scan fails if you scan NULL into a string without sql.NullString

func TestStorage_GetRecord_WithNullValues(t *testing.T) {
	tmpDB := createTempDB(t)
	defer os.Remove(tmpDB)

	store, err := NewStorage(tmpDB)
	require.NoError(t, err)
	defer store.Close()

	// Insert a record with intentional NULL values directly via SQL
	// This simulates legacy data or records where fields are genuinely NULL
	_, err = store.db.Exec(`
		INSERT INTO processing_records (
			order_id, provider, transaction_id, order_date, processed_at,
			order_total, order_subtotal, order_tax, order_tip, transaction_amount,
			split_count, status, error_message, item_count, match_confidence,
			dry_run, items_json, splits_json, multi_delivery_data
		) VALUES (
			'ORDER-NULL-TEST', 'walmart', NULL, datetime('now'), datetime('now'),
			100.00, 90.00, 10.00, 0, -100.00,
			2, 'success', NULL, 5, 0.95,
			0, NULL, NULL, NULL
		)
	`)
	require.NoError(t, err)

	// GetRecord should handle NULLs gracefully
	record, err := store.GetRecord("ORDER-NULL-TEST")
	require.NoError(t, err)
	require.NotNil(t, record)

	assert.Equal(t, "ORDER-NULL-TEST", record.OrderID)
	assert.Equal(t, "walmart", record.Provider)
	assert.Equal(t, "", record.TransactionID, "NULL transaction_id should become empty string")
	assert.Equal(t, "", record.ErrorMessage, "NULL error_message should become empty string")
	assert.Empty(t, record.Items, "NULL items_json should result in empty slice")
	assert.Empty(t, record.Splits, "NULL splits_json should result in empty slice")
	assert.Equal(t, "", record.MultiDeliveryData, "NULL multi_delivery_data should become empty string")
}

func TestStorage_ListOrders_WithNullValues(t *testing.T) {
	tmpDB := createTempDB(t)
	defer os.Remove(tmpDB)

	store, err := NewStorage(tmpDB)
	require.NoError(t, err)
	defer store.Close()

	// Insert records with various NULL combinations for intentionally nullable fields
	// Note: numeric fields must have values (migration backfills these in real data)
	// Only string fields like transaction_id, error_message, items_json can be NULL
	records := []struct {
		orderID       string
		transactionID interface{} // can be string or nil
		errorMessage  interface{}
		itemsJSON     interface{}
	}{
		{"ORDER-1", "txn-123", nil, `[{"name":"Item1","total_price":10.00}]`},
		{"ORDER-2", nil, "Some error", nil},
		{"ORDER-3", nil, nil, nil},
	}

	for _, r := range records {
		_, err := store.db.Exec(`
			INSERT INTO processing_records (
				order_id, provider, transaction_id, order_date, processed_at,
				order_total, order_subtotal, order_tax, order_tip, transaction_amount,
				split_count, status, error_message, item_count, match_confidence,
				dry_run, items_json, splits_json
			) VALUES (?, 'walmart', ?, datetime('now'), datetime('now'),
				50.00, 45.00, 5.00, 0, -50.00,
				0, 'success', ?, 0, 0.0,
				0, ?, '[]')
		`, r.orderID, r.transactionID, r.errorMessage, r.itemsJSON)
		require.NoError(t, err)
	}

	// ListOrders should handle all records without error
	result, err := store.ListOrders(OrderFilters{})
	require.NoError(t, err)
	assert.Equal(t, 3, result.TotalCount)

	// Find and verify each record
	orderMap := make(map[string]*ProcessingRecord)
	for _, order := range result.Orders {
		orderMap[order.OrderID] = order
	}

	// ORDER-1: has transaction_id and items, no error
	order1 := orderMap["ORDER-1"]
	require.NotNil(t, order1)
	assert.Equal(t, "txn-123", order1.TransactionID)
	assert.Equal(t, "", order1.ErrorMessage)
	assert.Len(t, order1.Items, 1)

	// ORDER-2: has error, no transaction_id or items
	order2 := orderMap["ORDER-2"]
	require.NotNil(t, order2)
	assert.Equal(t, "", order2.TransactionID)
	assert.Equal(t, "Some error", order2.ErrorMessage)
	assert.Empty(t, order2.Items)

	// ORDER-3: all nullable string fields are NULL
	order3 := orderMap["ORDER-3"]
	require.NotNil(t, order3)
	assert.Equal(t, "", order3.TransactionID)
	assert.Equal(t, "", order3.ErrorMessage)
	assert.Empty(t, order3.Items)
}

func TestStorage_SearchItems_WithNullItemsJSON(t *testing.T) {
	tmpDB := createTempDB(t)
	defer os.Remove(tmpDB)

	store, err := NewStorage(tmpDB)
	require.NoError(t, err)
	defer store.Close()

	// Insert a record with NULL items_json
	_, err = store.db.Exec(`
		INSERT INTO processing_records (
			order_id, provider, order_date, status, items_json
		) VALUES ('ORDER-NULL-ITEMS', 'walmart', datetime('now'), 'success', NULL)
	`)
	require.NoError(t, err)

	// Insert a record with valid items
	_, err = store.db.Exec(`
		INSERT INTO processing_records (
			order_id, provider, order_date, status, items_json
		) VALUES ('ORDER-WITH-ITEMS', 'walmart', datetime('now'), 'success',
			'[{"name":"Searchable Item","total_price":10.00}]')
	`)
	require.NoError(t, err)

	// SearchItems should skip NULL items_json and not error
	results, err := store.SearchItems("Searchable", 10)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "Searchable Item", results[0].ItemName)
}

func TestStorage_GetRecord_NonExistent(t *testing.T) {
	tmpDB := createTempDB(t)
	defer os.Remove(tmpDB)

	store, err := NewStorage(tmpDB)
	require.NoError(t, err)
	defer store.Close()

	// Should return nil, nil for non-existent record
	record, err := store.GetRecord("DOES-NOT-EXIST")
	require.NoError(t, err)
	assert.Nil(t, record)
}
