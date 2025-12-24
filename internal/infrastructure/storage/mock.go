package storage

// MockRepository is an in-memory implementation of Repository for testing.
// It stores all data in maps and slices, making tests fast and isolated.
type MockRepository struct {
	records       map[string]*ProcessingRecord
	syncRuns      map[int64]*mockSyncRun
	apiCalls      []APICall
	ledgers       map[string][]*OrderLedger // Keyed by order_id
	ledgerCharges map[int64][]LedgerCharge  // Keyed by ledger_id
	nextRunID     int64
	nextLedgerID  int64
	nextChargeID  int64

	// Hooks for test assertions
	SaveRecordCalled   bool
	LastSavedRecord    *ProcessingRecord
	GetRecordCalled    bool
	IsProcessedCalled  bool
	StartSyncRunCalled bool
	LogAPICallCalled   bool
	SaveLedgerCalled   bool
	LastSavedLedger    *OrderLedger

	// Error injection for testing error paths
	SaveRecordErr      error
	GetRecordErr       error
	StartSyncRunErr    error
	CompleteSyncRunErr error
	LogAPICallErr      error
	SaveLedgerErr      error
}

type mockSyncRun struct {
	id           int64
	provider     string
	lookbackDays int
	dryRun       bool
	ordersFound  int
	processed    int
	skipped      int
	errors       int
	completed    bool
}

// NewMockRepository creates a new mock repository for testing
func NewMockRepository() *MockRepository {
	return &MockRepository{
		records:       make(map[string]*ProcessingRecord),
		syncRuns:      make(map[int64]*mockSyncRun),
		apiCalls:      make([]APICall, 0),
		ledgers:       make(map[string][]*OrderLedger),
		ledgerCharges: make(map[int64][]LedgerCharge),
		nextRunID:     1,
		nextLedgerID:  1,
		nextChargeID:  1,
	}
}

// Compile-time check that MockRepository implements Repository
var _ Repository = (*MockRepository)(nil)

// Close does nothing for mock
func (m *MockRepository) Close() error {
	return nil
}

// SaveRecord saves a record to the in-memory map
func (m *MockRepository) SaveRecord(record *ProcessingRecord) error {
	m.SaveRecordCalled = true
	m.LastSavedRecord = record
	if m.SaveRecordErr != nil {
		return m.SaveRecordErr
	}
	// Deep copy to avoid test mutations
	copied := *record
	m.records[record.OrderID] = &copied
	return nil
}

// GetRecord retrieves a record from the in-memory map
func (m *MockRepository) GetRecord(orderID string) (*ProcessingRecord, error) {
	m.GetRecordCalled = true
	if m.GetRecordErr != nil {
		return nil, m.GetRecordErr
	}
	record, ok := m.records[orderID]
	if !ok {
		return nil, nil
	}
	return record, nil
}

// IsProcessed checks if an order exists with success status and not dry-run
func (m *MockRepository) IsProcessed(orderID string) bool {
	m.IsProcessedCalled = true
	record, ok := m.records[orderID]
	if !ok {
		return false
	}
	return record.Status == "success" && !record.DryRun
}

// GetStats returns mock statistics
func (m *MockRepository) GetStats() (*Stats, error) {
	stats := &Stats{
		ProviderStats: make(map[string]ProviderStats),
	}

	for _, record := range m.records {
		stats.TotalProcessed++
		stats.TotalAmount += record.OrderTotal
		stats.TotalSplits += record.SplitCount

		switch record.Status {
		case "success":
			stats.SuccessCount++
		case "failed":
			stats.FailedCount++
		case "skipped":
			stats.SkippedCount++
		}
		if record.DryRun {
			stats.DryRunCount++
		}

		// Update provider stats
		ps := stats.ProviderStats[record.Provider]
		ps.Count++
		ps.TotalAmount += record.OrderTotal
		if record.Status == "success" {
			ps.SuccessCount++
		}
		stats.ProviderStats[record.Provider] = ps
	}

	if stats.TotalProcessed > 0 {
		stats.AverageOrderAmount = stats.TotalAmount / float64(stats.TotalProcessed)
	}

	return stats, nil
}

