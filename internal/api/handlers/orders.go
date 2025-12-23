package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/api/dto"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/storage"
)

// OrdersHandler handles order-related HTTP requests.
type OrdersHandler struct {
	*Base
}

// NewOrdersHandler creates a new orders handler.
func NewOrdersHandler(repo storage.Repository) *OrdersHandler {
	return &OrdersHandler{
		Base: NewBase(repo),
	}
}

// List handles GET /api/orders - returns paginated list of orders.
func (h *OrdersHandler) List(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	filters := storage.OrderFilters{
		Provider:  r.URL.Query().Get("provider"),
		Status:    r.URL.Query().Get("status"),
		Search:    r.URL.Query().Get("search"),
		DaysBack:  ParseIntParam(r, "days_back", 0),
		Limit:     ParseIntParam(r, "limit", 50),
		Offset:    ParseIntParam(r, "offset", 0),
		OrderBy:   r.URL.Query().Get("order_by"),
		OrderDesc: ParseBoolParam(r, "order_desc", true),
	}

	// Query repository
	result, err := h.repo.ListOrders(filters)
	if err != nil {
		h.WriteError(w, http.StatusInternalServerError, dto.InternalError())
		return
	}

	// Convert to response
	response := dto.OrderListResponse{
		Orders:     make([]dto.OrderResponse, 0, len(result.Orders)),
		TotalCount: result.TotalCount,
		Limit:      result.Limit,
		Offset:     result.Offset,
	}

	for _, order := range result.Orders {
		response.Orders = append(response.Orders, toOrderResponse(order))
	}

	h.WriteJSON(w, http.StatusOK, response)
}

// Get handles GET /api/orders/{id} - returns a single order by ID.
func (h *OrdersHandler) Get(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "id")
	if orderID == "" {
		h.WriteError(w, http.StatusBadRequest, dto.BadRequestError("order ID is required"))
		return
	}

	record, err := h.repo.GetRecord(orderID)
	if err != nil {
		h.WriteError(w, http.StatusInternalServerError, dto.InternalError())
		return
	}

	if record == nil {
		h.WriteError(w, http.StatusNotFound, dto.NotFoundError("order"))
		return
	}

	response := toOrderResponse(record)
	h.WriteJSON(w, http.StatusOK, response)
}

// toOrderResponse converts a storage record to an API response.
func toOrderResponse(record *storage.ProcessingRecord) dto.OrderResponse {
	response := dto.OrderResponse{
		OrderID:           record.OrderID,
		Provider:          record.Provider,
		TransactionID:     record.TransactionID,
		OrderDate:         record.OrderDate.Format("2006-01-02"),
		ProcessedAt:       record.ProcessedAt.Format("2006-01-02T15:04:05Z"),
		OrderTotal:        record.OrderTotal,
		OrderSubtotal:     record.OrderSubtotal,
		OrderTax:          record.OrderTax,
		OrderTip:          record.OrderTip,
		TransactionAmount: record.TransactionAmount,
		Status:            record.Status,
		ErrorMessage:      record.ErrorMessage,
		ItemCount:         record.ItemCount,
		SplitCount:        record.SplitCount,
		MatchConfidence:   record.MatchConfidence,
		DryRun:            record.DryRun,
		Items:             make([]dto.ItemResponse, 0, len(record.Items)),
		Splits:            make([]dto.SplitResponse, 0, len(record.Splits)),
	}

	for _, item := range record.Items {
		response.Items = append(response.Items, dto.ItemResponse{
			Name:       item.Name,
			Quantity:   item.Quantity,
			UnitPrice:  item.UnitPrice,
			TotalPrice: item.TotalPrice,
			Category:   item.Category,
		})
	}

	for _, split := range record.Splits {
		splitResp := dto.SplitResponse{
			CategoryID:   split.CategoryID,
			CategoryName: split.CategoryName,
			Amount:       split.Amount,
			Notes:        split.Notes,
			Items:        make([]dto.ItemResponse, 0, len(split.Items)),
		}
		for _, item := range split.Items {
			splitResp.Items = append(splitResp.Items, dto.ItemResponse{
				Name:       item.Name,
				Quantity:   item.Quantity,
				UnitPrice:  item.UnitPrice,
				TotalPrice: item.TotalPrice,
				Category:   item.Category,
			})
		}
		response.Splits = append(response.Splits, splitResp)
	}

	return response
}
