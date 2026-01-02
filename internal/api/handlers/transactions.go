package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/api/dto"
)

// TransactionsHandler handles transaction-related HTTP requests.
type TransactionsHandler struct {
	monarchClient *monarch.Client
}

// NewTransactionsHandler creates a new transactions handler.
func NewTransactionsHandler(monarchClient *monarch.Client) *TransactionsHandler {
	return &TransactionsHandler{
		monarchClient: monarchClient,
	}
}

// writeJSON writes a JSON response with the given status code.
func (h *TransactionsHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// writeError writes an error response with the given status code.
func (h *TransactionsHandler) writeError(w http.ResponseWriter, status int, err dto.APIError) {
	h.writeJSON(w, status, err)
}

// List handles GET /api/transactions - returns paginated list of transactions from Monarch.
func (h *TransactionsHandler) List(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	limit := ParseIntParam(r, "limit", 50)
	offset := ParseIntParam(r, "offset", 0)
	search := r.URL.Query().Get("search")
	daysBack := ParseIntParam(r, "days_back", 30) // Default to last 30 days
	pendingOnly := ParseBoolParam(r, "pending", false)

	// Build query
	query := h.monarchClient.Transactions.Query()

	// Set date range
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -daysBack)
	query = query.Between(startDate, endDate)

	// Apply search filter if provided
	if search != "" {
		query = query.Search(search)
	}

	// Apply pagination
	query = query.Limit(limit).Offset(offset)

	// Execute query
	result, err := query.Execute(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, dto.InternalError())
		return
	}

	// Filter pending if requested
	transactions := result.Transactions
	if pendingOnly {
		filtered := make([]*monarch.Transaction, 0)
		for _, txn := range transactions {
			if txn.Pending {
				filtered = append(filtered, txn)
			}
		}
		transactions = filtered
	}

	// Convert to response
	response := dto.TransactionListResponse{
		Transactions: make([]dto.TransactionResponse, 0, len(transactions)),
		TotalCount:   result.TotalCount,
		Limit:        limit,
		Offset:       offset,
		HasMore:      result.HasMore,
	}

	for _, txn := range transactions {
		response.Transactions = append(response.Transactions, toTransactionResponse(txn))
	}

	h.writeJSON(w, http.StatusOK, response)
}

// Get handles GET /api/transactions/{id} - returns a single transaction with details.
func (h *TransactionsHandler) Get(w http.ResponseWriter, r *http.Request) {
	transactionID := chi.URLParam(r, "id")
	if transactionID == "" {
		h.writeError(w, http.StatusBadRequest, dto.BadRequestError("transaction ID is required"))
		return
	}

	// Fetch transaction details
	details, err := h.monarchClient.Transactions.Get(r.Context(), transactionID)
	if err != nil {
		if err == monarch.ErrNotFound {
			h.writeError(w, http.StatusNotFound, dto.NotFoundError("transaction"))
			return
		}
		h.writeError(w, http.StatusInternalServerError, dto.InternalError())
		return
	}

	response := toTransactionDetailResponse(details)
	h.writeJSON(w, http.StatusOK, response)
}

// toTransactionResponse converts a Monarch transaction to an API response.
func toTransactionResponse(txn *monarch.Transaction) dto.TransactionResponse {
	response := dto.TransactionResponse{
		ID:              txn.ID,
		Date:            txn.Date.Format("2006-01-02"),
		Amount:          txn.Amount,
		Pending:         txn.Pending,
		HideFromReports: txn.HideFromReports,
		PlaidName:       txn.PlaidName,
		Notes:           txn.Notes,
		HasSplits:       txn.HasSplits,
		IsRecurring:     txn.IsRecurring,
		NeedsReview:     txn.NeedsReview,
		CreatedAt:       txn.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:       txn.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		Tags:            make([]dto.TagResponse, 0),
	}

	if txn.ReviewedAt != nil {
		response.ReviewedAt = txn.ReviewedAt.Format("2006-01-02")
	}

	if txn.Merchant != nil {
		response.Merchant = &dto.MerchantResponse{
			ID:   txn.Merchant.ID,
			Name: txn.Merchant.Name,
		}
	}

	if txn.Account != nil {
		response.Account = &dto.TransactionAccountResponse{
			ID:          txn.Account.ID,
			DisplayName: txn.Account.DisplayName,
			Mask:        txn.Account.Mask,
			LogoURL:     txn.Account.LogoURL,
		}
	}

	if txn.Category != nil {
		response.Category = toCategoryResponse(txn.Category)
	}

	for _, tag := range txn.Tags {
		if tag != nil {
			response.Tags = append(response.Tags, dto.TagResponse{
				ID:    tag.ID,
				Name:  tag.Name,
				Color: tag.Color,
			})
		}
	}

	return response
}

// toTransactionDetailResponse converts a Monarch transaction detail to an API response.
func toTransactionDetailResponse(details *monarch.TransactionDetails) dto.TransactionDetailResponse {
	base := toTransactionResponse(details.Transaction)

	response := dto.TransactionDetailResponse{
		TransactionResponse: base,
		OriginalMerchant:    details.OriginalMerchant,
		Splits:              make([]dto.TransactionSplitResponse, 0),
	}

	if details.OriginalCategory != nil {
		response.OriginalCategory = toCategoryResponse(details.OriginalCategory)
	}

	for _, split := range details.Splits {
		if split != nil {
			splitResp := dto.TransactionSplitResponse{
				ID:     split.ID,
				Amount: split.Amount,
				Notes:  split.Notes,
			}

			if split.Merchant != nil {
				splitResp.Merchant = &dto.MerchantResponse{
					ID:   split.Merchant.ID,
					Name: split.Merchant.Name,
				}
			}

			if split.Category != nil {
				splitResp.Category = toCategoryResponse(split.Category)
			}

			response.Splits = append(response.Splits, splitResp)
		}
	}

	return response
}

// toCategoryResponse converts a Monarch category to an API response.
func toCategoryResponse(cat *monarch.TransactionCategory) *dto.CategoryResponse {
	if cat == nil {
		return nil
	}

	response := &dto.CategoryResponse{
		ID:               cat.ID,
		Name:             cat.Name,
		Icon:             cat.Icon,
		IsSystemCategory: cat.IsSystemCategory,
	}

	if cat.Group != nil {
		response.Group = &dto.CategoryGroupResponse{
			ID:   cat.Group.ID,
			Name: cat.Group.Name,
			Type: cat.Group.Type,
		}
	}

	return response
}