// StartSyncRun creates a new sync run and returns its ID
func (m *MockRepository) StartSyncRun(provider string, lookbackDays int, dryRun bool) (int64, error) {
	m.StartSyncRunCalled = true
	if m.StartSyncRunErr != nil {
		return 0, m.StartSyncRunErr
	}

	id := m.nextRunID
	m.nextRunID++

	m.syncRuns[id] = &mockSyncRun{
		id:           id,
		provider:     provider,
		lookbackDays: lookbackDays,
		dryRun:       dryRun,
	}

	return id, nil
}

// CompleteSyncRun marks a sync run as complete
func (m *MockRepository) CompleteSyncRun(runID int64, ordersFound, processed, skipped, errors int) error {
	if m.CompleteSyncRunErr != nil {
		return m.CompleteSyncRunErr
	}

	run, ok := m.syncRuns[runID]
	if !ok {
		return nil
	}

	run.ordersFound = ordersFound
	run.processed = processed
	run.skipped = skipped
	run.errors = errors
	run.completed = true

	return nil
}

// LogAPICall logs an API call
func (m *MockRepository) LogAPICall(call *APICall) error {
	m.LogAPICallCalled = true
	if m.LogAPICallErr != nil {
		return m.LogAPICallErr
	}
	m.apiCalls = append(m.apiCalls, *call)
	return nil
}

// GetAPICallsByOrderID retrieves API calls for an order
func (m *MockRepository) GetAPICallsByOrderID(orderID string) ([]APICall, error) {
	var result []APICall
	for _, call := range m.apiCalls {
		if call.OrderID == orderID {
			result = append(result, call)
		}
	}
	return result, nil
}

// GetAPICallsByRunID retrieves API calls for a sync run
func (m *MockRepository) GetAPICallsByRunID(runID int64) ([]APICall, error) {
	var result []APICall
	for _, call := range m.apiCalls {
		if call.RunID == runID {
			result = append(result, call)
		}
	}
	return result, nil
}

// ListOrders returns orders matching the given filters with pagination
func (m *MockRepository) ListOrders(filters OrderFilters) (*OrderListResult, error) {
	// Collect matching records
	var matching []*ProcessingRecord
	for _, r := range m.records {
		// Apply provider filter
		if filters.Provider != "" && r.Provider != filters.Provider {
			continue
		}
		// Apply status filter
		if filters.Status != "" && r.Status != filters.Status {
			continue
		}
		matching = append(matching, r)
	}

	// Apply defaults
	limit := filters.Limit
	if limit == 0 {
		limit = 50
	}

	// Apply pagination
	total := len(matching)
	start := filters.Offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	return &OrderListResult{
		Orders:     matching[start:end],
		TotalCount: total,
		Limit:      limit,
		Offset:     filters.Offset,
	}, nil
}

// SearchItems searches for items across all orders
func (m *MockRepository) SearchItems(query string, limit int) ([]ItemSearchResult, error) {
	if limit == 0 {
		limit = 50
	}

	var results []ItemSearchResult
	for _, r := range m.records {
		for _, item := range r.Items {
			// Simple case-insensitive contains match
			if containsIgnoreCase(item.Name, query) {
				results = append(results, ItemSearchResult{
					OrderID:   r.OrderID,
					Provider:  r.Provider,
					OrderDate: r.OrderDate.Format("2006-01-02"),
					ItemName:  item.Name,
					ItemPrice: item.TotalPrice,
					Category:  item.Category,
				})
				if len(results) >= limit {
					return results, nil
				}
			}
		}
	}
	return results, nil
}

// containsIgnoreCase is a helper for case-insensitive string matching
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(substr) == 0 ||
		(len(s) > 0 && containsIgnoreCaseImpl(s, substr)))
}

func containsIgnoreCaseImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFoldAt(s, i, substr) {
			return true
		}
	}
	return false
}

func equalFoldAt(s string, start int, substr string) bool {
	for j := 0; j < len(substr); j++ {
		c1 := s[start+j]
		c2 := substr[j]
		if c1 != c2 {
			// Simple ASCII case folding
			if c1 >= 'A' && c1 <= 'Z' {
				c1 += 'a' - 'A'
			}
			if c2 >= 'A' && c2 <= 'Z' {
				c2 += 'a' - 'A'
			}
			if c1 != c2 {
				return false
			}
		}
	}
	return true
}

