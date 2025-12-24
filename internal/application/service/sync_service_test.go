package service

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

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

// Helper to create a test logger
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestSyncService_IsJobStale_NotFound(t *testing.T) {
	svc := NewSyncService(nil, nil, nil, testLogger(), nil)

	isStale := svc.IsJobStale("non-existent", 30*time.Minute, 2*time.Hour)

	assert.False(t, isStale)
}

func TestSyncService_IsJobStale_CompletedJobNotStale(t *testing.T) {
	svc := NewSyncService(nil, nil, nil, testLogger(), nil)

	// Manually add a completed job
	svc.jobsMutex.Lock()
	svc.jobs["completed-job"] = &SyncJob{
		ID:        "completed-job",
		Provider:  "walmart",
		Status:    StatusCompleted,
		StartedAt: time.Now().Add(-3 * time.Hour), // Old but completed
		Progress:  SyncProgress{LastUpdate: time.Now().Add(-2 * time.Hour)},
	}
	svc.jobsMutex.Unlock()

	// Completed jobs should never be considered stale
	isStale := svc.IsJobStale("completed-job", 30*time.Minute, 2*time.Hour)

	assert.False(t, isStale)
}

func TestSyncService_IsJobStale_RunningJob_StaleByProgress(t *testing.T) {
	svc := NewSyncService(nil, nil, nil, testLogger(), nil)

	// Add a running job with old progress update
	svc.jobsMutex.Lock()
	svc.jobs["stale-job"] = &SyncJob{
		ID:        "stale-job",
		Provider:  "walmart",
		Status:    StatusRunning,
		StartedAt: time.Now().Add(-10 * time.Minute), // Started 10 min ago
		Progress:  SyncProgress{LastUpdate: time.Now().Add(-35 * time.Minute)}, // No update for 35 min
	}
	svc.jobsMutex.Unlock()

	// Should be stale because progress hasn't updated in 35 minutes (> 30 min threshold)
	isStale := svc.IsJobStale("stale-job", 30*time.Minute, 2*time.Hour)

	assert.True(t, isStale)
}

func TestSyncService_IsJobStale_RunningJob_StaleByDuration(t *testing.T) {
	svc := NewSyncService(nil, nil, nil, testLogger(), nil)

	// Add a running job that's been running too long
	svc.jobsMutex.Lock()
	svc.jobs["long-job"] = &SyncJob{
		ID:        "long-job",
		Provider:  "walmart",
		Status:    StatusRunning,
		StartedAt: time.Now().Add(-3 * time.Hour), // Started 3 hours ago
		Progress:  SyncProgress{LastUpdate: time.Now()}, // Recent progress
	}
	svc.jobsMutex.Unlock()

	// Should be stale because it's been running longer than 2 hours max
	isStale := svc.IsJobStale("long-job", 30*time.Minute, 2*time.Hour)

	assert.True(t, isStale)
}

func TestSyncService_IsJobStale_RunningJob_NotStale(t *testing.T) {
	svc := NewSyncService(nil, nil, nil, testLogger(), nil)

	// Add a healthy running job
	svc.jobsMutex.Lock()
	svc.jobs["healthy-job"] = &SyncJob{
		ID:        "healthy-job",
		Provider:  "walmart",
		Status:    StatusRunning,
		StartedAt: time.Now().Add(-10 * time.Minute), // Started 10 min ago
		Progress:  SyncProgress{LastUpdate: time.Now().Add(-5 * time.Minute)}, // Updated 5 min ago
	}
	svc.jobsMutex.Unlock()

	// Should NOT be stale - running for short time, recently updated
	isStale := svc.IsJobStale("healthy-job", 30*time.Minute, 2*time.Hour)

	assert.False(t, isStale)
}

func TestSyncService_MarkStaleJobsAsFailed_MarksStaleJobs(t *testing.T) {
	svc := NewSyncService(nil, nil, nil, testLogger(), nil)

	// Add a stale job
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc.jobsMutex.Lock()
	svc.jobs["stale-job"] = &SyncJob{
		ID:         "stale-job",
		Provider:   "walmart",
		Status:     StatusRunning,
		StartedAt:  time.Now().Add(-3 * time.Hour),
		Progress:   SyncProgress{LastUpdate: time.Now().Add(-35 * time.Minute)},
		cancelFunc: cancel,
	}
	svc.jobsMutex.Unlock()

	// Mark stale jobs
	marked := svc.MarkStaleJobsAsFailed(30*time.Minute, 2*time.Hour)

	assert.Equal(t, 1, marked)

	// Verify job was marked as failed
	job, err := svc.GetSyncJob("stale-job")
	assert.NoError(t, err)
	assert.Equal(t, StatusFailed, job.Status)
	assert.NotNil(t, job.CompletedAt)
	assert.NotNil(t, job.Error)
	assert.Contains(t, job.Error.Error(), "stale")

	// Verify context was cancelled
	select {
	case <-ctx.Done():
		// Expected
	default:
		t.Error("context should have been cancelled")
	}
}

