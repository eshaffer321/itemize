package storage

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A time parsed from an offset that isn't the local zone's (e.g. a Walmart
// "2026-10-31T19:11:53-07:00" read on a machine in MDT) carries an unnamed
// fixed zone. time.Time.String renders that as "-0700 -0700", which the
// SQLite driver can't parse back, so GetRecord failed with a Scan error.
func TestStorage_SaveRecord_UnnamedFixedZoneRoundTrips(t *testing.T) {
	tmpDB := createTempDB(t)
	defer os.Remove(tmpDB)

	store, err := NewStorage(tmpDB)
	require.NoError(t, err)
	defer store.Close()

	orderDate := time.Date(2026, 10, 31, 19, 11, 53, 0, time.FixedZone("", -7*3600))
	record := &ProcessingRecord{
		OrderID:     "02796073446817290005",
		Provider:    "walmart",
		OrderDate:   orderDate,
		ProcessedAt: time.Now().Truncate(time.Second),
		Status:      "success",
	}
	require.NoError(t, store.SaveRecord(record))

	got, err := store.GetRecord(record.OrderID)
	require.NoError(t, err)
	assert.True(t, got.OrderDate.Equal(orderDate), "got %v, want %v", got.OrderDate, orderDate)

	attempts, err := store.GetAttemptsByOrderID(record.OrderID)
	require.NoError(t, err)
	require.Len(t, attempts, 1)
	assert.True(t, attempts[0].OrderDate.Equal(orderDate))
}

// Rows already written in the unparseable "-0700 -0700" form must be
// repaired by a migration so they can be loaded again.
func TestMigrations_RepairUnnamedZoneTimestamps(t *testing.T) {
	tmpDB := createTempDB(t)
	defer os.Remove(tmpDB)

	store, err := NewStorage(tmpDB)
	require.NoError(t, err)

	_, err = store.db.Exec(`INSERT INTO processing_records (order_id, provider, order_date, processed_at, status, items_json, splits_json,
		 order_total, order_subtotal, order_tax, order_tip, transaction_amount, split_count, item_count, match_confidence)
		VALUES ('legacy', 'walmart', '2026-10-31 19:11:53 -0700 -0700', '2026-11-03 20:24:55.856105 -0700 -0700 m=+92.443189668', 'success', '[]', '[]',
		 5.90, 5.50, 0.40, 0, -5.90, 1, 1, 1)`)
	require.NoError(t, err)
	// Re-run migration 00011 against the legacy row on the next open.
	_, err = store.db.Exec(`DELETE FROM goose_db_version WHERE version_id = 11`)
	require.NoError(t, err)
	require.NoError(t, store.Close())

	store, err = NewStorage(tmpDB)
	require.NoError(t, err)
	defer store.Close()

	got, err := store.GetRecord("legacy")
	require.NoError(t, err)
	want := time.Date(2026, 10, 31, 19, 11, 53, 0, time.FixedZone("", -7*3600))
	assert.True(t, got.OrderDate.Equal(want), "got %v, want %v", got.OrderDate, want)
	wantProcessed := time.Date(2026, 11, 3, 20, 24, 55, 856105000, time.FixedZone("", -7*3600))
	assert.True(t, got.ProcessedAt.Equal(wantProcessed), "got %v, want %v", got.ProcessedAt, wantProcessed)
}
