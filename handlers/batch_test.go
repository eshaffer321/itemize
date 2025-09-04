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

