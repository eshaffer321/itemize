package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"monarchmoney-sync-backend/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGetSyncStatus_Success(t *testing.T) {
	// Test getting sync status
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/walmart/sync-status", GetSyncStatus)

	// Act
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/walmart/sync-status", nil)
	req.Header.Set("X-Extension-Key", "test-secret")
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var response models.SyncStatusResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "operational", response.Status)
	assert.GreaterOrEqual(t, response.OrdersProcessedToday, 0)
	assert.GreaterOrEqual(t, response.OrdersProcessedTotal, 0)
}

func TestGetSyncStatus_WithRecentSync(t *testing.T) {
	// Test sync status after processing orders
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// First process an order to update sync status
	router.POST("/api/walmart/orders", ReceiveOrders)
	router.GET("/api/walmart/sync-status", GetSyncStatus)

	orderTotal := 50.00
	order := models.Order{
		OrderNumber: "test-123",
		OrderDate:   "2024-01-15",
		OrderTotal:  &orderTotal,
	}

	// Process an order first
	jsonData, _ := json.Marshal(order)
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("POST", "/api/walmart/orders", bytes.NewBuffer(jsonData))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("X-Extension-Key", "test-secret")
	router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	// Now check sync status
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/walmart/sync-status", nil)
	req2.Header.Set("X-Extension-Key", "test-secret")
	router.ServeHTTP(w2, req2)

	// Assert
	assert.Equal(t, http.StatusOK, w2.Code)

	var response models.SyncStatusResponse
	err := json.Unmarshal(w2.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "operational", response.Status)
	assert.NotNil(t, response.LastSyncTimestamp)
	assert.True(t, response.LastSyncTimestamp.After(time.Now().Add(-1*time.Minute)))
	assert.GreaterOrEqual(t, response.OrdersProcessedToday, 1)
	assert.GreaterOrEqual(t, response.OrdersProcessedTotal, 1)
}

func TestGetSyncStatus_MissingAuth(t *testing.T) {
	// Test sync status endpoint with missing auth
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(AuthMiddleware())
	router.GET("/api/walmart/sync-status", GetSyncStatus)

	// Act
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/walmart/sync-status", nil)
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

