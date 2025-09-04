package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"monarchmoney-sync-backend/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestReceiveBatchOrders_Success(t *testing.T) {
	// Test successful batch order processing
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/walmart/orders/batch", ReceiveBatchOrders)

	orderTotal1 := 50.00
	orderTotal2 := 75.50
	batchRequest := models.BatchOrdersRequest{
		Orders: []models.Order{
			{
				OrderNumber: "123",
				OrderDate:   "2024-01-15",
				OrderTotal:  &orderTotal1,
				Items: []models.OrderItem{
					{
						Name:     "Item 1",
						Price:    25.00,
						Quantity: 2,
					},
				},
			},
			{
				OrderNumber: "456",
				OrderDate:   "2024-01-16",
				OrderTotal:  &orderTotal2,
			},
		},
	}

	jsonData, _ := json.Marshal(batchRequest)

	// Act
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/walmart/orders/batch", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Extension-Key", "test-secret")
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var response models.BatchOrdersResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, 2, response.ProcessedCount)
	assert.Equal(t, 0, response.FailedCount)
	assert.Len(t, response.Results, 2)

	// Check individual results
	assert.Equal(t, "123", response.Results[0].OrderNumber)
	assert.True(t, response.Results[0].Success)
	assert.NotEmpty(t, response.Results[0].ProcessingID)
	assert.Empty(t, response.Results[0].Error)

	assert.Equal(t, "456", response.Results[1].OrderNumber)
	assert.True(t, response.Results[1].Success)
	assert.NotEmpty(t, response.Results[1].ProcessingID)
	assert.Empty(t, response.Results[1].Error)
}

func TestReceiveBatchOrders_PartialFailure(t *testing.T) {
	// Test batch with some invalid orders
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/walmart/orders/batch", ReceiveBatchOrders)

	orderTotal := 50.00
	negativePrice := -10.00
	batchRequest := models.BatchOrdersRequest{
		Orders: []models.Order{
			{
				OrderNumber: "123",
				OrderDate:   "2024-01-15",
				OrderTotal:  &orderTotal,
			},
			{
				// Missing required OrderDate
				OrderNumber: "456",
			},
			{
				OrderNumber: "789",
				OrderDate:   "2024-01-17",
				Items: []models.OrderItem{
					{
						Name:     "Invalid Item",
						Price:    negativePrice,
						Quantity: 1,
					},
				},
			},
		},
	}

	jsonData, _ := json.Marshal(batchRequest)

	// Act
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/walmart/orders/batch", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Extension-Key", "test-secret")
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var response models.BatchOrdersResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.Success) // Overall success even with partial failures
	assert.Equal(t, 1, response.ProcessedCount)
	assert.Equal(t, 2, response.FailedCount)
	assert.Len(t, response.Results, 3)

	// First order should succeed
	assert.True(t, response.Results[0].Success)
	assert.Empty(t, response.Results[0].Error)

	// Second order should fail (missing date)
	assert.False(t, response.Results[1].Success)
	assert.NotEmpty(t, response.Results[1].Error)

	// Third order should fail (negative price)
	assert.False(t, response.Results[2].Success)
	assert.Contains(t, response.Results[2].Error, "price")
}

func TestReceiveBatchOrders_EmptyBatch(t *testing.T) {
	// Test empty batch request
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/walmart/orders/batch", ReceiveBatchOrders)

	batchRequest := models.BatchOrdersRequest{
		Orders: []models.Order{},
	}

	jsonData, _ := json.Marshal(batchRequest)

	// Act
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/walmart/orders/batch", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Extension-Key", "test-secret")
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "error", response["status"])
	assert.Contains(t, response["message"], "No orders provided")
}

