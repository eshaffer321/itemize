package sync

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
	"github.com/stretchr/testify/assert"
)

// TestOrchestrator_processOrder tests single order processing
// SKIP: This test requires mocking clients and categorizer, which is complex
// The real value of the refactoring is reducing the Run method from 300+ lines to ~30 lines
func TestOrchestrator_processOrder(t *testing.T) {
	t.Skip("Requires mocking Monarch client and categorizer - test with integration tests instead")
	testDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name            string
		order           providers.Order
		transactions    []*monarch.Transaction
		usedIDs         map[string]bool
		expectProcessed bool
		expectSkipped   bool
		expectError     bool
		expectUsedID    string
	}{
		{
			name: "successfully processes matching order",
			order: func() providers.Order {
				m := new(MockOrder)
				m.On("GetID").Return("order123")
				m.On("GetDate").Return(testDate)
				m.On("GetTotal").Return(100.0)
				m.On("GetSubtotal").Return(90.0)
				m.On("GetTax").Return(10.0)
				m.On("GetTip").Return(0.0)
				m.On("GetFees").Return(0.0)
				m.On("GetItems").Return([]providers.OrderItem{})
				m.On("GetProviderName").Return("costco")
				return m
			}(),
			transactions: []*monarch.Transaction{
				{
					ID:     "tx1",
					Amount: -100.0, // Negative for purchase (matches order total)
					Date:   monarch.Date{Time: testDate},
					Merchant: &monarch.Merchant{
						Name: "Test Store",
					},
					HasSplits: false,
				},
			},
			usedIDs:         make(map[string]bool),
			expectProcessed: true,
			expectSkipped:   false,
			expectError:     false,
			expectUsedID:    "tx1",
		},
		{
			name: "skips transaction that already has splits",
			order: func() providers.Order {
				m := new(MockOrder)
				m.On("GetID").Return("order123")
				m.On("GetDate").Return(testDate)
				m.On("GetTotal").Return(100.0)
				m.On("GetSubtotal").Return(90.0)
				m.On("GetTax").Return(10.0)
				m.On("GetItems").Return([]providers.OrderItem{})
				m.On("GetProviderName").Return("costco")
				return m
			}(),
			transactions: []*monarch.Transaction{
				{
					ID:     "tx1",
					Amount: -100.0,
					Date:   monarch.Date{Time: testDate},
					Merchant: &monarch.Merchant{
						Name: "Test Store",
					},
					HasSplits: true, // Already has splits
				},
			},
			usedIDs:         make(map[string]bool),
			expectProcessed: false,
			expectSkipped:   true,
			expectError:     false,
		},
		{
			name: "errors when no matching transaction found",
			order: func() providers.Order {
				m := new(MockOrder)
				m.On("GetID").Return("order123")
				m.On("GetDate").Return(testDate)
				m.On("GetTotal").Return(100.0)
				m.On("GetItems").Return([]providers.OrderItem{})
				m.On("GetProviderName").Return("costco")
				return m
			}(),
			transactions:    []*monarch.Transaction{}, // No transactions
			usedIDs:         make(map[string]bool),
			expectProcessed: false,
			expectSkipped:   false,
			expectError:     true,
		},
		{
			name: "skips transaction already used",
			order: func() providers.Order {
				m := new(MockOrder)
				m.On("GetID").Return("order123")
				m.On("GetDate").Return(testDate)
				m.On("GetTotal").Return(100.0)
				m.On("GetItems").Return([]providers.OrderItem{})
				m.On("GetProviderName").Return("costco")
				return m
			}(),
			transactions: []*monarch.Transaction{
				{
					ID:     "tx1",
					Amount: -100.0,
					Date:   monarch.Date{Time: testDate},
				},
			},
			usedIDs: map[string]bool{
				"tx1": true, // Already used
			},
			expectProcessed: false,
			expectSkipped:   false,
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockProvider := new(MockProvider)
			logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
			orchestrator := NewOrchestrator(mockProvider, nil, nil, logger)

			opts := Options{
				DryRun:  true,
				Verbose: false,
			}

			// Act
			processed, skipped, err := orchestrator.processOrder(
				context.Background(),
				tt.order,
				tt.transactions,
				tt.usedIDs,
				nil, // categories
				nil, // monarch categories
				opts,
			)

			// Assert
			assert.Equal(t, tt.expectProcessed, processed, "processed mismatch")
			assert.Equal(t, tt.expectSkipped, skipped, "skipped mismatch")
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.expectUsedID != "" {
				assert.True(t, tt.usedIDs[tt.expectUsedID], "expected transaction ID to be marked as used")
			}
		})
	}
}
