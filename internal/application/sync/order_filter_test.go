package sync

import (
	"testing"

	"github.com/eshaffer321/itemize/internal/infrastructure/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Regression: an -order-id dry run for an order with an exchanged item matched
// the $34.42 charge that the exchange's replacement order had already
// itemized, because orders left out by -order-id never reserve their
// transactions. A real run would have overwritten that charge's notes.
func TestReserveClaimedTransactions_OrderIDRunReservesOtherOrdersCharges(t *testing.T) {
	store := storage.NewMockRepository()
	require.NoError(t, store.SaveRecord(&storage.ProcessingRecord{
		OrderID: "112-2222222-2222222", Status: "success", TransactionID: "replacement-charge",
	}))
	require.NoError(t, store.SaveRecord(&storage.ProcessingRecord{
		OrderID: "112-1111111-1111111", Status: "success", TransactionID: "own-charge",
	}))
	orch := &Orchestrator{storage: store, logger: reconciliationTestLogger()}
	used := make(map[string]bool)

	require.NoError(t, orch.reserveClaimedTransactions(Options{OrderID: "112-1111111-1111111"}, used))

	assert.Equal(t, map[string]bool{"replacement-charge": true}, used)
}

func TestReserveClaimedTransactions_FullRunLeavesReservationToReconciliation(t *testing.T) {
	store := storage.NewMockRepository()
	require.NoError(t, store.SaveRecord(&storage.ProcessingRecord{
		OrderID: "112-2222222-2222222", Status: "success", TransactionID: "replacement-charge",
	}))
	orch := &Orchestrator{storage: store, logger: reconciliationTestLogger()}
	used := make(map[string]bool)

	require.NoError(t, orch.reserveClaimedTransactions(Options{}, used))

	assert.Empty(t, used)
}
