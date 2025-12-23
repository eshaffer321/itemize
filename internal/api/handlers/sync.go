package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/api/dto"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/application/service"
)

// SyncHandler handles sync-related HTTP requests.
type SyncHandler struct {
	*Base
	syncService *service.SyncService
}

// NewSyncHandler creates a new sync handler.
func NewSyncHandler(syncService *service.SyncService) *SyncHandler {
	return &SyncHandler{
		Base:        &Base{},
		syncService: syncService,
	}
}

// StartSync handles POST /api/sync - starts a new sync job.
func (h *SyncHandler) StartSync(w http.ResponseWriter, r *http.Request) {
	var req dto.StartSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.WriteError(w, http.StatusBadRequest, dto.BadRequestError("invalid request body"))
		return
	}

	// Validate request
	if req.Provider == "" {
		h.WriteError(w, http.StatusBadRequest, dto.BadRequestError("provider is required"))
		return
	}

	// Set defaults
	if req.LookbackDays <= 0 {
		req.LookbackDays = 14
	}

	// Convert to service request
	serviceReq := service.SyncRequest{
		Provider:     req.Provider,
		DryRun:       req.DryRun,
		LookbackDays: req.LookbackDays,
		MaxOrders:    req.MaxOrders,
		Force:        req.Force,
		Verbose:      req.Verbose,
		OrderID:      req.OrderID,
	}

	// Start sync
	jobID, err := h.syncService.StartSync(r.Context(), serviceReq)
	if err != nil {
		h.WriteError(w, http.StatusConflict, dto.APIError{
			Code:    "sync_conflict",
			Message: err.Error(),
		})
		return
	}

	// Return job ID
	response := dto.StartSyncResponse{
		JobID:    jobID,
		Provider: req.Provider,
		Status:   "pending",
	}

	h.WriteJSON(w, http.StatusAccepted, response)
}

// GetSyncStatus handles GET /api/sync/{jobId} - gets sync job status.
func (h *SyncHandler) GetSyncStatus(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")
	if jobID == "" {
		h.WriteError(w, http.StatusBadRequest, dto.BadRequestError("job ID is required"))
		return
	}

	job, err := h.syncService.GetSyncJob(jobID)
	if err != nil {
		h.WriteError(w, http.StatusNotFound, dto.NotFoundError("sync job"))
		return
	}

	response := toSyncJobResponse(job)
	h.WriteJSON(w, http.StatusOK, response)
}

// ListActiveSyncs handles GET /api/sync/active - lists active sync jobs.
func (h *SyncHandler) ListActiveSyncs(w http.ResponseWriter, r *http.Request) {
	jobs := h.syncService.ListActiveSyncJobs()

	response := dto.ActiveSyncsResponse{
		Jobs:  make([]dto.SyncJobResponse, 0, len(jobs)),
		Count: len(jobs),
	}

	for _, job := range jobs {
		response.Jobs = append(response.Jobs, toSyncJobResponse(job))
	}

	h.WriteJSON(w, http.StatusOK, response)
}

// ListAllSyncs handles GET /api/sync - lists all sync jobs.
func (h *SyncHandler) ListAllSyncs(w http.ResponseWriter, r *http.Request) {
	jobs := h.syncService.ListAllSyncJobs()

	response := dto.AllSyncsResponse{
		Jobs:  make([]dto.SyncJobResponse, 0, len(jobs)),
		Count: len(jobs),
	}

	for _, job := range jobs {
		response.Jobs = append(response.Jobs, toSyncJobResponse(job))
	}

	h.WriteJSON(w, http.StatusOK, response)
}

// CancelSync handles DELETE /api/sync/{jobId} - cancels a sync job.
func (h *SyncHandler) CancelSync(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")
	if jobID == "" {
		h.WriteError(w, http.StatusBadRequest, dto.BadRequestError("job ID is required"))
		return
	}

	if err := h.syncService.CancelSync(jobID); err != nil {
		h.WriteError(w, http.StatusConflict, dto.APIError{
			Code:    "cancel_failed",
			Message: err.Error(),
		})
		return
	}

	h.WriteJSON(w, http.StatusOK, dto.MessageResponse{
		Message: "Sync job cancelled successfully",
	})
}

// toSyncJobResponse converts a service model to an API response.
func toSyncJobResponse(job *service.SyncJob) dto.SyncJobResponse {
	response := dto.SyncJobResponse{
		JobID:     job.ID,
		Provider:  job.Provider,
		Status:    string(job.Status),
		DryRun:    job.Request.DryRun,
		StartedAt: job.StartedAt.Format(time.RFC3339),
		Progress:  toProgressResponse(job.Progress),
	}

	if job.CompletedAt != nil {
		completedAt := job.CompletedAt.Format(time.RFC3339)
		response.CompletedAt = &completedAt
	}

	if job.Result != nil {
		response.Result = &dto.SyncResultResponse{
			ProcessedCount: job.Result.ProcessedCount,
			SkippedCount:   job.Result.SkippedCount,
			ErrorCount:     job.Result.ErrorCount,
		}
	}

	if job.Error != nil {
		errMsg := job.Error.Error()
		response.Error = &errMsg
	}

	return response
}

// toProgressResponse converts progress to API response.
func toProgressResponse(progress service.SyncProgress) dto.SyncProgressResponse {
	return dto.SyncProgressResponse{
		CurrentPhase:    progress.CurrentPhase,
		TotalOrders:     progress.TotalOrders,
		ProcessedOrders: progress.ProcessedOrders,
		SkippedOrders:   progress.SkippedOrders,
		ErroredOrders:   progress.ErroredOrders,
		LastUpdate:      progress.LastUpdate.Format(time.RFC3339),
	}
}
