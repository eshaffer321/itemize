package sync

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/eshaffer321/itemize/internal/application/sync/handlers"
	"github.com/eshaffer321/itemize/internal/infrastructure/storage"
	"github.com/eshaffer321/monarch-go/v2/pkg/monarch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type reconciliationTestClient struct {
	detailsByID map[string]*monarch.TransactionDetails
	splitsByID  map[string][]*monarch.TransactionSplit
	getErr      error

	updatedTransactionID string
	updatedParams        *monarch.UpdateTransactionParams
	updatedSplitsID      string
	updatedSplits        []*monarch.TransactionSplit
}

func (c *reconciliationTestClient) GetTransaction(_ context.Context, id string) (*monarch.TransactionDetails, error) {
	if c.getErr != nil {
		return nil, c.getErr
	}
	return c.detailsByID[id], nil
}

func (c *reconciliationTestClient) GetSplits(_ context.Context, id string) ([]*monarch.TransactionSplit, error) {
	return c.splitsByID[id], nil
}

func (c *reconciliationTestClient) UpdateTransaction(_ context.Context, id string, params *monarch.UpdateTransactionParams) error {
	c.updatedTransactionID = id
	c.updatedParams = params
	return nil
}

func (c *reconciliationTestClient) UpdateSplits(_ context.Context, id string, splits []*monarch.TransactionSplit) error {
	c.updatedSplitsID = id
	c.updatedSplits = splits
	return nil
}

func reconciliationTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func reconciliationTestOrder(id string, date time.Time, total float64) *mockSimpleOrder {
	return &mockSimpleOrder{
		id:           id,
		date:         date,
		total:        total,
		subtotal:     total,
		providerName: "Amazon",
	}
}

func TestRecordSuccessWithResult_PendingTransactionIsProvisional(t *testing.T) {
	store := storage.NewMockRepository()
	orch := &Orchestrator{storage: store, logger: reconciliationTestLogger()}
	order := reconciliationTestOrder("ORDER-PENDING", time.Now(), 25.75)
	transaction := &monarch.Transaction{
		ID:      "pending-txn",
		Amount:  -25.75,
		Pending: true,
	}
	result := &handlers.ProcessResult{
		Processed:    true,
		Transaction:  transaction,
		CategoryID:   "auto",
		CategoryName: "Auto Maintenance",
		MonarchNotes: "Auto Maintenance:\n- Air filter $25.75",
	}

	orch.recordSuccessWithResult(order, transaction, nil, 1, false, result, nil)

	record, err := store.GetRecord(order.GetID())
	require.NoError(t, err)
	require.NotNil(t, record)
	assert.Equal(t, "provisional", record.Status)
	assert.False(t, store.IsProcessed(order.GetID()))
}

func TestProcessOrder_ReconcilesRedirectedSingleCategoryFromCachedRecord(t *testing.T) {
	orderDate := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	order := reconciliationTestOrder("ORDER-AIR-FILTER", orderDate, 25.75)
	store := storage.NewMockRepository()
	store.AddRecord(&storage.ProcessingRecord{
		OrderID:           order.GetID(),
		Provider:          "Amazon",
		TransactionID:     "pending-txn",
		OrderDate:         orderDate,
		ProcessedAt:       orderDate,
		TransactionAmount: -25.75,
		Status:            "provisional",
		CategoryID:        "auto",
		CategoryName:      "Auto Maintenance",
		MonarchNotes:      "Auto Maintenance:\n- Air filter $25.75",
	})
	client := &reconciliationTestClient{
		detailsByID: map[string]*monarch.TransactionDetails{
			"pending-txn": {
				Transaction: &monarch.Transaction{
					ID:      "posted-txn",
					Amount:  -25.75,
					Pending: false,
					Category: &monarch.TransactionCategory{
						ID:   "amazon-temp",
						Name: "[TEMP] Amazon",
					},
				},
			},
		},
	}
	orch := &Orchestrator{
		storage:              store,
		reconciliationClient: client,
		logger:               reconciliationTestLogger(),
	}
	used := make(map[string]bool)

	processed, skipped, err := orch.processOrder(
		context.Background(),
		order,
		[]*monarch.Transaction{client.detailsByID["pending-txn"].Transaction},
		used,
		nil,
		nil,
		Options{},
	)

	require.NoError(t, err)
	assert.True(t, processed)
	assert.False(t, skipped)
	assert.Equal(t, "posted-txn", client.updatedTransactionID)
	require.NotNil(t, client.updatedParams)
	require.NotNil(t, client.updatedParams.CategoryID)
	assert.Equal(t, "auto", *client.updatedParams.CategoryID)
	require.NotNil(t, client.updatedParams.Notes)
	assert.Contains(t, *client.updatedParams.Notes, "Air filter")
	assert.True(t, used["posted-txn"])

	record, getErr := store.GetRecord(order.GetID())
	require.NoError(t, getErr)
	assert.Equal(t, "success", record.Status)
	assert.Equal(t, "posted-txn", record.TransactionID)
}

