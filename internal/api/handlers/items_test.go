package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/api/dto"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/api/handlers"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/storage"
)

func TestItemsHandler_Search(t *testing.T) {
	t.Run("returns empty results when no matches", func(t *testing.T) {
		repo := storage.NewMockRepository()
		handler := handlers.NewItemsHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/items/search?q=nonexistent", nil)
		rec := httptest.NewRecorder()

		handler.Search(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response dto.ItemSearchResponse
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		assert.Empty(t, response.Items)
		assert.Equal(t, "nonexistent", response.Query)
		assert.Equal(t, 0, response.Count)
	})

	t.Run("returns matching items", func(t *testing.T) {
		repo := storage.NewMockRepository()
		repo.AddRecord(&storage.ProcessingRecord{
			OrderID:   "ORDER-1",
			Provider:  "walmart",
			OrderDate: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			Items: []storage.OrderItem{
				{Name: "Organic Milk", TotalPrice: 5.99, Category: "Groceries"},
				{Name: "Bread", TotalPrice: 2.50, Category: "Groceries"},
			},
		})
		repo.AddRecord(&storage.ProcessingRecord{
			OrderID:   "ORDER-2",
			Provider:  "costco",
			OrderDate: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
			Items: []storage.OrderItem{
				{Name: "Almond Milk", TotalPrice: 4.99, Category: "Groceries"},
			},
		})

		handler := handlers.NewItemsHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/items/search?q=milk", nil)
		rec := httptest.NewRecorder()

		handler.Search(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response dto.ItemSearchResponse
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, "milk", response.Query)
		assert.Equal(t, 2, response.Count)
		assert.Len(t, response.Items, 2)
	})

	t.Run("respects limit parameter", func(t *testing.T) {
		repo := storage.NewMockRepository()
		repo.AddRecord(&storage.ProcessingRecord{
			OrderID:   "ORDER-1",
			Provider:  "walmart",
			OrderDate: time.Now(),
			Items: []storage.OrderItem{
				{Name: "Item A", TotalPrice: 1.00},
				{Name: "Item B", TotalPrice: 2.00},
				{Name: "Item C", TotalPrice: 3.00},
			},
		})

		handler := handlers.NewItemsHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/items/search?q=item&limit=2", nil)
		rec := httptest.NewRecorder()

		handler.Search(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response dto.ItemSearchResponse
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		assert.Len(t, response.Items, 2)
	})

	t.Run("returns 400 when query is missing", func(t *testing.T) {
		repo := storage.NewMockRepository()
		handler := handlers.NewItemsHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/items/search", nil)
		rec := httptest.NewRecorder()

		handler.Search(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)

		var response dto.APIError
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, dto.ErrCodeBadRequest, response.Code)
	})

	t.Run("result contains order context", func(t *testing.T) {
		repo := storage.NewMockRepository()
		repo.AddRecord(&storage.ProcessingRecord{
			OrderID:   "ORDER-123",
			Provider:  "costco",
			OrderDate: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
			Items: []storage.OrderItem{
				{Name: "Shampoo", TotalPrice: 8.99, Category: "Personal Care"},
			},
		})

		handler := handlers.NewItemsHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/items/search?q=shampoo", nil)
		rec := httptest.NewRecorder()

		handler.Search(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response dto.ItemSearchResponse
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		require.Len(t, response.Items, 1)
		item := response.Items[0]
		assert.Equal(t, "ORDER-123", item.OrderID)
		assert.Equal(t, "costco", item.Provider)
		assert.Equal(t, "Shampoo", item.ItemName)
		assert.Equal(t, 8.99, item.ItemPrice)
		assert.Equal(t, "Personal Care", item.Category)
	})
}
