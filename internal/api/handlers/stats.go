package handlers

import (
	"net/http"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/api/dto"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/storage"
)

// StatsHandler handles stats-related HTTP requests.
type StatsHandler struct {
	*Base
}

// NewStatsHandler creates a new stats handler.
func NewStatsHandler(repo storage.Repository) *StatsHandler {
	return &StatsHandler{
		Base: NewBase(repo),
	}
}

// Get handles GET /api/stats - returns aggregate statistics.
func (h *StatsHandler) Get(w http.ResponseWriter, r *http.Request) {
	stats, err := h.repo.GetStats()
	if err != nil {
		h.WriteError(w, http.StatusInternalServerError, dto.InternalError())
		return
	}

	// Convert provider stats map to slice for easier frontend consumption
	providers := make([]dto.ProviderStatsResponse, 0, len(stats.ProviderStats))
	for provider, pStats := range stats.ProviderStats {
		providers = append(providers, dto.ProviderStatsResponse{
			Provider:     provider,
			Count:        pStats.Count,
			SuccessCount: pStats.SuccessCount,
			TotalAmount:  pStats.TotalAmount,
		})
	}

	response := dto.StatsResponse{
		TotalProcessed:     stats.TotalProcessed,
		SuccessCount:       stats.SuccessCount,
		FailedCount:        stats.FailedCount,
		SkippedCount:       stats.SkippedCount,
		DryRunCount:        stats.DryRunCount,
		TotalAmount:        stats.TotalAmount,
		AverageOrderAmount: stats.AverageOrderAmount,
		TotalSplits:        stats.TotalSplits,
		ProviderStats:      providers,
	}

	h.WriteJSON(w, http.StatusOK, response)
}