func TestProcessOrder_ReconcilesRedirectedSplitsFromCachedRecord(t *testing.T) {
	orderDate := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	order := reconciliationTestOrder("ORDER-SPLIT", orderDate, 58.28)
	store := storage.NewMockRepository()
	store.AddRecord(&storage.ProcessingRecord{
		OrderID:           order.GetID(),
		Provider:          "Amazon",
		TransactionID:     "pending-split",
		OrderDate:         orderDate,
		ProcessedAt:       orderDate,
		TransactionAmount: -58.28,
		SplitCount:        2,
		Status:            "success",
		Splits: []storage.SplitDetail{
			{CategoryID: "home", Amount: -33.57, Notes: "Home Spending:\n- Vase $33.57"},
			{CategoryID: "household", Amount: -24.71, Notes: "Household Supplies:\n- Dispenser $24.71"},
		},
	})
	client := &reconciliationTestClient{
		detailsByID: map[string]*monarch.TransactionDetails{
			"pending-split": {
				Transaction: &monarch.Transaction{
					ID:      "posted-split",
					Amount:  -58.28,
					Pending: false,
				},
			},
		},
	}
	orch := &Orchestrator{
		storage:              store,
		reconciliationClient: client,
		logger:               reconciliationTestLogger(),
	}

	processed, skipped, err := orch.processOrder(
		context.Background(),
		order,
		[]*monarch.Transaction{client.detailsByID["pending-split"].Transaction},
		make(map[string]bool),
		nil,
		nil,
		Options{},
	)

	require.NoError(t, err)
	assert.True(t, processed)
	assert.False(t, skipped)
	assert.Equal(t, "posted-split", client.updatedSplitsID)
	require.Len(t, client.updatedSplits, 2)
	assert.Equal(t, "home", client.updatedSplits[0].CategoryID)
	assert.Equal(t, -33.57, client.updatedSplits[0].Amount)
	assert.Empty(t, client.updatedTransactionID)
}

func TestProcessOrder_DoesNotOverwriteSameIDFinalManualCategory(t *testing.T) {
	orderDate := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	order := reconciliationTestOrder("ORDER-MANUAL", orderDate, 25.75)
	store := storage.NewMockRepository()
	store.AddRecord(&storage.ProcessingRecord{
		OrderID:           order.GetID(),
		Provider:          "Amazon",
		TransactionID:     "posted-txn",
		OrderDate:         orderDate,
		ProcessedAt:       orderDate,
		TransactionAmount: -25.75,
		Status:            "success",
		CategoryID:        "auto",
		CategoryName:      "Auto Maintenance",
	})
	client := &reconciliationTestClient{
		detailsByID: map[string]*monarch.TransactionDetails{
			"posted-txn": {
				Transaction: &monarch.Transaction{
					ID:      "posted-txn",
					Amount:  -25.75,
					Pending: false,
					Category: &monarch.TransactionCategory{
						ID:   "manual",
						Name: "His Fund",
					},
				},
			},
		},
	}
	orch := &Orchestrator{
		storage:              store,
		reconciliationClient: client,
		logger:               reconciliationTestLogger(),
	}

	processed, skipped, err := orch.processOrder(
		context.Background(),
		order,
		[]*monarch.Transaction{client.detailsByID["posted-txn"].Transaction},
		make(map[string]bool),
		nil,
		nil,
		Options{},
	)

	require.NoError(t, err)
	assert.False(t, processed)
	assert.True(t, skipped)
	assert.Empty(t, client.updatedTransactionID)
	assert.Empty(t, client.updatedSplitsID)
}

func TestProcessOrder_LegacySuccessStillPendingBecomesProvisional(t *testing.T) {
	orderDate := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	order := reconciliationTestOrder("ORDER-LEGACY-PENDING", orderDate, 37.58)
	store := storage.NewMockRepository()
	store.AddRecord(&storage.ProcessingRecord{
		OrderID:           order.GetID(),
		Provider:          "Amazon",
		TransactionID:     "pending-txn",
		OrderDate:         orderDate,
		ProcessedAt:       orderDate,
		TransactionAmount: -37.58,
		Status:            "success",
		CategoryID:        "kid-needs",
		CategoryName:      "Kid Needs",
		MonarchNotes:      "Kid Needs:\n- Pajamas $37.58",
	})
	client := &reconciliationTestClient{
		detailsByID: map[string]*monarch.TransactionDetails{
			"pending-txn": {
				Transaction: &monarch.Transaction{
					ID:      "pending-txn",
					Amount:  -37.58,
					Pending: true,
				},
			},
		},
	}
	orch := &Orchestrator{
		storage:              store,
		reconciliationClient: client,
		logger:               reconciliationTestLogger(),
	}

	processed, skipped, err := orch.processOrder(
		context.Background(),
		order,
		nil,
		make(map[string]bool),
		nil,
		nil,
		Options{},
	)

	require.NoError(t, err)
	assert.False(t, processed)
	assert.True(t, skipped)
	record, getErr := store.GetRecord(order.GetID())
	require.NoError(t, getErr)
	assert.Equal(t, "provisional", record.Status)
	assert.False(t, store.IsProcessed(order.GetID()))
	assert.Empty(t, client.updatedTransactionID)
}

