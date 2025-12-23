package handlers

import (
	"net/http"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/api/dto"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/storage"
)

// ItemsHandler handles item-related HTTP requests.
type ItemsHandler struct {
	*Base
}

// NewItemsHandler creates a new items handler.
func NewItemsHandler(repo storage.Repository) *ItemsHandler {
	return &ItemsHandler{
		Base: NewBase(repo),
	}
}

// Search handles GET /api/items/search - searches for items across orders.
func (h *ItemsHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		h.WriteError(w, http.StatusBadRequest, dto.BadRequestError("search query 'q' is required"))
		return
	}

	limit := ParseIntParam(r, "limit", 50)

	results, err := h.repo.SearchItems(query, limit)
	if err != nil {
		h.WriteError(w, http.StatusInternalServerError, dto.InternalError())
		return
	}

	response := dto.ItemSearchResponse{
		Items: make([]dto.ItemSearchResultResponse, 0, len(results)),
		Query: query,
		Count: len(results),
	}

	for _, item := range results {
		response.Items = append(response.Items, dto.ItemSearchResultResponse{
			OrderID:   item.OrderID,
			Provider:  item.Provider,
			OrderDate: item.OrderDate,
			ItemName:  item.ItemName,
			ItemPrice: item.ItemPrice,
			Category:  item.Category,
		})
	}

	h.WriteJSON(w, http.StatusOK, response)
}