// ListSyncRuns returns recent sync runs
func (m *MockRepository) ListSyncRuns(limit int) ([]SyncRun, error) {
	if limit == 0 {
		limit = 20
	}

	var runs []SyncRun
	for _, r := range m.syncRuns {
		status := "running"
		if r.completed {
			status = "completed"
		}
		runs = append(runs, SyncRun{
			ID:              r.id,
			Provider:        r.provider,
			LookbackDays:    r.lookbackDays,
			DryRun:          r.dryRun,
			OrdersFound:     r.ordersFound,
			OrdersProcessed: r.processed,
			OrdersSkipped:   r.skipped,
			OrdersErrored:   r.errors,
			Status:          status,
		})
		if len(runs) >= limit {
			break
		}
	}
	return runs, nil
}

// GetSyncRun retrieves a sync run by ID (implements SyncRunRepository interface)
func (m *MockRepository) GetSyncRun(runID int64) (*SyncRun, error) {
	r, ok := m.syncRuns[runID]
	if !ok {
		return nil, nil
	}
	status := "running"
	if r.completed {
		status = "completed"
	}
	return &SyncRun{
		ID:              r.id,
		Provider:        r.provider,
		LookbackDays:    r.lookbackDays,
		DryRun:          r.dryRun,
		OrdersFound:     r.ordersFound,
		OrdersProcessed: r.processed,
		OrdersSkipped:   r.skipped,
		OrdersErrored:   r.errors,
		Status:          status,
	}, nil
}

// Helper methods for test setup

// AddRecord adds a record directly (for test setup)
func (m *MockRepository) AddRecord(record *ProcessingRecord) {
	m.records[record.OrderID] = record
}

// GetAllRecords returns all stored records (for assertions)
func (m *MockRepository) GetAllRecords() []*ProcessingRecord {
	result := make([]*ProcessingRecord, 0, len(m.records))
	for _, r := range m.records {
		result = append(result, r)
	}
	return result
}

// GetMockSyncRun returns the internal mockSyncRun for test assertions
func (m *MockRepository) GetMockSyncRun(id int64) *mockSyncRun {
	return m.syncRuns[id]
}

// Reset clears all data and flags (for reuse between tests)
func (m *MockRepository) Reset() {
	m.records = make(map[string]*ProcessingRecord)
	m.syncRuns = make(map[int64]*mockSyncRun)
	m.apiCalls = make([]APICall, 0)
	m.ledgers = make(map[string][]*OrderLedger)
	m.ledgerCharges = make(map[int64][]LedgerCharge)
	m.nextRunID = 1
	m.nextLedgerID = 1
	m.nextChargeID = 1
	m.SaveRecordCalled = false
	m.LastSavedRecord = nil
	m.GetRecordCalled = false
	m.IsProcessedCalled = false
	m.StartSyncRunCalled = false
	m.LogAPICallCalled = false
	m.SaveLedgerCalled = false
	m.LastSavedLedger = nil
	m.SaveRecordErr = nil
	m.GetRecordErr = nil
	m.StartSyncRunErr = nil
	m.CompleteSyncRunErr = nil
	m.LogAPICallErr = nil
	m.SaveLedgerErr = nil
}

// ================================================================
// LEDGER REPOSITORY METHODS
// ================================================================

// SaveLedger saves a ledger snapshot with its charges
func (m *MockRepository) SaveLedger(ledger *OrderLedger) error {
	m.SaveLedgerCalled = true
	m.LastSavedLedger = ledger
	if m.SaveLedgerErr != nil {
		return m.SaveLedgerErr
	}

	// Assign ID
	ledger.ID = m.nextLedgerID
	m.nextLedgerID++

	// Calculate version based on existing ledgers for this order
	existingLedgers := m.ledgers[ledger.OrderID]
	ledger.LedgerVersion = len(existingLedgers) + 1

	// Deep copy the ledger
	copied := *ledger

	// Process and store charges
	var charges []LedgerCharge
	for i := range ledger.Charges {
		charge := ledger.Charges[i]
		charge.ID = m.nextChargeID
		m.nextChargeID++
		charge.OrderLedgerID = copied.ID
		charge.OrderID = copied.OrderID
		charge.SyncRunID = copied.SyncRunID
		charges = append(charges, charge)
	}
	copied.Charges = charges

	// Store in ledgers map (append to history)
	m.ledgers[ledger.OrderID] = append(m.ledgers[ledger.OrderID], &copied)

	// Store charges by ledger ID
	m.ledgerCharges[copied.ID] = charges

	return nil
}

