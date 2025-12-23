package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/api/dto"
)

// HealthHandler handles health check requests.
type HealthHandler struct{}

// NewHealthHandler creates a new health handler.
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// ServeHTTP handles the health check request.
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := dto.NewHealthResponse()
	_ = json.NewEncoder(w).Encode(response)
}