func TestProcessOrder_FinalizesProvisionalWhenPostedStateAlreadyMatches(t *testing.T) {
	orderDate := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	order := reconciliationTestOrder("ORDER-ALREADY-CORRECT", orderDate, 37.58)
	notes := "Kid Needs:\n- Pajamas $37.58"
	store := storage.NewMockRepository()
	store.AddRecord(&storage.ProcessingRecord{
		OrderID:           order.GetID(),
		Provider:          "Amazon",
		TransactionID:     "same-txn",
		OrderDate:         orderDate,
		ProcessedAt:       orderDate,
		TransactionAmount: -37.58,
		Status:            "provisional",
		CategoryID:        "kid-needs",
		CategoryName:      "Kid Needs",
		MonarchNotes:      notes,
	})
	client := &reconciliationTestClient{
		detailsByID: map[string]*monarch.TransactionDetails{
			"same-txn": {
				Transaction: &monarch.Transaction{
					ID:      "same-txn",
					Amount:  -37.58,
					Pending: false,
					Notes:   notes,
					Category: &monarch.TransactionCategory{
						ID:   "kid-needs",
						Name: "Kid Needs",
					},
				},
			},
		},
	}
	orch := &Orchestrator{
		storage:              store,
		reconciliationClient: client,
		logger:               reconciliationTestLogger(),
	}

	processed, skipped, err := orch.processOrder(
		context.Background(),
		order,
		nil,
		make(map[string]bool),
		nil,
		nil,
		Options{},
	)

	require.NoError(t, err)
	assert.True(t, processed)
	assert.False(t, skipped)
	assert.Empty(t, client.updatedTransactionID)
	record, getErr := store.GetRecord(order.GetID())
	require.NoError(t, getErr)
	assert.Equal(t, "success", record.Status)
}

func TestProcessOrder_UsesUniquePostedFallbackWhenRedirectIsMissing(t *testing.T) {
	orderDate := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	order := reconciliationTestOrder("ORDER-FALLBACK", orderDate, 25.75)
	store := storage.NewMockRepository()
	store.AddRecord(&storage.ProcessingRecord{
		OrderID:           order.GetID(),
		Provider:          "Amazon",
		TransactionID:     "missing-pending",
		OrderDate:         orderDate,
		ProcessedAt:       orderDate,
		TransactionAmount: -25.75,
		Status:            "provisional",
		CategoryID:        "auto",
		CategoryName:      "Auto Maintenance",
		MonarchNotes:      "Auto Maintenance:\n- Air filter $25.75",
	})
	client := &reconciliationTestClient{getErr: monarch.ErrNotFound}
	posted := &monarch.Transaction{
		ID:      "unique-posted",
		Amount:  -25.75,
		Date:    monarch.Date{Time: orderDate.AddDate(0, 0, 1)},
		Pending: false,
		Category: &monarch.TransactionCategory{
			ID:   "amazon-temp",
			Name: "[TEMP] Amazon",
		},
	}
	orch := &Orchestrator{
		storage:              store,
		reconciliationClient: client,
		logger:               reconciliationTestLogger(),
	}

	processed, skipped, err := orch.processOrder(
		context.Background(),
		order,
		[]*monarch.Transaction{posted},
		make(map[string]bool),
		nil,
		nil,
		Options{},
	)

	require.NoError(t, err)
	assert.True(t, processed)
	assert.False(t, skipped)
	assert.Equal(t, "unique-posted", client.updatedTransactionID)
	record, getErr := store.GetRecord(order.GetID())
	require.NoError(t, getErr)
	assert.Equal(t, "unique-posted", record.TransactionID)
	assert.Equal(t, "success", record.Status)
}

