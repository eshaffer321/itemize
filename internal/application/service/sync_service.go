package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/clients"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
	appsync "github.com/eshaffer321/monarchmoney-sync-backend/internal/application/sync"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/config"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/logging"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/storage"
)

// SyncStatus represents the current state of a sync job.
type SyncStatus string

const (
	StatusPending   SyncStatus = "pending"
	StatusRunning   SyncStatus = "running"
	StatusCompleted SyncStatus = "completed"
	StatusFailed    SyncStatus = "failed"
	StatusCancelled SyncStatus = "cancelled"
)

// Job staleness thresholds
const (
	// DefaultJobStaleThreshold is how long a job can go without progress updates
	// before being considered stale. Jobs that don't update progress for this
	// duration are assumed to be hung or crashed.
	DefaultJobStaleThreshold = 30 * time.Minute

	// DefaultJobMaxDuration is the maximum time a job can run before being
	// forcefully marked as failed. This prevents runaway jobs.
	DefaultJobMaxDuration = 2 * time.Hour
)

// SyncRequest holds parameters for starting a sync.
type SyncRequest struct {
	Provider     string // "walmart", "costco", "amazon"
	DryRun       bool
	LookbackDays int
	MaxOrders    int
	Force        bool
	Verbose      bool
	OrderID      string // If set, only process this specific order
}

// SyncProgress holds real-time progress information.
type SyncProgress struct {
	CurrentPhase    string    // "pending", "initializing", "fetching_orders", "processing_orders", "completed", "failed"
	TotalOrders     int
	ProcessedOrders int
	SkippedOrders   int
	ErroredOrders   int
	LastUpdate      time.Time
}

// SyncJob represents a running or completed sync job.
type SyncJob struct {
	ID          string
	Provider    string
	Status      SyncStatus
	Request     SyncRequest
	StartedAt   time.Time
	CompletedAt *time.Time
	Progress    SyncProgress
	Result      *appsync.Result
	Error       error
	cancelFunc  context.CancelFunc
}

// ProviderFactory creates providers from config.
type ProviderFactory func(cfg *config.Config, verbose bool) (providers.OrderProvider, error)

// SyncService manages sync operations.
type SyncService struct {
	cfg              *config.Config
	clients          *clients.Clients
	storage          storage.Repository
	logger           *slog.Logger
	providerFactory  map[string]ProviderFactory

	// Job management
	jobs      map[string]*SyncJob
	jobsMutex sync.RWMutex

	// Provider-level locking (only one sync per provider at a time)
	providerLocks map[string]*sync.Mutex
	locksMutex    sync.Mutex

	// Background cleanup
	cleanupStop chan struct{}
	cleanupDone chan struct{}
}

// NewSyncService creates a new sync service.
func NewSyncService(
	cfg *config.Config,
	clients *clients.Clients,
	store storage.Repository,
	logger *slog.Logger,
	providerFactory map[string]ProviderFactory,
) *SyncService {
	return &SyncService{
		cfg:             cfg,
		clients:         clients,
		storage:         store,
		logger:          logger,
		providerFactory: providerFactory,
		jobs:            make(map[string]*SyncJob),
		providerLocks:   make(map[string]*sync.Mutex),
	}
}

// StartSync starts a new sync job asynchronously.
// Note: The passed context is NOT used as the parent for the background job.
// Background sync jobs use context.Background() to avoid being cancelled when
// the HTTP request completes. Use CancelSync() to cancel a running job.
func (s *SyncService) StartSync(_ context.Context, req SyncRequest) (string, error) {
	// Validate provider
	if !s.isValidProvider(req.Provider) {
		return "", fmt.Errorf("invalid provider: %s", req.Provider)
	}

	// Check if provider is already running a sync
	if !s.tryLockProvider(req.Provider) {
		return "", fmt.Errorf("sync already running for provider: %s", req.Provider)
	}

	// Create job ID
	jobID := s.generateJobID(req.Provider)

	// Create cancellable context from Background - NOT from the request context.
	// This prevents the job from being cancelled when the HTTP request completes.
	jobCtx, cancel := context.WithCancel(context.Background())

	// Create job
	job := &SyncJob{
		ID:         jobID,
		Provider:   req.Provider,
		Status:     StatusPending,
		Request:    req,
		StartedAt:  time.Now(),
		cancelFunc: cancel,
		Progress:   SyncProgress{CurrentPhase: "pending", LastUpdate: time.Now()},
	}

	// Store job
	s.jobsMutex.Lock()
	s.jobs[jobID] = job
	s.jobsMutex.Unlock()

	// Start background goroutine
	go s.runSyncJob(jobCtx, job)

	s.logger.Info("sync job started",
		"job_id", jobID,
		"provider", req.Provider,
		"dry_run", req.DryRun,
		"lookback_days", req.LookbackDays,
	)

	return jobID, nil
}

