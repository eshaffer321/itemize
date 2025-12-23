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

	// ListOrders returns orders matching the given filters with pagination
	ListOrders(filters OrderFilters) (*OrderListResult, error)

	// SearchItems searches for items across all orders
	SearchItems(query string, limit int) ([]ItemSearchResult, error)
}

// OrderFilters defines filters for listing orders
type OrderFilters struct {
	Provider  string // Filter by provider (empty = all)
	Status    string // Filter by status (empty = all)
	DaysBack  int    // How many days back to look (0 = all time)
	Limit     int    // Max results (0 = default 50)
	Offset    int    // Pagination offset
	OrderBy   string // Sort field: "date", "total", "processed_at" (default: "processed_at")
	OrderDesc bool   // Sort descending (default: true)
}

// OrderListResult contains paginated order results
type OrderListResult struct {
	Orders     []*ProcessingRecord `json:"orders"`
	TotalCount int                 `json:"total_count"`
	Limit      int                 `json:"limit"`
	Offset     int                 `json:"offset"`
}

// ItemSearchResult represents an item found in search
type ItemSearchResult struct {
	OrderID    string  `json:"order_id"`
	Provider   string  `json:"provider"`
	OrderDate  string  `json:"order_date"`
	ItemName   string  `json:"item_name"`
	ItemPrice  float64 `json:"item_price"`
	Category   string  `json:"category,omitempty"`
}

// SyncRunRepository handles sync run tracking
type SyncRunRepository interface {
	// StartSyncRun records the start of a sync run and returns the run ID
	StartSyncRun(provider string, lookbackDays int, dryRun bool) (int64, error)

	// CompleteSyncRun records the completion of a sync run
	CompleteSyncRun(runID int64, ordersFound, processed, skipped, errors int) error

	// ListSyncRuns returns recent sync runs
	ListSyncRuns(limit int) ([]SyncRun, error)

	// GetSyncRun retrieves a sync run by ID
	GetSyncRun(runID int64) (*SyncRun, error)
}

// SyncRun represents a sync run record
type SyncRun struct {
	ID              int64   `json:"id"`
	Provider        string  `json:"provider"`
	StartedAt       string  `json:"started_at"`
	CompletedAt     string  `json:"completed_at,omitempty"`
	LookbackDays    int     `json:"lookback_days"`
	DryRun          bool    `json:"dry_run"`
	OrdersFound     int     `json:"orders_found"`
	OrdersProcessed int     `json:"orders_processed"`
	OrdersSkipped   int     `json:"orders_skipped"`
	OrdersErrored   int     `json:"orders_errored"`
	Status          string  `json:"status"`
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
