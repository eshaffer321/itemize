package storage

import (
	"os"
	"testing"
	"time"
)

func TestStorage_SaveLedger(t *testing.T) {
	// Create temp DB
	tmpFile, err := os.CreateTemp("", "ledger_test_*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	s, err := NewStorage(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer s.Close()

	// Create a ledger (SyncRunID 0 means NULL, no foreign key constraint)
	ledger := &OrderLedger{
		OrderID:            "test-order-123",
		SyncRunID:          0,
		Provider:           "walmart",
		FetchedAt:          time.Now(),
		LedgerState:        LedgerStateCharged,
		LedgerJSON:         `{"test": "data"}`,
		TotalCharged:       99.99,
		ChargeCount:        1,
		PaymentMethodTypes: "CREDITCARD",
		HasRefunds:         false,
		IsValid:            true,
		ValidationNotes:    "",
		Charges: []LedgerCharge{
			{
				ChargeSequence: 1,
				ChargeAmount:   99.99,
				ChargeType:     "payment",
				PaymentMethod:  "CREDITCARD",
				CardType:       "VISA",
				CardLastFour:   "1234",
			},
		},
	}

	// Save the ledger
	err = s.SaveLedger(ledger)
	if err != nil {
		t.Fatalf("failed to save ledger: %v", err)
	}

	// Verify ID was assigned
	if ledger.ID == 0 {
		t.Error("expected ledger ID to be assigned")
	}

	// Retrieve it
	retrieved, err := s.GetLatestLedger("test-order-123")
	if err != nil {
		t.Fatalf("failed to get latest ledger: %v", err)
	}

	if retrieved == nil {
		t.Fatal("expected to retrieve ledger, got nil")
	}

	if retrieved.OrderID != "test-order-123" {
		t.Errorf("expected order_id 'test-order-123', got %q", retrieved.OrderID)
	}

	if retrieved.Provider != "walmart" {
		t.Errorf("expected provider 'walmart', got %q", retrieved.Provider)
	}

	if retrieved.LedgerState != LedgerStateCharged {
		t.Errorf("expected state 'charged', got %q", retrieved.LedgerState)
	}

	if retrieved.TotalCharged != 99.99 {
		t.Errorf("expected total_charged 99.99, got %f", retrieved.TotalCharged)
	}

	if retrieved.LedgerVersion != 1 {
		t.Errorf("expected version 1, got %d", retrieved.LedgerVersion)
	}

	// Check charges were saved
	if len(retrieved.Charges) != 1 {
		t.Errorf("expected 1 charge, got %d", len(retrieved.Charges))
	} else {
		charge := retrieved.Charges[0]
		if charge.ChargeAmount != 99.99 {
			t.Errorf("expected charge amount 99.99, got %f", charge.ChargeAmount)
		}
		if charge.PaymentMethod != "CREDITCARD" {
			t.Errorf("expected payment method 'CREDITCARD', got %q", charge.PaymentMethod)
		}
		if charge.CardLastFour != "1234" {
			t.Errorf("expected card last four '1234', got %q", charge.CardLastFour)
		}
	}
}

func TestStorage_LedgerVersioning(t *testing.T) {
	// Create temp DB
	tmpFile, err := os.CreateTemp("", "ledger_version_test_*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	s, err := NewStorage(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer s.Close()

	// Save first ledger (pending state)
	ledger1 := &OrderLedger{
		OrderID:     "order-456",
		Provider:    "walmart",
		FetchedAt:   time.Now().Add(-24 * time.Hour),
		LedgerState: LedgerStatePending,
		LedgerJSON:  `{"state": "pending"}`,
	}
	if err := s.SaveLedger(ledger1); err != nil {
		t.Fatalf("failed to save ledger 1: %v", err)
	}

	// Save second ledger (charged state)
	ledger2 := &OrderLedger{
		OrderID:      "order-456",
		Provider:     "walmart",
		FetchedAt:    time.Now(),
		LedgerState:  LedgerStateCharged,
		LedgerJSON:   `{"state": "charged"}`,
		TotalCharged: 50.00,
		ChargeCount:  1,
		Charges: []LedgerCharge{
			{
				ChargeSequence: 1,
				ChargeAmount:   50.00,
				ChargeType:     "payment",
				PaymentMethod:  "CREDITCARD",
			},
		},
	}
	if err := s.SaveLedger(ledger2); err != nil {
		t.Fatalf("failed to save ledger 2: %v", err)
	}

	// Version should be 2 now
	if ledger2.LedgerVersion != 2 {
		t.Errorf("expected version 2, got %d", ledger2.LedgerVersion)
	}

	// Get latest should return the highest version (version 2)
	latest, err := s.GetLatestLedger("order-456")
	if err != nil {
		t.Fatalf("failed to get latest: %v", err)
	}
	if latest.LedgerVersion != 2 {
		t.Errorf("expected version 2 (latest), got %d", latest.LedgerVersion)
	}
	if latest.LedgerState != LedgerStateCharged {
		t.Errorf("expected state 'charged', got %q", latest.LedgerState)
	}

	// Get history should return both
	history, err := s.GetLedgerHistory("order-456")
	if err != nil {
		t.Fatalf("failed to get history: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(history))
	}
	// Verify both versions are present (order might vary due to same timestamp)
	foundV1, foundV2 := false, false
	for _, h := range history {
		if h.LedgerVersion == 1 && h.LedgerState == LedgerStatePending {
			foundV1 = true
		}
		if h.LedgerVersion == 2 && h.LedgerState == LedgerStateCharged {
			foundV2 = true
		}
	}
	if !foundV1 {
		t.Error("expected to find version 1 with pending state")
	}
	if !foundV2 {
		t.Error("expected to find version 2 with charged state")
	}
}

func TestStorage_GetLedgerByID(t *testing.T) {
	// Create temp DB
	tmpFile, err := os.CreateTemp("", "ledger_byid_test_*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	s, err := NewStorage(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer s.Close()

	// Save a ledger
	ledger := &OrderLedger{
		OrderID:     "order-789",
		Provider:    "costco",
		FetchedAt:   time.Now(),
		LedgerState: LedgerStateCharged,
		LedgerJSON:  `{}`,
	}
	if err := s.SaveLedger(ledger); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Retrieve by ID
	retrieved, err := s.GetLedgerByID(ledger.ID)
	if err != nil {
		t.Fatalf("failed to get by ID: %v", err)
	}
	if retrieved == nil {
		t.Fatal("expected ledger, got nil")
	}
	if retrieved.OrderID != "order-789" {
		t.Errorf("expected order_id 'order-789', got %q", retrieved.OrderID)
	}

	// Non-existent ID should return nil
	missing, err := s.GetLedgerByID(9999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if missing != nil {
		t.Error("expected nil for non-existent ID")
	}
}

func TestStorage_ListLedgers(t *testing.T) {
	// Create temp DB
	tmpFile, err := os.CreateTemp("", "ledger_list_test_*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	s, err := NewStorage(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer s.Close()

	// Save multiple ledgers
	for _, data := range []struct {
		orderID  string
		provider string
		state    LedgerState
	}{
		{"order-1", "walmart", LedgerStateCharged},
		{"order-2", "walmart", LedgerStatePending},
		{"order-3", "costco", LedgerStateCharged},
		{"order-4", "amazon", LedgerStateRefunded},
	} {
		ledger := &OrderLedger{
			OrderID:     data.orderID,
			Provider:    data.provider,
			FetchedAt:   time.Now(),
			LedgerState: data.state,
			LedgerJSON:  `{}`,
		}
		if err := s.SaveLedger(ledger); err != nil {
			t.Fatalf("failed to save ledger: %v", err)
		}
	}

	// List all
	result, err := s.ListLedgers(LedgerFilters{})
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}
	if result.TotalCount != 4 {
		t.Errorf("expected total count 4, got %d", result.TotalCount)
	}

	// Filter by provider
	result, err = s.ListLedgers(LedgerFilters{Provider: "walmart"})
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}
	if result.TotalCount != 2 {
		t.Errorf("expected 2 walmart ledgers, got %d", result.TotalCount)
	}

	// Filter by state
	result, err = s.ListLedgers(LedgerFilters{State: LedgerStateCharged})
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}
	if result.TotalCount != 2 {
		t.Errorf("expected 2 charged ledgers, got %d", result.TotalCount)
	}

	// Pagination
	result, err = s.ListLedgers(LedgerFilters{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}
	if len(result.Ledgers) != 2 {
		t.Errorf("expected 2 ledgers with limit, got %d", len(result.Ledgers))
	}
	if result.TotalCount != 4 {
		t.Errorf("expected total count 4, got %d", result.TotalCount)
	}
}

func TestStorage_UpdateChargeMatch(t *testing.T) {
	// Create temp DB
	tmpFile, err := os.CreateTemp("", "ledger_match_test_*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	s, err := NewStorage(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer s.Close()

	// Save a ledger with charges
	ledger := &OrderLedger{
		OrderID:     "match-order",
		Provider:    "walmart",
		FetchedAt:   time.Now(),
		LedgerState: LedgerStateCharged,
		LedgerJSON:  `{}`,
		Charges: []LedgerCharge{
			{
				ChargeSequence: 1,
				ChargeAmount:   100.00,
				ChargeType:     "payment",
				PaymentMethod:  "CREDITCARD",
			},
		},
	}
	if err := s.SaveLedger(ledger); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Get the charge ID
	retrieved, _ := s.GetLatestLedger("match-order")
	chargeID := retrieved.Charges[0].ID

	// Update match
	err = s.UpdateChargeMatch(chargeID, "monarch-tx-123", 0.95, 3)
	if err != nil {
		t.Fatalf("failed to update match: %v", err)
	}

	// Verify update
	retrieved, _ = s.GetLatestLedger("match-order")
	charge := retrieved.Charges[0]
	if !charge.IsMatched {
		t.Error("expected charge to be matched")
	}
	if charge.MonarchTransactionID != "monarch-tx-123" {
		t.Errorf("expected monarch tx 'monarch-tx-123', got %q", charge.MonarchTransactionID)
	}
	if charge.MatchConfidence != 0.95 {
		t.Errorf("expected confidence 0.95, got %f", charge.MatchConfidence)
	}
	if charge.SplitCount != 3 {
		t.Errorf("expected split count 3, got %d", charge.SplitCount)
	}
}

func TestStorage_GetUnmatchedCharges(t *testing.T) {
	// Create temp DB
	tmpFile, err := os.CreateTemp("", "ledger_unmatched_test_*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	s, err := NewStorage(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer s.Close()

	// Save ledgers with mixed matched/unmatched charges
	ledger1 := &OrderLedger{
		OrderID:     "unmatched-1",
		Provider:    "walmart",
		FetchedAt:   time.Now(),
		LedgerState: LedgerStateCharged,
		LedgerJSON:  `{}`,
		Charges: []LedgerCharge{
			{ChargeSequence: 1, ChargeAmount: 50.00, ChargeType: "payment", PaymentMethod: "CREDITCARD"},
		},
	}
	if err := s.SaveLedger(ledger1); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	ledger2 := &OrderLedger{
		OrderID:     "unmatched-2",
		Provider:    "costco",
		FetchedAt:   time.Now(),
		LedgerState: LedgerStateCharged,
		LedgerJSON:  `{}`,
		Charges: []LedgerCharge{
			{ChargeSequence: 1, ChargeAmount: 75.00, ChargeType: "payment", PaymentMethod: "CREDITCARD"},
		},
	}
	if err := s.SaveLedger(ledger2); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Mark one as matched
	retrieved, _ := s.GetLatestLedger("unmatched-1")
	s.UpdateChargeMatch(retrieved.Charges[0].ID, "tx-matched", 1.0, 1)

	// Get unmatched for all providers
	unmatched, err := s.GetUnmatchedCharges("", 50)
	if err != nil {
		t.Fatalf("failed to get unmatched: %v", err)
	}
	if len(unmatched) != 1 {
		t.Errorf("expected 1 unmatched charge, got %d", len(unmatched))
	}
	if len(unmatched) > 0 && unmatched[0].OrderID != "unmatched-2" {
		t.Errorf("expected order 'unmatched-2', got %q", unmatched[0].OrderID)
	}

	// Get unmatched for specific provider
	unmatched, err = s.GetUnmatchedCharges("walmart", 50)
	if err != nil {
		t.Fatalf("failed to get unmatched: %v", err)
	}
	if len(unmatched) != 0 {
		t.Errorf("expected 0 unmatched walmart charges, got %d", len(unmatched))
	}
}

func TestMockRepository_Ledger(t *testing.T) {
	mock := NewMockRepository()

	// Test SaveLedger
	ledger := &OrderLedger{
		OrderID:     "mock-order-1",
		Provider:    "walmart",
		FetchedAt:   time.Now(),
		LedgerState: LedgerStateCharged,
		LedgerJSON:  `{"test": true}`,
		Charges: []LedgerCharge{
			{ChargeSequence: 1, ChargeAmount: 25.00, ChargeType: "payment"},
		},
	}
	if err := mock.SaveLedger(ledger); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	if !mock.SaveLedgerCalled {
		t.Error("expected SaveLedgerCalled to be true")
	}
	if mock.LastSavedLedger != ledger {
		t.Error("expected LastSavedLedger to be set")
	}
	if ledger.ID == 0 {
		t.Error("expected ID to be assigned")
	}
	if ledger.LedgerVersion != 1 {
		t.Errorf("expected version 1, got %d", ledger.LedgerVersion)
	}

	// Test GetLatestLedger
	latest, err := mock.GetLatestLedger("mock-order-1")
	if err != nil {
		t.Fatalf("failed to get latest: %v", err)
	}
	if latest == nil {
		t.Fatal("expected ledger, got nil")
	}
	if latest.OrderID != "mock-order-1" {
		t.Errorf("expected order 'mock-order-1', got %q", latest.OrderID)
	}

	// Test version incrementing
	ledger2 := &OrderLedger{
		OrderID:     "mock-order-1",
		Provider:    "walmart",
		FetchedAt:   time.Now(),
		LedgerState: LedgerStateRefunded,
		LedgerJSON:  `{"refund": true}`,
	}
	mock.SaveLedger(ledger2)
	if ledger2.LedgerVersion != 2 {
		t.Errorf("expected version 2, got %d", ledger2.LedgerVersion)
	}

	// Test GetLedgerHistory
	history, _ := mock.GetLedgerHistory("mock-order-1")
	if len(history) != 2 {
		t.Errorf("expected 2 history entries, got %d", len(history))
	}

	// Test error injection
	mock.SaveLedgerErr = os.ErrNotExist
	if err := mock.SaveLedger(&OrderLedger{}); err != os.ErrNotExist {
		t.Errorf("expected injected error, got %v", err)
	}

	// Test Reset clears ledger data
	mock.Reset()
	if mock.SaveLedgerCalled {
		t.Error("expected SaveLedgerCalled to be false after reset")
	}
	latest, _ = mock.GetLatestLedger("mock-order-1")
	if latest != nil {
		t.Error("expected nil after reset")
	}
}
