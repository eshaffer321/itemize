package storage

// MockRepository is an in-memory implementation of Repository for testing.
// It stores all data in maps and slices, making tests fast and isolated.
type MockRepository struct {
	records   map[string]*ProcessingRecord
	syncRuns  map[int64]*mockSyncRun
	apiCalls  []APICall
	nextRunID int64

	// Hooks for test assertions
	SaveRecordCalled   bool
	LastSavedRecord    *ProcessingRecord
	GetRecordCalled    bool
	IsProcessedCalled  bool
	StartSyncRunCalled bool
	LogAPICallCalled   bool

	// Error injection for testing error paths
	SaveRecordErr      error
	GetRecordErr       error
	StartSyncRunErr    error
	CompleteSyncRunErr error
	LogAPICallErr      error
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
		records:   make(map[string]*ProcessingRecord),
		syncRuns:  make(map[int64]*mockSyncRun),
		apiCalls:  make([]APICall, 0),
		nextRunID: 1,
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

// GetSyncRun returns a sync run by ID (for assertions)
func (m *MockRepository) GetSyncRun(id int64) *mockSyncRun {
	return m.syncRuns[id]
}

// Reset clears all data and flags (for reuse between tests)
func (m *MockRepository) Reset() {
	m.records = make(map[string]*ProcessingRecord)
	m.syncRuns = make(map[int64]*mockSyncRun)
	m.apiCalls = make([]APICall, 0)
	m.nextRunID = 1
	m.SaveRecordCalled = false
	m.LastSavedRecord = nil
	m.GetRecordCalled = false
	m.IsProcessedCalled = false
	m.StartSyncRunCalled = false
	m.LogAPICallCalled = false
	m.SaveRecordErr = nil
	m.GetRecordErr = nil
	m.StartSyncRunErr = nil
	m.CompleteSyncRunErr = nil
	m.LogAPICallErr = nil
}
