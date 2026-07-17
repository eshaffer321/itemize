package sync

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/eshaffer321/itemize/internal/adapters/providers"
	amazonprovider "github.com/eshaffer321/itemize/internal/adapters/providers/amazon"
	"github.com/eshaffer321/itemize/internal/application/sync/handlers"
	"github.com/eshaffer321/itemize/internal/domain/categorizer"
	"github.com/eshaffer321/itemize/internal/domain/matcher"
	"github.com/eshaffer321/itemize/internal/domain/splitter"
	"github.com/eshaffer321/itemize/internal/infrastructure/storage"
	"github.com/eshaffer321/monarch-go/v2/pkg/monarch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingRefundSplitter struct{}

func (failingRefundSplitter) CreateSplits(context.Context, providers.Order, *monarch.Transaction, []categorizer.Category, []*monarch.TransactionCategory) ([]*monarch.TransactionSplit, error) {
	return nil, errors.New("categorizer unavailable")
}

func (failingRefundSplitter) GetSingleCategoryInfo(context.Context, providers.Order, []categorizer.Category) (string, string, error) {
	return "", "", errors.New("categorizer unavailable")
}

func TestFetchAmazonReturnsUsesProviderCapability(t *testing.T) {
	records := []amazonprovider.ReturnRecord{{RMAID: "DfetchRRMA", RefundAmount: 12.34}}
	provider := &MockProvider{returnRecords: records}
	orchestrator := &Orchestrator{provider: provider, logger: slog.Default()}

	got, err := orchestrator.fetchAmazonReturns(context.Background())
	require.NoError(t, err)
	assert.Equal(t, records, got)

	provider.returnErr = errors.New("return center unavailable")
	got, err = orchestrator.fetchAmazonReturns(context.Background())
	assert.Nil(t, got)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch Amazon returns")
}

func TestProcessAmazonReturnsIncludesCalendarDayAtLookbackBoundary(t *testing.T) {
	now := time.Date(2026, time.July, 17, 15, 30, 0, 0, time.UTC)
	issuedAt := time.Date(2026, time.July, 3, 0, 0, 0, 0, time.UTC)
	records := []amazonprovider.ReturnRecord{{
		OrderID:        "112-1111111-2222222",
		RMAID:          "D8ExampleRRMA",
		RefundAmount:   11.36,
		HasRefundTotal: true,
		RefundIssuedAt: &issuedAt,
		Items:          []amazonprovider.ReturnedItem{{ASIN: "B0EXAMPLE1", Name: "Insulated Sporty Cup", Price: 10.72}},
	}}
	temporary := &monarch.TransactionCategory{ID: "temp-amazon", Name: "[TEMP] Amazon"}
	transactions := []*monarch.Transaction{{
		ID: "refund-credit", Amount: 11.36, Date: toMonarchDate(issuedAt), Category: temporary,
	}}
	transactionMatcher := matcher.NewMatcher(matcher.Config{AmountTolerance: 0.01, DateTolerance: 5})
	spl := splitter.NewSplitter(&mockCategorizer{categoryID: "kid-needs", categoryName: "Kid Needs"})
	orchestrator := &Orchestrator{
		amazonHandler: handlers.NewAmazonHandler(transactionMatcher, nil, &mockSplitterAdapter{splitter: spl}, &processOrderTestMonarch{}, slog.Default()),
		logger:        slog.Default(),
	}
	result := &Result{}

	orchestrator.processAmazonReturns(context.Background(), records, transactions, map[string]bool{}, []categorizer.Category{{ID: "kid-needs", Name: "Kid Needs"}}, nil, Options{DryRun: true, LookbackDays: 14}, now, result)

	assert.Equal(t, 1, result.RefundProcessedCount)
	assert.Equal(t, 0, result.RefundSkippedCount)
}

func TestReturnFallsWithinLookbackUsesIssuedDateNotCreatedDate(t *testing.T) {
	now := time.Date(2026, time.July, 17, 15, 30, 0, 0, time.UTC)
	createdAt := time.Date(2026, time.June, 14, 0, 0, 0, 0, time.UTC)
	issuedAt := time.Date(2026, time.July, 3, 0, 0, 0, 0, time.UTC)
	record := amazonprovider.ReturnRecord{CreatedAt: createdAt, RefundIssuedAt: &issuedAt}

	assert.True(t, returnFallsWithinLookback(record, 14, now))
}

