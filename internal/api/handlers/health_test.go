package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/api/dto"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/api/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthHandler_ServeHTTP(t *testing.T) {
	t.Run("returns 200 OK with health status", func(t *testing.T) {
		handler := handlers.NewHealthHandler()

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		var response dto.HealthResponse
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, "ok", response.Status)
		assert.NotEmpty(t, response.Timestamp)
	})
}
