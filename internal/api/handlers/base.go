package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/api/dto"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/storage"
)

// Base provides shared functionality for all handlers.
type Base struct {
	repo storage.Repository
}

// NewBase creates a new base handler with the given repository.
func NewBase(repo storage.Repository) *Base {
	return &Base{repo: repo}
}

// WriteJSON writes a JSON response with the given status code.
func (b *Base) WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// WriteError writes an error response with the given status code.
func (b *Base) WriteError(w http.ResponseWriter, status int, err dto.APIError) {
	b.WriteJSON(w, status, err)
}

// ParseIntParam parses an integer query parameter with a default value.
func ParseIntParam(r *http.Request, name string, defaultVal int) int {
	val := r.URL.Query().Get(name)
	if val == "" {
		return defaultVal
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return parsed
}

// ParseBoolParam parses a boolean query parameter with a default value.
func ParseBoolParam(r *http.Request, name string, defaultVal bool) bool {
	val := r.URL.Query().Get(name)
	if val == "" {
		return defaultVal
	}
	return val == "true" || val == "1"
}