func TestProcessAmazonReturnsGroupsIndistinguishableCredits(t *testing.T) {
	now := time.Date(2026, time.July, 17, 15, 30, 0, 0, time.UTC)
	issuedAt := time.Date(2026, time.July, 3, 0, 0, 0, 0, time.UTC)
	records := []amazonprovider.ReturnRecord{
		{OrderID: "112-1111111-2222222", RMAID: "DfirstRRMA", RefundAmount: 14.41, HasRefundTotal: true, RefundIssuedAt: &issuedAt, Items: []amazonprovider.ReturnedItem{{ASIN: "B0CUPONE", Name: "Sesame Street Kids Cup", Price: 13.59}}},
		{OrderID: "112-1111111-2222222", RMAID: "DsecondRRMA", RefundAmount: 14.41, HasRefundTotal: true, RefundIssuedAt: &issuedAt, Items: []amazonprovider.ReturnedItem{{ASIN: "B0CUPTWO", Name: "Disney Kids Cup", Price: 13.59}}},
	}
	temporary := &monarch.TransactionCategory{ID: "temp-amazon", Name: "[TEMP] Amazon"}
	transactions := []*monarch.Transaction{
		{ID: "refund-1", Amount: 14.41, Date: toMonarchDate(issuedAt), Category: temporary},
		{ID: "refund-2", Amount: 14.41, Date: toMonarchDate(issuedAt), Category: temporary},
	}
	transactionMatcher := matcher.NewMatcher(matcher.Config{AmountTolerance: 0.01, DateTolerance: 5})
	spl := splitter.NewSplitter(&mockCategorizer{categoryID: "kid-needs", categoryName: "Kid Needs"})
	orchestrator := &Orchestrator{
		amazonHandler: handlers.NewAmazonHandler(transactionMatcher, nil, &mockSplitterAdapter{splitter: spl}, &processOrderTestMonarch{}, slog.Default()),
		logger:        slog.Default(),
		storage:       storage.NewMockRepository(),
	}
	result := &Result{}

	orchestrator.processAmazonReturns(context.Background(), records, transactions, map[string]bool{}, []categorizer.Category{{ID: "kid-needs", Name: "Kid Needs"}}, nil, Options{LookbackDays: 14}, now, result)

	assert.Equal(t, 2, result.RefundProcessedCount)
	assert.Equal(t, 0, result.RefundSkippedCount)
	assert.True(t, orchestrator.storage.IsProcessed("amazon-refund:DfirstRRMA"))
	assert.True(t, orchestrator.storage.IsProcessed("amazon-refund:DsecondRRMA"))
	for _, refundID := range []string{"amazon-refund:DfirstRRMA", "amazon-refund:DsecondRRMA"} {
		associations, err := orchestrator.storage.GetOrderTransactions(refundID)
		require.NoError(t, err)
		assert.Empty(t, associations, "the indistinguishable group must not assert a credit-to-item mapping")
	}
}

func TestProcessAmazonReturnsLeavesUnmatchedRefundUntouched(t *testing.T) {
	now := time.Date(2026, time.July, 17, 15, 30, 0, 0, time.UTC)
	issuedAt := time.Date(2026, time.July, 10, 0, 0, 0, 0, time.UTC)
	record := amazonprovider.ReturnRecord{
		OrderID: "112-1111111-2222222", RMAID: "DunmatchedRRMA", RefundAmount: 9.99,
		HasRefundTotal: true, RefundIssuedAt: &issuedAt,
		Items: []amazonprovider.ReturnedItem{{ASIN: "B0UNMATCHED", Name: "Unmatched item", Price: 9.99}},
	}
	handler := handlers.NewAmazonHandler(matcher.NewMatcher(matcher.Config{AmountTolerance: 0.01, DateTolerance: 5}), nil, failingRefundSplitter{}, &processOrderTestMonarch{}, slog.Default())
	orchestrator := &Orchestrator{amazonHandler: handler, logger: slog.Default()}
	result := &Result{}

	orchestrator.processAmazonReturns(context.Background(), []amazonprovider.ReturnRecord{record}, nil, map[string]bool{}, nil, nil, Options{DryRun: true, LookbackDays: 14}, now, result)

	assert.Equal(t, 1, result.RefundSkippedCount)
	assert.Zero(t, result.ErrorCount)
}

