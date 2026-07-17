package sync

import (
	"context"
	"log/slog"
	"testing"
	"time"

	amazonprovider "github.com/eshaffer321/itemize/internal/adapters/providers/amazon"
	"github.com/eshaffer321/itemize/internal/application/sync/handlers"
	"github.com/eshaffer321/itemize/internal/domain/categorizer"
	"github.com/eshaffer321/itemize/internal/domain/matcher"
	"github.com/eshaffer321/itemize/internal/domain/splitter"
	"github.com/eshaffer321/monarch-go/v2/pkg/monarch"
	"github.com/stretchr/testify/assert"
)

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
	}
	result := &Result{}

	orchestrator.processAmazonReturns(context.Background(), records, transactions, map[string]bool{}, []categorizer.Category{{ID: "kid-needs", Name: "Kid Needs"}}, nil, Options{DryRun: true, LookbackDays: 14}, now, result)

	assert.Equal(t, 2, result.RefundProcessedCount)
	assert.Equal(t, 0, result.RefundSkippedCount)
}
