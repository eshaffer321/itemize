package storage

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
