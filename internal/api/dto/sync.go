package dto

// StartSyncRequest is the request body for starting a sync.
type StartSyncRequest struct {
	Provider     string `json:"provider"`      // "walmart", "costco", "amazon"
	DryRun       bool   `json:"dry_run"`       // Preview mode
	LookbackDays int    `json:"lookback_days"` // How many days to look back (default 14)
	MaxOrders    int    `json:"max_orders"`    // Max orders to process (0 = all)
	Force        bool   `json:"force"`         // Force reprocess already processed orders
	Verbose      bool   `json:"verbose"`       // Verbose logging
	OrderID      string `json:"order_id"`      // Optional: process only this order
}

// StartSyncResponse is returned when a sync is started.
type StartSyncResponse struct {
	JobID    string `json:"job_id"`
	Provider string `json:"provider"`
	Status   string `json:"status"`
}

// SyncJobResponse represents a sync job's status.
type SyncJobResponse struct {
	JobID       string               `json:"job_id"`
	Provider    string               `json:"provider"`
	Status      string               `json:"status"`
	DryRun      bool                 `json:"dry_run"`
	StartedAt   string               `json:"started_at"`
	CompletedAt *string              `json:"completed_at,omitempty"`
	Progress    SyncProgressResponse `json:"progress"`
	Result      *SyncResultResponse  `json:"result,omitempty"`
	Error       *string              `json:"error,omitempty"`
}

// SyncProgressResponse represents real-time progress.
type SyncProgressResponse struct {
	CurrentPhase    string `json:"current_phase"`
	TotalOrders     int    `json:"total_orders"`
	ProcessedOrders int    `json:"processed_orders"`
	SkippedOrders   int    `json:"skipped_orders"`
	ErroredOrders   int    `json:"errored_orders"`
	LastUpdate      string `json:"last_update"`
}

// SyncResultResponse represents the final result.
type SyncResultResponse struct {
	ProcessedCount int `json:"processed_count"`
	SkippedCount   int `json:"skipped_count"`
	ErrorCount     int `json:"error_count"`
}

// ActiveSyncsResponse lists active sync jobs.
type ActiveSyncsResponse struct {
	Jobs  []SyncJobResponse `json:"jobs"`
	Count int               `json:"count"`
}

// AllSyncsResponse lists all sync jobs (including completed).
type AllSyncsResponse struct {
	Jobs  []SyncJobResponse `json:"jobs"`
	Count int               `json:"count"`
}

// MessageResponse is a generic message response.
type MessageResponse struct {
	Message string `json:"message"`
}
