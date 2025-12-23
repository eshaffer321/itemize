package api_test

import (
	"encoding/json"
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

// =============================================================================
// API Integration Tests
// =============================================================================
// These tests use real SQLite databases to test the full stack:
// HTTP request → Router → Handlers → Storage → SQLite
//
// This catches issues that mock-based tests miss, like:
// - SQL NULL handling errors (the bug we fixed!)
// - JSON serialization through the full pipeline
// - Router configuration and middleware

func createTestServer(t *testing.T) (*httptest.Server, *storage.Storage, func()) {
	t.Helper()

	// Create temp database
	tmpFile, err := os.CreateTemp("", "api_integration_*.db")
	require.NoError(t, err)
	tmpFile.Close()

	// Create real storage
	store, err := storage.NewStorage(tmpFile.Name())
	require.NoError(t, err)

	// Create real server with real storage
	cfg := api.DefaultConfig()
	server := api.NewServer(cfg, store, nil) // nil logger = use default

	// Create test server
	ts := httptest.NewServer(server.Router())

	cleanup := func() {
		ts.Close()
		store.Close()
		os.Remove(tmpFile.Name())
	}

	return ts, store, cleanup
}

func TestAPI_Integration_HealthCheck(t *testing.T) {
	ts, _, cleanup := createTestServer(t)
	defer cleanup()

	resp, err := http.Get(ts.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var health dto.HealthResponse
	err = json.NewDecoder(resp.Body).Decode(&health)
	require.NoError(t, err)

	assert.Equal(t, "ok", health.Status)
}

func TestAPI_Integration_ListOrders_Empty(t *testing.T) {
	ts, _, cleanup := createTestServer(t)
	defer cleanup()

	resp, err := http.Get(ts.URL + "/api/orders")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result dto.OrderListResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, 0, result.TotalCount)
	assert.Empty(t, result.Orders)
}

func TestAPI_Integration_ListOrders_WithData(t *testing.T) {
	ts, store, cleanup := createTestServer(t)
	defer cleanup()

	// Insert test data
	now := time.Now()
	records := []*storage.ProcessingRecord{
		{
			OrderID:     "ORDER-1",
			Provider:    "walmart",
			Status:      "success",
			OrderTotal:  100.00,
			OrderDate:   now,
			ProcessedAt: now,
			Items: []storage.OrderItem{
				{Name: "Milk", Quantity: 1, UnitPrice: 3.99, TotalPrice: 3.99, Category: "Groceries"},
			},
		},
		{
			OrderID:     "ORDER-2",
			Provider:    "costco",
			Status:      "failed",
			OrderTotal:  200.00,
			OrderDate:   now,
			ProcessedAt: now,
		},
	}

	for _, r := range records {
		err := store.SaveRecord(r)
		require.NoError(t, err)
	}

	t.Run("list all orders", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/orders")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result dto.OrderListResponse
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, 2, result.TotalCount)
		assert.Len(t, result.Orders, 2)
	})

	t.Run("filter by provider", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/orders?provider=walmart")
		require.NoError(t, err)
		defer resp.Body.Close()

		var result dto.OrderListResponse
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, 1, result.TotalCount)
		assert.Equal(t, "walmart", result.Orders[0].Provider)
	})

	t.Run("filter by status", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/orders?status=failed")
		require.NoError(t, err)
		defer resp.Body.Close()

		var result dto.OrderListResponse
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, 1, result.TotalCount)
		assert.Equal(t, "failed", result.Orders[0].Status)
	})

	t.Run("pagination", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/orders?limit=1&offset=0")
		require.NoError(t, err)
		defer resp.Body.Close()

		var result dto.OrderListResponse
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, 2, result.TotalCount)
		assert.Len(t, result.Orders, 1)
		assert.Equal(t, 1, result.Limit)
		assert.Equal(t, 0, result.Offset)
	})
}

func TestAPI_Integration_GetOrder(t *testing.T) {
	ts, store, cleanup := createTestServer(t)
	defer cleanup()

	// Insert test record
	now := time.Now()
	record := &storage.ProcessingRecord{
		OrderID:     "ORDER-GET-TEST",
		Provider:    "walmart",
		Status:      "success",
		OrderTotal:  99.99,
		OrderDate:   now,
		ProcessedAt: now,
		Items: []storage.OrderItem{
			{Name: "Test Item", Quantity: 2, UnitPrice: 10.00, TotalPrice: 20.00, Category: "Test"},
		},
		Splits: []storage.SplitDetail{
			{CategoryID: "cat-1", CategoryName: "Test", Amount: 20.00},
		},
	}
	err := store.SaveRecord(record)
	require.NoError(t, err)

	t.Run("get existing order", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/orders/ORDER-GET-TEST")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var order dto.OrderResponse
		err = json.NewDecoder(resp.Body).Decode(&order)
		require.NoError(t, err)

		assert.Equal(t, "ORDER-GET-TEST", order.OrderID)
		assert.Equal(t, "walmart", order.Provider)
		assert.Equal(t, 99.99, order.OrderTotal)
		assert.Len(t, order.Items, 1)
		assert.Equal(t, "Test Item", order.Items[0].Name)
		assert.Len(t, order.Splits, 1)
	})

	t.Run("get non-existent order returns 404", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/orders/DOES-NOT-EXIST")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)

		var apiErr dto.APIError
		err = json.NewDecoder(resp.Body).Decode(&apiErr)
		require.NoError(t, err)

		assert.Equal(t, dto.ErrCodeNotFound, apiErr.Code)
	})
}