func TestSyncService_MarkStaleJobsAsFailed_SkipsHealthyJobs(t *testing.T) {
	svc := NewSyncService(nil, nil, nil, testLogger(), nil)

	// Add a healthy job
	svc.jobsMutex.Lock()
	svc.jobs["healthy-job"] = &SyncJob{
		ID:        "healthy-job",
		Provider:  "walmart",
		Status:    StatusRunning,
		StartedAt: time.Now().Add(-10 * time.Minute),
		Progress:  SyncProgress{LastUpdate: time.Now().Add(-5 * time.Minute)},
	}
	svc.jobsMutex.Unlock()

	// Try to mark stale jobs
	marked := svc.MarkStaleJobsAsFailed(30*time.Minute, 2*time.Hour)

	assert.Equal(t, 0, marked)

	// Verify job is still running
	job, err := svc.GetSyncJob("healthy-job")
	assert.NoError(t, err)
	assert.Equal(t, StatusRunning, job.Status)
}

func TestSyncService_MarkStaleJobsAsFailed_SkipsCompletedJobs(t *testing.T) {
	svc := NewSyncService(nil, nil, nil, testLogger(), nil)

	// Add a completed job that would appear "stale" if we checked it
	completedTime := time.Now().Add(-1 * time.Hour)
	svc.jobsMutex.Lock()
	svc.jobs["completed-job"] = &SyncJob{
		ID:          "completed-job",
		Provider:    "walmart",
		Status:      StatusCompleted,
		StartedAt:   time.Now().Add(-3 * time.Hour),
		CompletedAt: &completedTime,
		Progress:    SyncProgress{LastUpdate: completedTime},
	}
	svc.jobsMutex.Unlock()

	// Try to mark stale jobs
	marked := svc.MarkStaleJobsAsFailed(30*time.Minute, 2*time.Hour)

	assert.Equal(t, 0, marked)

	// Verify job is still completed (not changed to failed)
	job, err := svc.GetSyncJob("completed-job")
	assert.NoError(t, err)
	assert.Equal(t, StatusCompleted, job.Status)
}

func TestSyncService_CleanupOldJobs_RemovesOldCompletedJobs(t *testing.T) {
	svc := NewSyncService(nil, nil, nil, testLogger(), nil)

	// Add an old completed job
	oldTime := time.Now().Add(-25 * time.Hour)
	svc.jobsMutex.Lock()
	svc.jobs["old-job"] = &SyncJob{
		ID:          "old-job",
		Provider:    "walmart",
		Status:      StatusCompleted,
		CompletedAt: &oldTime,
	}
	svc.jobsMutex.Unlock()

	// Cleanup jobs older than 24 hours
	removed := svc.CleanupOldJobs(24 * time.Hour)

	assert.Equal(t, 1, removed)

	// Verify job was removed
	_, err := svc.GetSyncJob("old-job")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSyncService_CleanupOldJobs_KeepsRecentCompletedJobs(t *testing.T) {
	svc := NewSyncService(nil, nil, nil, testLogger(), nil)

	// Add a recently completed job
	recentTime := time.Now().Add(-1 * time.Hour)
	svc.jobsMutex.Lock()
	svc.jobs["recent-job"] = &SyncJob{
		ID:          "recent-job",
		Provider:    "walmart",
		Status:      StatusCompleted,
		CompletedAt: &recentTime,
	}
	svc.jobsMutex.Unlock()

	// Cleanup jobs older than 24 hours
	removed := svc.CleanupOldJobs(24 * time.Hour)

	assert.Equal(t, 0, removed)

	// Verify job still exists
	_, err := svc.GetSyncJob("recent-job")
	assert.NoError(t, err)
}

func TestSyncService_CleanupOldJobs_KeepsRunningJobs(t *testing.T) {
	svc := NewSyncService(nil, nil, nil, testLogger(), nil)

	// Add an old running job (shouldn't be removed by cleanup)
	svc.jobsMutex.Lock()
	svc.jobs["running-job"] = &SyncJob{
		ID:        "running-job",
		Provider:  "walmart",
		Status:    StatusRunning,
		StartedAt: time.Now().Add(-25 * time.Hour),
	}
	svc.jobsMutex.Unlock()

	// Cleanup old jobs
	removed := svc.CleanupOldJobs(24 * time.Hour)

	// Running jobs should NOT be removed
	assert.Equal(t, 0, removed)

	// Verify job still exists
	_, err := svc.GetSyncJob("running-job")
	assert.NoError(t, err)
}