func TestProcessAmazonReturnsReportsCategorizationAndGroupCardinalityFailures(t *testing.T) {
	now := time.Date(2026, time.July, 17, 15, 30, 0, 0, time.UTC)
	issuedAt := time.Date(2026, time.July, 10, 0, 0, 0, 0, time.UTC)
	makeRecord := func(rma, asin string) amazonprovider.ReturnRecord {
		return amazonprovider.ReturnRecord{
			OrderID: "112-1111111-2222222", RMAID: rma, RefundAmount: 14.41,
			HasRefundTotal: true, RefundIssuedAt: &issuedAt,
			Items: []amazonprovider.ReturnedItem{{ASIN: asin, Name: "Returned item", Price: 14.41}},
		}
	}
	temporary := &monarch.TransactionCategory{ID: "temp-amazon", Name: "[TEMP] Amazon"}
	transaction := &monarch.Transaction{ID: "credit", Amount: 14.41, Date: toMonarchDate(issuedAt), Category: temporary}
	handler := handlers.NewAmazonHandler(matcher.NewMatcher(matcher.Config{AmountTolerance: 0.01, DateTolerance: 5}), nil, failingRefundSplitter{}, &processOrderTestMonarch{}, slog.Default())
	orchestrator := &Orchestrator{amazonHandler: handler, logger: slog.Default()}

	singleResult := &Result{}
	orchestrator.processAmazonReturns(context.Background(), []amazonprovider.ReturnRecord{makeRecord("DsingleRRMA", "B0SINGLE")}, []*monarch.Transaction{transaction}, map[string]bool{}, nil, nil, Options{DryRun: true, LookbackDays: 14}, now, singleResult)
	assert.Equal(t, 1, singleResult.ErrorCount)
	require.Len(t, singleResult.Errors, 1)
	assert.Contains(t, singleResult.Errors[0].Error(), "categorizer unavailable")

	groupResult := &Result{}
	records := []amazonprovider.ReturnRecord{makeRecord("DoneRRMA", "B0ONE"), makeRecord("DtwoRRMA", "B0TWO")}
	orchestrator.processAmazonReturns(context.Background(), records, []*monarch.Transaction{transaction}, map[string]bool{}, nil, nil, Options{DryRun: true, LookbackDays: 14}, now, groupResult)
	assert.Equal(t, 2, groupResult.RefundSkippedCount)
	assert.Zero(t, groupResult.ErrorCount)
}

func TestProcessAmazonReturnsHonorsFiltersAndNilGuards(t *testing.T) {
	now := time.Date(2026, time.July, 17, 0, 0, 0, 0, time.UTC)
	issuedAt := now.AddDate(0, 0, -30)
	record := amazonprovider.ReturnRecord{OrderID: "order", RMAID: "DfilterRRMA", RefundIssuedAt: &issuedAt}
	orchestrator := &Orchestrator{logger: slog.Default()}
	result := &Result{}

	orchestrator.processAmazonReturns(context.Background(), []amazonprovider.ReturnRecord{record}, nil, map[string]bool{}, nil, nil, Options{OrderID: "different", LookbackDays: 14}, now, result)
	orchestrator.processAmazonReturns(context.Background(), []amazonprovider.ReturnRecord{record}, nil, map[string]bool{}, nil, nil, Options{OrderID: "order", LookbackDays: 14}, now, result)
	orchestrator.processAmazonReturns(context.Background(), nil, nil, nil, nil, nil, Options{}, now, nil)

	assert.Zero(t, result.RefundProcessedCount)
	assert.Zero(t, result.RefundSkippedCount)
	assert.False(t, returnFallsWithinLookback(amazonprovider.ReturnRecord{}, 14, now))
}