// TestAPI_Integration_NullHandling verifies that records with NULL database values
// are handled correctly through the full API stack. This tests the exact bug we fixed.
func TestAPI_Integration_NullHandling(t *testing.T) {
	ts, store, cleanup := createTestServer(t)
	defer cleanup()

	// Insert a record with NULL values directly via SQL (simulating legacy data)
	_, err := store.DB().Exec(`
		INSERT INTO processing_records (
			order_id, provider, transaction_id, order_date, processed_at,
			order_total, order_subtotal, order_tax, order_tip, transaction_amount,
			split_count, status, error_message, item_count, match_confidence,
			dry_run, items_json, splits_json, multi_delivery_data
		) VALUES (
			'ORDER-NULL-API', 'walmart', NULL, datetime('now'), datetime('now'),
			100.00, 90.00, 10.00, 0, -100.00,
			0, 'success', NULL, 0, 0.0,
			0, NULL, NULL, NULL
		)
	`)
	require.NoError(t, err)

	t.Run("list orders with NULL values works", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/orders")
		require.NoError(t, err)
		defer resp.Body.Close()

		// This would have returned 500 before the NULL fix
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result dto.OrderListResponse
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, 1, result.TotalCount)
		order := result.Orders[0]
		assert.Equal(t, "ORDER-NULL-API", order.OrderID)
		assert.Equal(t, "", order.TransactionID) // NULL → empty string
		assert.Equal(t, "", order.ErrorMessage)  // NULL → empty string
		assert.Empty(t, order.Items)             // NULL → empty slice
	})

	t.Run("get single order with NULL values works", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/orders/ORDER-NULL-API")
		require.NoError(t, err)
		defer resp.Body.Close()

		// This would have returned 500 before the NULL fix
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var order dto.OrderResponse
		err = json.NewDecoder(resp.Body).Decode(&order)
		require.NoError(t, err)

		assert.Equal(t, "ORDER-NULL-API", order.OrderID)
		assert.Equal(t, "", order.TransactionID)
		assert.Equal(t, "", order.ErrorMessage)
	})
}

func TestAPI_Integration_SearchItems(t *testing.T) {
	ts, store, cleanup := createTestServer(t)
	defer cleanup()

	// Insert test records with items
	now := time.Now()
	record := &storage.ProcessingRecord{
		OrderID:     "ORDER-SEARCH",
		Provider:    "walmart",
		Status:      "success",
		OrderDate:   now,
		ProcessedAt: now,
		Items: []storage.OrderItem{
			{Name: "Organic Milk", TotalPrice: 5.99, Category: "Groceries"},
			{Name: "Whole Wheat Bread", TotalPrice: 3.49, Category: "Groceries"},
		},
	}
	err := store.SaveRecord(record)
	require.NoError(t, err)

	t.Run("search finds items", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/items/search?q=milk")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result dto.ItemSearchResponse
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, "milk", result.Query)
		assert.Equal(t, 1, result.Count)
		assert.Equal(t, "Organic Milk", result.Items[0].ItemName)
	})

	t.Run("search without query returns 400", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/items/search")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("search with no matches returns empty", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/items/search?q=nonexistent")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result dto.ItemSearchResponse
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, 0, result.Count)
		assert.Empty(t, result.Items)
	})
}

func TestAPI_Integration_SyncRuns(t *testing.T) {
	ts, store, cleanup := createTestServer(t)
	defer cleanup()

	// Create some sync runs
	runID1, err := store.StartSyncRun("walmart", 14, false)
	require.NoError(t, err)
	err = store.CompleteSyncRun(runID1, 10, 8, 1, 1)
	require.NoError(t, err)

	_, err = store.StartSyncRun("costco", 7, true)
	require.NoError(t, err)
	// Leave second run as "running" (not completed)

	t.Run("list sync runs", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/runs")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result dto.SyncRunListResponse
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, 2, result.Count)
	})

	t.Run("get single run", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/runs/1")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var run dto.SyncRunResponse
		err = json.NewDecoder(resp.Body).Decode(&run)
		require.NoError(t, err)

		assert.Equal(t, int64(1), run.ID)
		assert.Equal(t, "walmart", run.Provider)
		assert.Equal(t, 14, run.LookbackDays)
		// Status is "completed_with_errors" because we have 1 error (ordersErrored=1)
		assert.Equal(t, "completed_with_errors", run.Status)
	})

	t.Run("get non-existent run returns 404", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/runs/9999")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("invalid run ID returns 400", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/runs/not-a-number")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestAPI_Integration_CORS(t *testing.T) {
	ts, _, cleanup := createTestServer(t)
	defer cleanup()

	// Test preflight request
	req, err := http.NewRequest(http.MethodOptions, ts.URL+"/api/orders", nil)
	require.NoError(t, err)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Equal(t, "http://localhost:3000", resp.Header.Get("Access-Control-Allow-Origin"))
}
