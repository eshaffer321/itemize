package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSyncService_StartSync_MissingProvider(t *testing.T) {
	svc := NewSyncService(nil, nil, nil, nil, nil)

	_, err := svc.StartSync(nil, SyncRequest{
		Provider: "",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid provider")
}

func TestSyncService_StartSync_UnknownProvider(t *testing.T) {
	svc := NewSyncService(nil, nil, nil, nil, map[string]ProviderFactory{
		"walmart": nil,
	})

	_, err := svc.StartSync(nil, SyncRequest{
		Provider: "unknown",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid provider")
}

func TestSyncService_GetSyncJob_NotFound(t *testing.T) {
	svc := NewSyncService(nil, nil, nil, nil, nil)

	_, err := svc.GetSyncJob("non-existent")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSyncService_ListActiveSyncJobs_Empty(t *testing.T) {
	svc := NewSyncService(nil, nil, nil, nil, nil)

	jobs := svc.ListActiveSyncJobs()

	assert.Empty(t, jobs)
}

func TestSyncService_ListAllSyncJobs_Empty(t *testing.T) {
	svc := NewSyncService(nil, nil, nil, nil, nil)

	jobs := svc.ListAllSyncJobs()

	assert.Empty(t, jobs)
}

func TestSyncService_CancelSync_NotFound(t *testing.T) {
	svc := NewSyncService(nil, nil, nil, nil, nil)

	err := svc.CancelSync("non-existent")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSyncStatus_String(t *testing.T) {
	assert.Equal(t, "pending", string(StatusPending))
	assert.Equal(t, "running", string(StatusRunning))
	assert.Equal(t, "completed", string(StatusCompleted))
	assert.Equal(t, "failed", string(StatusFailed))
	assert.Equal(t, "cancelled", string(StatusCancelled))
}

func TestSyncRequest_Defaults(t *testing.T) {
	req := SyncRequest{
		Provider: "walmart",
	}

	// Default values should be zero values
	assert.False(t, req.DryRun)
	assert.Equal(t, 0, req.LookbackDays)
	assert.Equal(t, 0, req.MaxOrders)
	assert.False(t, req.Force)
	assert.False(t, req.Verbose)
	assert.Empty(t, req.OrderID)
}

func TestSyncProgress_Initial(t *testing.T) {
	progress := SyncProgress{
		CurrentPhase: "fetching_orders",
	}

	assert.Equal(t, "fetching_orders", progress.CurrentPhase)
	assert.Equal(t, 0, progress.TotalOrders)
	assert.Equal(t, 0, progress.ProcessedOrders)
	assert.Equal(t, 0, progress.SkippedOrders)
	assert.Equal(t, 0, progress.ErroredOrders)
}

func TestSyncJob_Initial(t *testing.T) {
	job := SyncJob{
		ID:       "test-job-123",
		Provider: "walmart",
		Status:   StatusPending,
	}

	assert.Equal(t, "test-job-123", job.ID)
	assert.Equal(t, "walmart", job.Provider)
	assert.Equal(t, StatusPending, job.Status)
	assert.Nil(t, job.Result)
	assert.Nil(t, job.Error)
}
