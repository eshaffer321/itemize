package target

import (
	"path/filepath"
	"testing"
	"time"

	targetgo "github.com/daslicious/target-go"
	"github.com/eshaffer321/itemize/internal/infrastructure/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Bug (2026-10-11): Target timestamps carry a numeric offset ("…-05:00"), which Go formats with a
// numeric zone name ("2026-10-06 23:17:42 -0500 -0500"). The SQLite driver can't parse that back,
// so every later run failed on the order with "load processing record: sql: Scan error on column
// index 4, name "order_date"".
func TestOrderDatesSurviveTheDedupDB(t *testing.T) {
	store, err := storage.NewStorage(filepath.Join(t.TempDir(), "sync.db"))
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	var receipt targetgo.StoreOrder
	var online targetgo.OnlineOrder
	loadJSON(t, "sample_store_order.json", &receipt)
	loadJSON(t, "sample_online_order.json", &online)

	storeOrder, err := convertStore(&receipt, targetgo.OrderSummary{})
	require.NoError(t, err)
	onlineOrder, err := convertOnline(&online, targetgo.OrderSummary{})
	require.NoError(t, err)

	for _, order := range []*Order{storeOrder, onlineOrder} {
		require.NoError(t, store.SaveRecord(&storage.ProcessingRecord{
			OrderID:     order.GetID(),
			Provider:    order.GetProviderName(),
			OrderDate:   order.GetDate(),
			ProcessedAt: time.Now(),
			OrderTotal:  order.GetTotal(),
			Status:      "dry-run",
			DryRun:      true,
		}))
		record, err := store.GetRecord(order.GetID())
		require.NoError(t, err, order.GetID())
		require.NotNil(t, record)
		assert.True(t, record.OrderDate.Equal(order.GetDate()), "%s: stored %v, order %v", order.GetID(), record.OrderDate, order.GetDate())
	}
}