func TestProcessOrder_AmbiguousPostedFallbackMakesNoWrite(t *testing.T) {
	orderDate := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	order := reconciliationTestOrder("ORDER-AMBIGUOUS", orderDate, 11.01)
	store := storage.NewMockRepository()
	store.AddRecord(&storage.ProcessingRecord{
		OrderID:           order.GetID(),
		Provider:          "Amazon",
		TransactionID:     "missing-pending",
		OrderDate:         orderDate,
		ProcessedAt:       orderDate,
		TransactionAmount: -11.01,
		Status:            "provisional",
		CategoryID:        "kid-needs",
		CategoryName:      "Kid Needs",
	})
	client := &reconciliationTestClient{getErr: monarch.ErrNotFound}
	transactions := []*monarch.Transaction{
		{ID: "posted-one", Amount: -11.01, Date: monarch.Date{Time: orderDate}, Pending: false},
		{ID: "posted-two", Amount: -11.01, Date: monarch.Date{Time: orderDate}, Pending: false},
	}
	orch := &Orchestrator{
		storage:              store,
		reconciliationClient: client,
		logger:               reconciliationTestLogger(),
	}

	processed, skipped, err := orch.processOrder(
		context.Background(),
		order,
		transactions,
		make(map[string]bool),
		nil,
		nil,
		Options{},
	)

	require.NoError(t, err)
	assert.False(t, processed)
	assert.True(t, skipped)
	assert.Empty(t, client.updatedTransactionID)
	assert.Empty(t, client.updatedSplitsID)
	record, getErr := store.GetRecord(order.GetID())
	require.NoError(t, getErr)
	assert.Equal(t, "provisional", record.Status)
	assert.Equal(t, "missing-pending", record.TransactionID)
}

func TestProcessOrder_RepairsSameIDTemporaryCategoryButNotOtherCategories(t *testing.T) {
	orderDate := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	order := reconciliationTestOrder("ORDER-SAME-ID-TEMP", orderDate, 25.75)
	store := storage.NewMockRepository()
	store.AddRecord(&storage.ProcessingRecord{
		OrderID:           order.GetID(),
		Provider:          "Amazon",
		TransactionID:     "posted-txn",
		OrderDate:         orderDate,
		ProcessedAt:       orderDate,
		TransactionAmount: -25.75,
		Status:            "success",
		CategoryID:        "auto",
		CategoryName:      "Auto Maintenance",
		MonarchNotes:      "Auto Maintenance:\n- Air filter $25.75",
	})
	client := &reconciliationTestClient{
		detailsByID: map[string]*monarch.TransactionDetails{
			"posted-txn": {
				Transaction: &monarch.Transaction{
					ID:      "posted-txn",
					Amount:  -25.75,
					Pending: false,
					Category: &monarch.TransactionCategory{
						ID:   "amazon-temp",
						Name: "[TEMP] Amazon",
					},
				},
			},
		},
	}
	orch := &Orchestrator{
		storage:              store,
		reconciliationClient: client,
		logger:               reconciliationTestLogger(),
	}

	processed, skipped, err := orch.processOrder(
		context.Background(),
		order,
		nil,
		make(map[string]bool),
		nil,
		nil,
		Options{},
	)

	require.NoError(t, err)
	assert.True(t, processed)
	assert.False(t, skipped)
	assert.Equal(t, "posted-txn", client.updatedTransactionID)
	require.NotNil(t, client.updatedParams)
	require.NotNil(t, client.updatedParams.CategoryID)
	assert.Equal(t, "auto", *client.updatedParams.CategoryID)
}

func TestProcessOrder_ReconciliationDryRunMakesNoWrites(t *testing.T) {
	orderDate := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	order := reconciliationTestOrder("ORDER-DRY-RUN", orderDate, 25.75)
	store := storage.NewMockRepository()
	store.AddRecord(&storage.ProcessingRecord{
		OrderID:           order.GetID(),
		Provider:          "Amazon",
		TransactionID:     "pending-txn",
		OrderDate:         orderDate,
		ProcessedAt:       orderDate,
		TransactionAmount: -25.75,
		Status:            "provisional",
		CategoryID:        "auto",
		CategoryName:      "Auto Maintenance",
	})
	client := &reconciliationTestClient{
		detailsByID: map[string]*monarch.TransactionDetails{
			"pending-txn": {
				Transaction: &monarch.Transaction{
					ID:      "posted-txn",
					Amount:  -25.75,
					Pending: false,
					Category: &monarch.TransactionCategory{
						ID:   "amazon-temp",
						Name: "[TEMP] Amazon",
					},
				},
			},
		},
	}
	orch := &Orchestrator{
		storage:              store,
		reconciliationClient: client,
		logger:               reconciliationTestLogger(),
	}

	processed, skipped, err := orch.processOrder(
		context.Background(),
		order,
		nil,
		make(map[string]bool),
		nil,
		nil,
		Options{DryRun: true},
	)

	require.NoError(t, err)
	assert.True(t, processed)
	assert.False(t, skipped)
	assert.Empty(t, client.updatedTransactionID)
	assert.Empty(t, client.updatedSplitsID)
	record, getErr := store.GetRecord(order.GetID())
	require.NoError(t, getErr)
	assert.Equal(t, "provisional", record.Status)
	assert.Equal(t, "pending-txn", record.TransactionID)
}
