package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/api/dto"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/api/handlers"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/storage"
)

func TestRunsHandler_List(t *testing.T) {
	t.Run("returns empty list when no runs", func(t *testing.T) {
		repo := storage.NewMockRepository()
		handler := handlers.NewRunsHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/runs", nil)
		rec := httptest.NewRecorder()

		handler.List(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response dto.SyncRunListResponse
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		assert.Empty(t, response.Runs)
		assert.Equal(t, 0, response.Count)
	})

	t.Run("returns runs from repository", func(t *testing.T) {
		repo := storage.NewMockRepository()

		// Create some sync runs
		runID1, _ := repo.StartSyncRun("walmart", 14, false)
		_ = repo.CompleteSyncRun(runID1, 10, 8, 1, 1)

		runID2, _ := repo.StartSyncRun("costco", 7, true)
		_ = repo.CompleteSyncRun(runID2, 5, 5, 0, 0)

		handler := handlers.NewRunsHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/runs", nil)
		rec := httptest.NewRecorder()

		handler.List(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response dto.SyncRunListResponse
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, 2, response.Count)
		assert.Len(t, response.Runs, 2)
	})

	t.Run("respects limit parameter", func(t *testing.T) {
		repo := storage.NewMockRepository()

		for i := 0; i < 5; i++ {
			runID, _ := repo.StartSyncRun("walmart", 14, false)
			_ = repo.CompleteSyncRun(runID, 10, 10, 0, 0)
		}

		handler := handlers.NewRunsHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/runs?limit=3", nil)
		rec := httptest.NewRecorder()

		handler.List(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response dto.SyncRunListResponse
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		assert.Len(t, response.Runs, 3)
	})
}

func TestRunsHandler_Get(t *testing.T) {
	t.Run("returns run by ID", func(t *testing.T) {
		repo := storage.NewMockRepository()
		runID, _ := repo.StartSyncRun("walmart", 14, false)
		_ = repo.CompleteSyncRun(runID, 10, 8, 1, 1)

		handler := handlers.NewRunsHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/runs/1", nil)
		req = req.WithContext(setChiURLParam(req.Context(), "id", "1"))
		rec := httptest.NewRecorder()

		handler.Get(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response dto.SyncRunResponse
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, int64(1), response.ID)
		assert.Equal(t, "walmart", response.Provider)
		assert.Equal(t, 14, response.LookbackDays)
		assert.Equal(t, 10, response.OrdersFound)
		assert.Equal(t, 8, response.OrdersProcessed)
		assert.Equal(t, "completed", response.Status)
	})

	t.Run("returns 404 for non-existent run", func(t *testing.T) {
		repo := storage.NewMockRepository()
		handler := handlers.NewRunsHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/runs/999", nil)
		req = req.WithContext(setChiURLParam(req.Context(), "id", "999"))
		rec := httptest.NewRecorder()

		handler.Get(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)

		var response dto.APIError
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, dto.ErrCodeNotFound, response.Code)
	})

	t.Run("returns 400 for invalid ID", func(t *testing.T) {
		repo := storage.NewMockRepository()
		handler := handlers.NewRunsHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/runs/invalid", nil)
		req = req.WithContext(setChiURLParam(req.Context(), "id", "invalid"))
		rec := httptest.NewRecorder()

		handler.Get(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}