// GetLatestLedger retrieves the most recent ledger for an order
func (m *MockRepository) GetLatestLedger(orderID string) (*OrderLedger, error) {
	ledgers := m.ledgers[orderID]
	if len(ledgers) == 0 {
		return nil, nil
	}
	// Return the last one (most recent)
	latest := ledgers[len(ledgers)-1]
	// Attach charges
	result := *latest
	result.Charges = m.ledgerCharges[latest.ID]
	return &result, nil
}

// GetLedgerHistory retrieves all ledger snapshots for an order (newest first)
func (m *MockRepository) GetLedgerHistory(orderID string) ([]*OrderLedger, error) {
	ledgers := m.ledgers[orderID]
	if len(ledgers) == 0 {
		return nil, nil
	}

	// Return in reverse order (newest first)
	result := make([]*OrderLedger, len(ledgers))
	for i, l := range ledgers {
		copied := *l
		copied.Charges = m.ledgerCharges[l.ID]
		result[len(ledgers)-1-i] = &copied
	}
	return result, nil
}

// GetLedgerByID retrieves a specific ledger by ID
func (m *MockRepository) GetLedgerByID(id int64) (*OrderLedger, error) {
	for _, ledgers := range m.ledgers {
		for _, l := range ledgers {
			if l.ID == id {
				result := *l
				result.Charges = m.ledgerCharges[l.ID]
				return &result, nil
			}
		}
	}
	return nil, nil
}

// ListLedgers returns ledgers matching the given filters with pagination
func (m *MockRepository) ListLedgers(filters LedgerFilters) (*LedgerListResult, error) {
	var matching []*OrderLedger

	for _, ledgers := range m.ledgers {
		for _, l := range ledgers {
			// Apply filters
			if filters.OrderID != "" && l.OrderID != filters.OrderID {
				continue
			}
			if filters.Provider != "" && l.Provider != filters.Provider {
				continue
			}
			if filters.State != "" && l.LedgerState != filters.State {
				continue
			}
			matching = append(matching, l)
		}
	}

	// Apply defaults
	limit := filters.Limit
	if limit == 0 {
		limit = 50
	}

	// Apply pagination
	total := len(matching)
	start := filters.Offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	return &LedgerListResult{
		Ledgers:    matching[start:end],
		TotalCount: total,
		Limit:      limit,
		Offset:     filters.Offset,
	}, nil
}

// UpdateChargeMatch updates a ledger charge's match status
func (m *MockRepository) UpdateChargeMatch(chargeID int64, transactionID string, confidence float64, splitCount int) error {
	for ledgerID, charges := range m.ledgerCharges {
		for i, charge := range charges {
			if charge.ID == chargeID {
				m.ledgerCharges[ledgerID][i].MonarchTransactionID = transactionID
				m.ledgerCharges[ledgerID][i].IsMatched = true
				m.ledgerCharges[ledgerID][i].MatchConfidence = confidence
				m.ledgerCharges[ledgerID][i].SplitCount = splitCount
				return nil
			}
		}
	}
	return nil
}

// GetUnmatchedCharges returns charges that haven't been matched to Monarch transactions
func (m *MockRepository) GetUnmatchedCharges(provider string, limit int) ([]LedgerCharge, error) {
	if limit == 0 {
		limit = 50
	}

	var result []LedgerCharge
	for _, charges := range m.ledgerCharges {
		for _, charge := range charges {
			if charge.IsMatched {
				continue
			}
			if charge.ChargeType != "payment" {
				continue
			}
			// Check provider via the ledger
			ledger, _ := m.GetLedgerByID(charge.OrderLedgerID)
			if ledger != nil && provider != "" && ledger.Provider != provider {
				continue
			}
			result = append(result, charge)
			if len(result) >= limit {
				return result, nil
			}
		}
	}
	return result, nil
}
