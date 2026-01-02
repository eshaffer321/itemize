package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/api/dto"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/storage"
)

// LedgersHandler handles ledger-related HTTP requests.
type LedgersHandler struct {
	*Base
}

// NewLedgersHandler creates a new ledgers handler.
func NewLedgersHandler(repo storage.Repository) *LedgersHandler {
	return &LedgersHandler{
		Base: NewBase(repo),
	}
}

// List handles GET /api/ledgers - returns paginated list of ledgers.
func (h *LedgersHandler) List(w http.ResponseWriter, r *http.Request) {
	filters := storage.LedgerFilters{
		OrderID:  r.URL.Query().Get("order_id"),
		Provider: r.URL.Query().Get("provider"),
		Limit:    ParseIntParam(r, "limit", 50),
		Offset:   ParseIntParam(r, "offset", 0),
	}

	// Parse state filter
	if state := r.URL.Query().Get("state"); state != "" {
		filters.State = storage.LedgerState(state)
	}

	result, err := h.repo.ListLedgers(filters)
	if err != nil {
		h.WriteError(w, http.StatusInternalServerError, dto.InternalError())
		return
	}

	response := dto.LedgerListResponse{
		Ledgers:    make([]dto.LedgerResponse, 0, len(result.Ledgers)),
		TotalCount: result.TotalCount,
		Limit:      result.Limit,
		Offset:     result.Offset,
	}

	for _, ledger := range result.Ledgers {
		response.Ledgers = append(response.Ledgers, toLedgerResponse(ledger))
	}

	h.WriteJSON(w, http.StatusOK, response)
}

// Get handles GET /api/ledgers/{id} - returns a single ledger by ID.
func (h *LedgersHandler) Get(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		h.WriteError(w, http.StatusBadRequest, dto.BadRequestError("ledger ID is required"))
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.WriteError(w, http.StatusBadRequest, dto.BadRequestError("invalid ledger ID"))
		return
	}

	ledger, err := h.repo.GetLedgerByID(id)
	if err != nil {
		h.WriteError(w, http.StatusInternalServerError, dto.InternalError())
		return
	}

	if ledger == nil {
		h.WriteError(w, http.StatusNotFound, dto.NotFoundError("ledger"))
		return
	}

	response := toLedgerResponse(ledger)
	h.WriteJSON(w, http.StatusOK, response)
}

// GetByOrderID handles GET /api/orders/{orderID}/ledger - returns the latest ledger for an order.
func (h *LedgersHandler) GetByOrderID(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")
	if orderID == "" {
		h.WriteError(w, http.StatusBadRequest, dto.BadRequestError("order ID is required"))
		return
	}

	ledger, err := h.repo.GetLatestLedger(orderID)
	if err != nil {
		h.WriteError(w, http.StatusInternalServerError, dto.InternalError())
		return
	}

	if ledger == nil {
		h.WriteError(w, http.StatusNotFound, dto.NotFoundError("ledger"))
		return
	}

	response := toLedgerResponse(ledger)
	h.WriteJSON(w, http.StatusOK, response)
}

// GetHistoryByOrderID handles GET /api/orders/{orderID}/ledgers - returns all ledgers for an order.
func (h *LedgersHandler) GetHistoryByOrderID(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")
	if orderID == "" {
		h.WriteError(w, http.StatusBadRequest, dto.BadRequestError("order ID is required"))
		return
	}

	ledgers, err := h.repo.GetLedgerHistory(orderID)
	if err != nil {
		h.WriteError(w, http.StatusInternalServerError, dto.InternalError())
		return
	}

	response := dto.LedgerListResponse{
		Ledgers:    make([]dto.LedgerResponse, 0, len(ledgers)),
		TotalCount: len(ledgers),
		Limit:      len(ledgers),
		Offset:     0,
	}

	for _, ledger := range ledgers {
		response.Ledgers = append(response.Ledgers, toLedgerResponse(ledger))
	}

	h.WriteJSON(w, http.StatusOK, response)
}

// toLedgerResponse converts a storage OrderLedger to an API response.
func toLedgerResponse(ledger *storage.OrderLedger) dto.LedgerResponse {
	response := dto.LedgerResponse{
		ID:                 ledger.ID,
		OrderID:            ledger.OrderID,
		SyncRunID:          ledger.SyncRunID,
		Provider:           ledger.Provider,
		FetchedAt:          ledger.FetchedAt.Format("2006-01-02T15:04:05Z"),
		LedgerState:        string(ledger.LedgerState),
		LedgerVersion:      ledger.LedgerVersion,
		TotalCharged:       ledger.TotalCharged,
		ChargeCount:        ledger.ChargeCount,
		PaymentMethodTypes: ledger.PaymentMethodTypes,
		HasRefunds:         ledger.HasRefunds,
		IsValid:            ledger.IsValid,
		ValidationNotes:    ledger.ValidationNotes,
		Charges:            make([]dto.ChargeResponse, 0, len(ledger.Charges)),
	}

	for _, charge := range ledger.Charges {
		chargeResp := dto.ChargeResponse{
			ID:                   charge.ID,
			ChargeSequence:       charge.ChargeSequence,
			ChargeAmount:         charge.ChargeAmount,
			ChargeType:           charge.ChargeType,
			PaymentMethod:        charge.PaymentMethod,
			CardType:             charge.CardType,
			CardLastFour:         charge.CardLastFour,
			MonarchTransactionID: charge.MonarchTransactionID,
			IsMatched:            charge.IsMatched,
			MatchConfidence:      charge.MatchConfidence,
			SplitCount:           charge.SplitCount,
		}
		// Only include charged_at if it's not zero
		if !charge.ChargedAt.IsZero() {
			chargeResp.ChargedAt = charge.ChargedAt.Format("2006-01-02T15:04:05Z07:00")
		}
		response.Charges = append(response.Charges, chargeResp)
	}

	return response
}
