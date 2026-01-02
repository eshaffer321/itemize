package api_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/api"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/api/dto"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/storage"
)

func newTestServer(t *testing.T) (*api.Server, *storage.MockRepository) {
	t.Helper()
	repo := storage.NewMockRepository()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	server := api.NewServer(api.DefaultConfig(), repo, nil, nil, logger) // nil syncService, nil monarchClient for read-only tests
	return server, repo
}

func TestServer_HealthEndpoint(t *testing.T) {
	server, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	server.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response dto.HealthResponse
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "ok", response.Status)
}

func TestServer_OrdersEndpoints(t *testing.T) {
	t.Run("GET /api/orders returns orders", func(t *testing.T) {
		server, repo := newTestServer(t)
		repo.AddRecord(&storage.ProcessingRecord{
			OrderID:   "ORDER-1",
			Provider:  "walmart",
			OrderDate: time.Now(),
			Status:    "success",
		})

		req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
		rec := httptest.NewRecorder()

		server.Router().ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response dto.OrderListResponse
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, 1, response.TotalCount)
	})

	t.Run("GET /api/orders/:id returns single order", func(t *testing.T) {
		server, repo := newTestServer(t)
		repo.AddRecord(&storage.ProcessingRecord{
			OrderID:   "ORDER-123",
			Provider:  "costco",
			OrderDate: time.Now(),
			Status:    "success",
		})

		req := httptest.NewRequest(http.MethodGet, "/api/orders/ORDER-123", nil)
		rec := httptest.NewRecorder()

		server.Router().ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response dto.OrderResponse
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, "ORDER-123", response.OrderID)
	})

	t.Run("GET /api/orders/:id returns 404 for missing order", func(t *testing.T) {
		server, _ := newTestServer(t)

		req := httptest.NewRequest(http.MethodGet, "/api/orders/MISSING", nil)
		rec := httptest.NewRecorder()

		server.Router().ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestServer_ItemsEndpoints(t *testing.T) {
	t.Run("GET /api/items/search returns matching items", func(t *testing.T) {
		server, repo := newTestServer(t)
		repo.AddRecord(&storage.ProcessingRecord{
			OrderID:   "ORDER-1",
			Provider:  "walmart",
			OrderDate: time.Now(),
			Items: []storage.OrderItem{
				{Name: "Milk", TotalPrice: 3.99},
			},
		})

		req := httptest.NewRequest(http.MethodGet, "/api/items/search?q=milk", nil)
		rec := httptest.NewRecorder()

		server.Router().ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response dto.ItemSearchResponse
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, 1, response.Count)
	})

	t.Run("GET /api/items/search returns 400 without query", func(t *testing.T) {
		server, _ := newTestServer(t)

		req := httptest.NewRequest(http.MethodGet, "/api/items/search", nil)
		rec := httptest.NewRecorder()

		server.Router().ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestServer_RunsEndpoints(t *testing.T) {
	t.Run("GET /api/runs returns runs", func(t *testing.T) {
		server, repo := newTestServer(t)
		runID, _ := repo.StartSyncRun("walmart", 14, false)
		_ = repo.CompleteSyncRun(runID, 10, 8, 1, 1)

		req := httptest.NewRequest(http.MethodGet, "/api/runs", nil)
		rec := httptest.NewRecorder()

		server.Router().ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response dto.SyncRunListResponse
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, 1, response.Count)
	})

	t.Run("GET /api/runs/:id returns single run", func(t *testing.T) {
		server, repo := newTestServer(t)
		runID, _ := repo.StartSyncRun("walmart", 14, false)
		_ = repo.CompleteSyncRun(runID, 10, 8, 1, 1)

		req := httptest.NewRequest(http.MethodGet, "/api/runs/1", nil)
		rec := httptest.NewRecorder()

		server.Router().ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response dto.SyncRunResponse
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, int64(1), response.ID)
	})
}

func TestServer_CORS(t *testing.T) {
	server, _ := newTestServer(t)

	t.Run("sets CORS headers for allowed origin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		rec := httptest.NewRecorder()

		server.Router().ServeHTTP(rec, req)

		assert.Equal(t, "http://localhost:3000", rec.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("handles OPTIONS preflight", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/api/orders", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		rec := httptest.NewRecorder()

		server.Router().ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}
