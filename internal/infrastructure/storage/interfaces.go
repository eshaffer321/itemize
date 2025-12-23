package storage

// Repository defines the complete storage interface.
// This interface allows swapping implementations (SQLite, PostgreSQL, etc.)
// and makes testing with mocks straightforward.
type Repository interface {
	OrderRepository
	SyncRunRepository
	APICallRepository
	Close() error
}

// OrderRepository handles processing record operations
type OrderRepository interface {
	// SaveRecord saves or updates a processing record
	SaveRecord(record *ProcessingRecord) error

	// GetRecord retrieves a record by order ID
	GetRecord(orderID string) (*ProcessingRecord, error)

	// IsProcessed checks if an order has been successfully processed (non-dry-run)
	IsProcessed(orderID string) bool

	// GetStats returns aggregate statistics
	GetStats() (*Stats, error)
}

// SyncRunRepository handles sync run tracking
type SyncRunRepository interface {
	// StartSyncRun records the start of a sync run and returns the run ID
	StartSyncRun(provider string, lookbackDays int, dryRun bool) (int64, error)

	// CompleteSyncRun records the completion of a sync run
	CompleteSyncRun(runID int64, ordersFound, processed, skipped, errors int) error
}

// APICallRepository handles API call logging
type APICallRepository interface {
	// LogAPICall logs an API call to the database
	LogAPICall(call *APICall) error

	// GetAPICallsByOrderID retrieves all API calls for a specific order
	GetAPICallsByOrderID(orderID string) ([]APICall, error)

	// GetAPICallsByRunID retrieves all API calls for a specific sync run
	GetAPICallsByRunID(runID int64) ([]APICall, error)
}
