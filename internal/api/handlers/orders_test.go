package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/api/dto"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/api/handlers"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/storage"
)

func TestOrdersHandler_List(t *testing.T) {
	t.Run("returns empty list when no orders", func(t *testing.T) {
		repo := storage.NewMockRepository()
		handler := handlers.NewOrdersHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
		rec := httptest.NewRecorder()

		handler.List(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		var response dto.OrderListResponse
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		assert.Empty(t, response.Orders)
		assert.Equal(t, 0, response.TotalCount)
		assert.Equal(t, 50, response.Limit) // default limit
		assert.Equal(t, 0, response.Offset)
	})

	t.Run("returns orders from repository", func(t *testing.T) {
		repo := storage.NewMockRepository()
		repo.AddRecord(&storage.ProcessingRecord{
			OrderID:     "ORDER-1",
			Provider:    "walmart",
			OrderDate:   time.Now(),
			ProcessedAt: time.Now(),
			OrderTotal:  100.50,
			Status:      "success",
			ItemCount:   3,
			Items: []storage.OrderItem{
				{Name: "Milk", Quantity: 1, UnitPrice: 3.99, TotalPrice: 3.99, Category: "Groceries"},
			},
		})
		repo.AddRecord(&storage.ProcessingRecord{
			OrderID:     "ORDER-2",
			Provider:    "costco",
			OrderDate:   time.Now(),
			ProcessedAt: time.Now(),
			OrderTotal:  250.00,
			Status:      "success",
			ItemCount:   5,
		})

		handler := handlers.NewOrdersHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
		rec := httptest.NewRecorder()

		handler.List(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response dto.OrderListResponse
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, 2, response.TotalCount)
		assert.Len(t, response.Orders, 2)
	})

	t.Run("filters by provider", func(t *testing.T) {
		repo := storage.NewMockRepository()
		repo.AddRecord(&storage.ProcessingRecord{
			OrderID:  "ORDER-1",
			Provider: "walmart",
			Status:   "success",
		})
		repo.AddRecord(&storage.ProcessingRecord{
			OrderID:  "ORDER-2",
			Provider: "costco",
			Status:   "success",
		})

		handler := handlers.NewOrdersHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/orders?provider=walmart", nil)
		rec := httptest.NewRecorder()

		handler.List(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response dto.OrderListResponse
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, 1, response.TotalCount)
		assert.Equal(t, "walmart", response.Orders[0].Provider)
	})

	t.Run("filters by status", func(t *testing.T) {
		repo := storage.NewMockRepository()
		repo.AddRecord(&storage.ProcessingRecord{
			OrderID:  "ORDER-1",
			Provider: "walmart",
			Status:   "success",
		})
		repo.AddRecord(&storage.ProcessingRecord{
			OrderID:  "ORDER-2",
			Provider: "walmart",
			Status:   "failed",
		})

		handler := handlers.NewOrdersHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/orders?status=failed", nil)
		rec := httptest.NewRecorder()

		handler.List(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response dto.OrderListResponse
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, 1, response.TotalCount)
		assert.Equal(t, "failed", response.Orders[0].Status)
	})

	t.Run("respects pagination params", func(t *testing.T) {
		repo := storage.NewMockRepository()
		for i := 0; i < 10; i++ {
			repo.AddRecord(&storage.ProcessingRecord{
				OrderID:  "ORDER-" + string(rune('A'+i)),
				Provider: "walmart",
				Status:   "success",
			})
		}

		handler := handlers.NewOrdersHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/orders?limit=3&offset=2", nil)
		rec := httptest.NewRecorder()

		handler.List(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response dto.OrderListResponse
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, 10, response.TotalCount)
		assert.Len(t, response.Orders, 3)
		assert.Equal(t, 3, response.Limit)
		assert.Equal(t, 2, response.Offset)
	})
}

func TestOrdersHandler_Get(t *testing.T) {
	t.Run("returns order by ID", func(t *testing.T) {
		repo := storage.NewMockRepository()
		repo.AddRecord(&storage.ProcessingRecord{
			OrderID:     "ORDER-123",
			Provider:    "walmart",
			OrderDate:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			ProcessedAt: time.Date(2024, 1, 16, 10, 30, 0, 0, time.UTC),
			OrderTotal:  150.00,
			Status:      "success",
			ItemCount:   2,
			Items: []storage.OrderItem{
				{Name: "Milk", Quantity: 2, UnitPrice: 3.99, TotalPrice: 7.98, Category: "Groceries"},
				{Name: "Bread", Quantity: 1, UnitPrice: 2.50, TotalPrice: 2.50, Category: "Groceries"},
			},
			Splits: []storage.SplitDetail{
				{CategoryID: "cat-1", CategoryName: "Groceries", Amount: 10.48},
			},
		})

		handler := handlers.NewOrdersHandler(repo)

		// Set up chi router context for URL params
		req := httptest.NewRequest(http.MethodGet, "/api/orders/ORDER-123", nil)
		req = req.WithContext(setChiURLParam(req.Context(), "id", "ORDER-123"))

		rec := httptest.NewRecorder()

		handler.Get(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response dto.OrderResponse
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, "ORDER-123", response.OrderID)
		assert.Equal(t, "walmart", response.Provider)
		assert.Equal(t, 150.00, response.OrderTotal)
		assert.Len(t, response.Items, 2)
		assert.Len(t, response.Splits, 1)
	})

	t.Run("returns 404 for non-existent order", func(t *testing.T) {
		repo := storage.NewMockRepository()
		handler := handlers.NewOrdersHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/orders/NONEXISTENT", nil)
		req = req.WithContext(setChiURLParam(req.Context(), "id", "NONEXISTENT"))

		rec := httptest.NewRecorder()

		handler.Get(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)

		var response dto.APIError
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, dto.ErrCodeNotFound, response.Code)
	})
}

// Helper to set chi URL param in context
func setChiURLParam(ctx context.Context, key, value string) context.Context {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return context.WithValue(ctx, chi.RouteCtxKey, rctx)
}