func TestReceiveBatchOrders_InvalidJSON(t *testing.T) {
	// Test invalid JSON in batch request
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/walmart/orders/batch", ReceiveBatchOrders)

	invalidJSON := []byte(`{"orders": [{"invalid": json}]}`)

	// Act
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/walmart/orders/batch", bytes.NewBuffer(invalidJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Extension-Key", "test-secret")
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "error", response["status"])
	assert.Contains(t, response["message"], "Invalid JSON")
}

func TestReceiveBatchOrders_MissingAuth(t *testing.T) {
	// Test batch endpoint with missing auth
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(AuthMiddleware())
	router.POST("/api/walmart/orders/batch", ReceiveBatchOrders)

	batchRequest := models.BatchOrdersRequest{
		Orders: []models.Order{
			{
				OrderNumber: "123",
				OrderDate:   "2024-01-15",
			},
		},
	}

	jsonData, _ := json.Marshal(batchRequest)

	// Act
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/walmart/orders/batch", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	// Note: NOT setting X-Extension-Key header
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "error", response["status"])
	assert.Contains(t, response["message"], "Unauthorized")
}

func TestReceiveBatchOrders_ValidationErrors(t *testing.T) {
	// Test various validation error scenarios
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/walmart/orders/batch", ReceiveBatchOrders)

	testCases := []struct {
		name        string
		orders      []models.Order
		expectError string
	}{
		{
			name: "Missing order number",
			orders: []models.Order{
				{
					OrderDate: "2024-01-15",
				},
			},
			expectError: "missing order number",
		},
		{
			name: "Empty order number",
			orders: []models.Order{
				{
					OrderNumber: "",
					OrderDate:   "2024-01-15",
				},
			},
			expectError: "missing order number",
		},
		{
			name: "Missing order date",
			orders: []models.Order{
				{
					OrderNumber: "123",
				},
			},
			expectError: "missing order date",
		},
		{
			name: "Invalid item quantity zero",
			orders: []models.Order{
				{
					OrderNumber: "123",
					OrderDate:   "2024-01-15",
					Items: []models.OrderItem{
						{
							Name:     "Test",
							Price:    10.00,
							Quantity: 0,
						},
					},
				},
			},
			expectError: "invalid quantity",
		},
		{
			name: "Invalid item quantity negative",
			orders: []models.Order{
				{
					OrderNumber: "123",
					OrderDate:   "2024-01-15",
					Items: []models.OrderItem{
						{
							Name:     "Test",
							Price:    10.00,
							Quantity: -1,
						},
					},
				},
			},
			expectError: "invalid quantity",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			batchRequest := models.BatchOrdersRequest{Orders: tc.orders}
			jsonData, _ := json.Marshal(batchRequest)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/walmart/orders/batch", bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Extension-Key", "test-secret")
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response models.BatchOrdersResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Equal(t, 0, response.ProcessedCount)
			assert.Equal(t, 1, response.FailedCount)
			assert.Contains(t, response.Results[0].Error, tc.expectError)
		})
	}
}

func TestLogBatchOrder(_ *testing.T) {
	// Test logBatchOrder function directly
	orderTotal := 100.00
	order := models.Order{
		OrderNumber: "TEST123",
		OrderDate:   "2024-01-15",
		OrderTotal:  &orderTotal,
		Items: []models.OrderItem{
			{Name: "Item1", Price: 50.00, Quantity: 1},
			{Name: "Item2", Price: 50.00, Quantity: 1},
		},
	}

	// This should not panic and should log properly
	logBatchOrder(order)
	
	// Test with nil items
	orderNoItems := models.Order{
		OrderNumber: "TEST456",
		OrderDate:   "2024-01-16",
		OrderTotal:  &orderTotal,
	}
	logBatchOrder(orderNoItems)
	
	// Test with nil total
	orderNoTotal := models.Order{
		OrderNumber: "TEST789",
		OrderDate:   "2024-01-17",
		Items: []models.OrderItem{
			{Name: "Item1", Price: 25.00, Quantity: 2},
		},
	}
	logBatchOrder(orderNoTotal)
}