// GetSyncJob retrieves a sync job by ID.
func (s *SyncService) GetSyncJob(jobID string) (*SyncJob, error) {
	s.jobsMutex.RLock()
	defer s.jobsMutex.RUnlock()

	job, exists := s.jobs[jobID]
	if !exists {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	return job, nil
}

// ListActiveSyncJobs returns all running or pending jobs.
func (s *SyncService) ListActiveSyncJobs() []*SyncJob {
	s.jobsMutex.RLock()
	defer s.jobsMutex.RUnlock()

	var active []*SyncJob
	for _, job := range s.jobs {
		if job.Status == StatusPending || job.Status == StatusRunning {
			active = append(active, job)
		}
	}
	return active
}

// ListAllSyncJobs returns all jobs (for debugging/monitoring).
func (s *SyncService) ListAllSyncJobs() []*SyncJob {
	s.jobsMutex.RLock()
	defer s.jobsMutex.RUnlock()

	jobs := make([]*SyncJob, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// CancelSync cancels a running sync job.
func (s *SyncService) CancelSync(jobID string) error {
	s.jobsMutex.Lock()
	defer s.jobsMutex.Unlock()

	job, exists := s.jobs[jobID]
	if !exists {
		return fmt.Errorf("job not found: %s", jobID)
	}

	if job.Status != StatusPending && job.Status != StatusRunning {
		return fmt.Errorf("job cannot be cancelled: status=%s", job.Status)
	}

	job.cancelFunc()
	job.Status = StatusCancelled
	now := time.Now()
	job.CompletedAt = &now
	job.Progress.CurrentPhase = "cancelled"
	job.Progress.LastUpdate = now

	s.logger.Info("sync job cancelled", "job_id", jobID)
	return nil
}

// runSyncJob executes the sync job in a background goroutine.
func (s *SyncService) runSyncJob(ctx context.Context, job *SyncJob) {
	defer s.unlockProvider(job.Provider)

	// Update status to running
	s.updateJobStatus(job.ID, StatusRunning, SyncProgress{
		CurrentPhase: "initializing",
		LastUpdate:   time.Now(),
	})

	// Create provider
	factory, ok := s.providerFactory[job.Request.Provider]
	if !ok {
		s.failJob(job.ID, fmt.Errorf("no factory for provider: %s", job.Request.Provider))
		return
	}

	provider, err := factory(s.cfg, job.Request.Verbose)
	if err != nil {
		s.failJob(job.ID, fmt.Errorf("failed to create provider: %w", err))
		return
	}

	// Create logger for this sync
	loggingCfg := s.cfg.Observability.Logging
	if job.Request.Verbose {
		loggingCfg.Level = "debug"
	}
	syncLogger := logging.NewLoggerWithSystem(loggingCfg, "sync")

	// Create orchestrator
	orchestrator := appsync.NewOrchestrator(provider, s.clients, s.storage, syncLogger)

	// Update progress to fetching
	s.updateJobStatus(job.ID, StatusRunning, SyncProgress{
		CurrentPhase: "fetching_orders",
		LastUpdate:   time.Now(),
	})

	// Convert request to options with progress callback
	opts := appsync.Options{
		DryRun:       job.Request.DryRun,
		LookbackDays: job.Request.LookbackDays,
		MaxOrders:    job.Request.MaxOrders,
		Force:        job.Request.Force,
		Verbose:      job.Request.Verbose,
		OrderID:      job.Request.OrderID,
		ProgressCallback: func(update appsync.ProgressUpdate) {
			s.updateJobProgress(job.ID, update)
		},
	}

	// Run sync
	result, err := orchestrator.Run(ctx, opts)

	if err != nil {
		if ctx.Err() == context.Canceled {
			// Already marked as cancelled in CancelSync
			return
		}
		s.failJob(job.ID, err)
		return
	}

	// Mark as completed
	s.completeJob(job.ID, result)
}

// updateJobStatus updates a job's status and progress.
func (s *SyncService) updateJobStatus(jobID string, status SyncStatus, progress SyncProgress) {
	s.jobsMutex.Lock()
	defer s.jobsMutex.Unlock()

	if job, exists := s.jobs[jobID]; exists {
		job.Status = status
		job.Progress = progress
	}
}

// updateJobProgress updates job progress from orchestrator callback.
func (s *SyncService) updateJobProgress(jobID string, update appsync.ProgressUpdate) {
	s.jobsMutex.Lock()
	defer s.jobsMutex.Unlock()

	if job, exists := s.jobs[jobID]; exists {
		job.Progress.CurrentPhase = update.Phase
		job.Progress.TotalOrders = update.TotalOrders
		job.Progress.ProcessedOrders = update.ProcessedOrders
		job.Progress.SkippedOrders = update.SkippedOrders
		job.Progress.ErroredOrders = update.ErroredOrders
		job.Progress.LastUpdate = time.Now()
	}
}

// completeJob marks a job as completed with results.
func (s *SyncService) completeJob(jobID string, result *appsync.Result) {
	s.jobsMutex.Lock()
	defer s.jobsMutex.Unlock()

	if job, exists := s.jobs[jobID]; exists {
		now := time.Now()
		job.Status = StatusCompleted
		job.CompletedAt = &now
		job.Result = result
		// Preserve TotalOrders from the existing progress while updating other fields
		job.Progress.CurrentPhase = "completed"
		job.Progress.ProcessedOrders = result.ProcessedCount
		job.Progress.SkippedOrders = result.SkippedCount
		job.Progress.ErroredOrders = result.ErrorCount
		job.Progress.LastUpdate = now
		s.logger.Info("sync job completed",
			"job_id", jobID,
			"total", job.Progress.TotalOrders,
			"processed", result.ProcessedCount,
			"skipped", result.SkippedCount,
			"errors", result.ErrorCount,
		)
	}
}

// failJob marks a job as failed with an error.
func (s *SyncService) failJob(jobID string, err error) {
	s.jobsMutex.Lock()
	defer s.jobsMutex.Unlock()

	if job, exists := s.jobs[jobID]; exists {
		now := time.Now()
		job.Status = StatusFailed
		job.CompletedAt = &now
		job.Error = err
		job.Progress = SyncProgress{
			CurrentPhase: "failed",
			LastUpdate:   now,
		}
		s.logger.Error("sync job failed", "job_id", jobID, "error", err)
	}
}

// tryLockProvider attempts to acquire the lock for a provider.
func (s *SyncService) tryLockProvider(provider string) bool {
	s.locksMutex.Lock()
	defer s.locksMutex.Unlock()

	if _, exists := s.providerLocks[provider]; !exists {
		s.providerLocks[provider] = &sync.Mutex{}
	}

	return s.providerLocks[provider].TryLock()
}

// unlockProvider releases the lock for a provider.
func (s *SyncService) unlockProvider(provider string) {
	s.locksMutex.Lock()
	defer s.locksMutex.Unlock()

	if lock, exists := s.providerLocks[provider]; exists {
		lock.Unlock()
	}
}

// isValidProvider checks if a provider is valid.
func (s *SyncService) isValidProvider(provider string) bool {
	_, exists := s.providerFactory[provider]
	return exists
}

// generateJobID creates a unique job ID.
func (s *SyncService) generateJobID(provider string) string {
	return fmt.Sprintf("%s-%d", provider, time.Now().UnixNano())
}

// CleanupOldJobs removes completed jobs older than the specified duration.
func (s *SyncService) CleanupOldJobs(maxAge time.Duration) int {
	s.jobsMutex.Lock()
	defer s.jobsMutex.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for id, job := range s.jobs {
		// Only remove completed jobs
		if job.Status == StatusCompleted || job.Status == StatusFailed || job.Status == StatusCancelled {
			if job.CompletedAt != nil && job.CompletedAt.Before(cutoff) {
				delete(s.jobs, id)
				removed++
			}
		}
	}

	if removed > 0 {
		s.logger.Debug("cleaned up old sync jobs", "removed", removed)
	}

	return removed
}

// MarkStaleJobsAsFailed finds jobs that appear to be stuck and marks them as failed.
// A job is considered stale if:
// 1. It has been running longer than maxDuration, OR
// 2. Its Progress.LastUpdate is older than staleThreshold
//
// This handles cases where:
// - The goroutine panicked and never updated the job status
// - The job is genuinely stuck (infinite loop, deadlock, etc.)
// - The server restarted and orphaned in-memory job state
func (s *SyncService) MarkStaleJobsAsFailed(staleThreshold, maxDuration time.Duration) int {
	s.jobsMutex.Lock()
	defer s.jobsMutex.Unlock()

	now := time.Now()
	marked := 0

	for id, job := range s.jobs {
		// Only check running or pending jobs
		if job.Status != StatusRunning && job.Status != StatusPending {
			continue
		}

		isStale := false
		reason := ""

		// Check if job has exceeded max duration
		if now.Sub(job.StartedAt) > maxDuration {
			isStale = true
			reason = fmt.Sprintf("exceeded max duration of %v (started %v ago)", maxDuration, now.Sub(job.StartedAt).Round(time.Second))
		}

		// Check if progress hasn't been updated recently
		if !isStale && now.Sub(job.Progress.LastUpdate) > staleThreshold {
			isStale = true
			reason = fmt.Sprintf("no progress update for %v (threshold: %v)", now.Sub(job.Progress.LastUpdate).Round(time.Second), staleThreshold)
		}

		if isStale {
			// Cancel the context if it exists (in case goroutine is still running)
			if job.cancelFunc != nil {
				job.cancelFunc()
			}

			// Mark as failed
			job.Status = StatusFailed
			job.CompletedAt = &now
			job.Error = fmt.Errorf("job marked as stale: %s", reason)
			job.Progress.CurrentPhase = "failed"
			job.Progress.LastUpdate = now

			// Release the provider lock
			s.releaseProviderLockUnsafe(job.Provider)

			s.logger.Warn("marked stale job as failed",
				"job_id", id,
				"provider", job.Provider,
				"reason", reason,
				"started_at", job.StartedAt,
				"last_update", job.Progress.LastUpdate,
			)

			marked++
		}
	}

	return marked
}

// releaseProviderLockUnsafe releases a provider lock without acquiring locksMutex.
// MUST only be called while holding jobsMutex to avoid races.
func (s *SyncService) releaseProviderLockUnsafe(provider string) {
	s.locksMutex.Lock()
	defer s.locksMutex.Unlock()

	if lock, exists := s.providerLocks[provider]; exists {
		// TryLock then Unlock ensures we don't panic if already unlocked
		if lock.TryLock() {
			lock.Unlock()
		} else {
			// Lock is held, so unlock it
			lock.Unlock()
		}
	}
}

// IsJobStale checks if a specific job is considered stale.
func (s *SyncService) IsJobStale(jobID string, staleThreshold, maxDuration time.Duration) bool {
	s.jobsMutex.RLock()
	defer s.jobsMutex.RUnlock()

	job, exists := s.jobs[jobID]
	if !exists {
		return false
	}

	if job.Status != StatusRunning && job.Status != StatusPending {
		return false
	}

	now := time.Now()
	return now.Sub(job.StartedAt) > maxDuration || now.Sub(job.Progress.LastUpdate) > staleThreshold
}

// StartBackgroundCleanup starts a background goroutine that periodically:
// 1. Marks stale jobs as failed
// 2. Cleans up old completed jobs
//
// The cleanup runs every checkInterval. Call StopBackgroundCleanup to stop it.
func (s *SyncService) StartBackgroundCleanup(checkInterval time.Duration) {
	s.cleanupStop = make(chan struct{})
	s.cleanupDone = make(chan struct{})

	go func() {
		defer close(s.cleanupDone)

		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()

		s.logger.Info("background job cleanup started",
			"check_interval", checkInterval,
			"stale_threshold", DefaultJobStaleThreshold,
			"max_duration", DefaultJobMaxDuration,
		)

		for {
			select {
			case <-s.cleanupStop:
				s.logger.Info("background job cleanup stopped")
				return
			case <-ticker.C:
				// Mark stale jobs as failed
				staleMarked := s.MarkStaleJobsAsFailed(DefaultJobStaleThreshold, DefaultJobMaxDuration)
				if staleMarked > 0 {
					s.logger.Info("marked stale jobs as failed", "count", staleMarked)
				}

				// Clean up old completed jobs (keep for 24 hours)
				cleaned := s.CleanupOldJobs(24 * time.Hour)
				if cleaned > 0 {
					s.logger.Debug("cleaned up old jobs", "count", cleaned)
				}
			}
		}
	}()
}

// StopBackgroundCleanup stops the background cleanup goroutine.
// This method blocks until the cleanup goroutine has fully stopped.
func (s *SyncService) StopBackgroundCleanup() {
	if s.cleanupStop == nil {
		return
	}

	close(s.cleanupStop)
	<-s.cleanupDone
}
