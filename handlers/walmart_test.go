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

func TestReceiveOrders_Success(t *testing.T) {
	// Arrange
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/walmart/orders", ReceiveOrders)

	orderTotal := 150.00
	order := models.Order{
		OrderNumber: "123456789",
		OrderDate:   "2024-01-15",
		OrderTotal:  &orderTotal,
		Items: []models.OrderItem{
			{
				Name:     "Great Value Milk",
				Price:    3.99,
				Quantity: 1,
			},
			{
				Name:     "Bounty Paper Towels",
				Price:    12.99,
				Quantity: 2,
			},
		},
	}

	jsonData, _ := json.Marshal(order)

	// Act
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/walmart/orders", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Extension-Key", "test-secret")
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var response models.OrderResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "success", response.Status)
	assert.Equal(t, "123456789", response.OrderID)
}

func TestReceiveOrders_InvalidJSON(t *testing.T) {
	// Arrange
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/walmart/orders", ReceiveOrders)

	invalidJSON := []byte(`{"invalid": json}`)

	// Act
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/walmart/orders", bytes.NewBuffer(invalidJSON))
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

func TestReceiveOrders_MissingAuth(t *testing.T) {
	// Arrange
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(AuthMiddleware())
	router.POST("/api/walmart/orders", ReceiveOrders)

	orderTotal := 150.00
	order := models.Order{
		OrderNumber: "123456789",
		OrderDate:   "2024-01-15",
		OrderTotal:  &orderTotal,
		Items: []models.OrderItem{
			{
				Name:     "Great Value Milk",
				Price:    3.99,
				Quantity: 1,
			},
		},
	}

	jsonData, _ := json.Marshal(order)

	// Act
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/walmart/orders", bytes.NewBuffer(jsonData))
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

func TestReceiveOrders_EmptyOrder(t *testing.T) {
	// Arrange
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/walmart/orders", ReceiveOrders)

	order := models.Order{
		// Missing required fields
	}

	jsonData, _ := json.Marshal(order)

	// Act
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/walmart/orders", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Extension-Key", "test-secret")
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "error", response["status"])
	assert.Contains(t, response["message"], "validation")
}

func TestReceiveOrders_WithoutOrderTotal(t *testing.T) {
	// Test that orders without orderTotal are accepted
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/walmart/orders", ReceiveOrders)

	order := models.Order{
		OrderNumber: "123456789",
		OrderDate:   "2024-01-15",
		// No OrderTotal field
		Items: []models.OrderItem{
			{
				Name:     "Great Value Milk",
				Price:    3.99,
				Quantity: 1,
			},
		},
	}

	jsonData, _ := json.Marshal(order)

	// Act
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/walmart/orders", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Extension-Key", "test-secret")
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var response models.OrderResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "success", response.Status)
	assert.Equal(t, 1, response.ItemCount)
	assert.Nil(t, response.TotalAmount)
}

func TestReceiveOrders_WithoutItems(t *testing.T) {
	// Test that orders without items are accepted
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/walmart/orders", ReceiveOrders)

	orderTotal := 150.00
	order := models.Order{
		OrderNumber: "123456789",
		OrderDate:   "2024-01-15",
		OrderTotal:  &orderTotal,
		// No Items field
	}

	jsonData, _ := json.Marshal(order)

	// Act
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/walmart/orders", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Extension-Key", "test-secret")
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var response models.OrderResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "success", response.Status)
	assert.Equal(t, 0, response.ItemCount)
	assert.Equal(t, 150.00, *response.TotalAmount)
}

func TestReceiveOrders_WithAdditionalFields(t *testing.T) {
	// Test that orders with new optional fields are handled properly
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/walmart/orders", ReceiveOrders)

	orderTotal := 150.00
	tax := 12.50
	deliveryCharges := 5.99
	tip := 10.00

	order := models.Order{
		OrderNumber:     "123456789",
		OrderDate:       "2024-01-15",
		OrderTotal:      &orderTotal,
		Tax:             &tax,
		DeliveryCharges: &deliveryCharges,
		Tip:             &tip,
		Items: []models.OrderItem{
			{
				Name:       "Great Value Milk",
				Price:      3.99,
				Quantity:   1,
				ProductURL: "https://walmart.com/product/123",
				Category:   "Groceries",
			},
		},
	}

	jsonData, _ := json.Marshal(order)

	// Act
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/walmart/orders", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Extension-Key", "test-secret")
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var response models.OrderResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "success", response.Status)
	assert.NotEmpty(t, response.ProcessingID)
	assert.Equal(t, 1, response.ItemCount)
	assert.Equal(t, 150.00, *response.TotalAmount)
}

func TestReceiveOrders_ItemValidationErrors(t *testing.T) {
	// Test item validation error paths
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/walmart/orders", ReceiveOrders)

	testCases := []struct {
		name        string
		items       []models.OrderItem
		expectError string
	}{
		{
			name: "Negative item price",
			items: []models.OrderItem{
				{
					Name:     "Invalid Item",
					Price:    -10.00,
					Quantity: 1,
				},
			},
			expectError: "Invalid item price",
		},
		{
			name: "Zero item quantity",
			items: []models.OrderItem{
				{
					Name:     "Invalid Item",
					Price:    10.00,
					Quantity: 0,
				},
			},
			expectError: "Invalid item quantity",
		},
		{
			name: "Negative item quantity",
			items: []models.OrderItem{
				{
					Name:     "Invalid Item",
					Price:    10.00,
					Quantity: -1,
				},
			},
			expectError: "Invalid item quantity",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			order := models.Order{
				OrderNumber: "123456",
				OrderDate:   "2024-01-15",
				Items:       tc.items,
			}

			jsonData, _ := json.Marshal(order)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/walmart/orders", bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Extension-Key", "test-secret")
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Equal(t, "error", response["status"])
			assert.Contains(t, response["message"], tc.expectError)
		})
	}
}

func TestAuthMiddleware_EmptyKey(t *testing.T) {
	// Test auth middleware with empty key
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(AuthMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Test with empty header
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Extension-Key", "")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "error", response["status"])
	assert.Contains(t, response["message"], "Unauthorized")
}

func TestAuthMiddleware_WrongKey(t *testing.T) {
	// Test auth middleware with wrong key
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(AuthMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Test with wrong key
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Extension-Key", "wrong-key")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "error", response["status"])
	assert.Contains(t, response["message"], "Unauthorized")
}
