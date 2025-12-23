package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/api/dto"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/storage"
)

// RunsHandler handles sync run-related HTTP requests.
type RunsHandler struct {
	*Base
}

// NewRunsHandler creates a new runs handler.
func NewRunsHandler(repo storage.Repository) *RunsHandler {
	return &RunsHandler{
		Base: NewBase(repo),
	}
}

// List handles GET /api/runs - returns list of sync runs.
func (h *RunsHandler) List(w http.ResponseWriter, r *http.Request) {
	limit := ParseIntParam(r, "limit", 20)

	runs, err := h.repo.ListSyncRuns(limit)
	if err != nil {
		h.WriteError(w, http.StatusInternalServerError, dto.InternalError())
		return
	}

	response := dto.SyncRunListResponse{
		Runs:  make([]dto.SyncRunResponse, 0, len(runs)),
		Count: len(runs),
	}

	for _, run := range runs {
		response.Runs = append(response.Runs, toSyncRunResponse(run))
	}

	h.WriteJSON(w, http.StatusOK, response)
}

// Get handles GET /api/runs/{id} - returns a single sync run by ID.
func (h *RunsHandler) Get(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		h.WriteError(w, http.StatusBadRequest, dto.BadRequestError("run ID is required"))
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.WriteError(w, http.StatusBadRequest, dto.BadRequestError("invalid run ID"))
		return
	}

	run, err := h.repo.GetSyncRun(id)
	if err != nil {
		h.WriteError(w, http.StatusInternalServerError, dto.InternalError())
		return
	}

	if run == nil {
		h.WriteError(w, http.StatusNotFound, dto.NotFoundError("sync run"))
		return
	}

	response := toSyncRunResponse(*run)
	h.WriteJSON(w, http.StatusOK, response)
}

// toSyncRunResponse converts a storage SyncRun to an API response.
func toSyncRunResponse(run storage.SyncRun) dto.SyncRunResponse {
	return dto.SyncRunResponse{
		ID:              run.ID,
		Provider:        run.Provider,
		StartedAt:       run.StartedAt,
		CompletedAt:     run.CompletedAt,
		LookbackDays:    run.LookbackDays,
		DryRun:          run.DryRun,
		OrdersFound:     run.OrdersFound,
		OrdersProcessed: run.OrdersProcessed,
		OrdersSkipped:   run.OrdersSkipped,
		OrdersErrored:   run.OrdersErrored,
		Status:          run.Status,
	}
}
