package dto

// OrderListParams represents query parameters for listing orders.
type OrderListParams struct {
	Provider  string `json:"provider"`
	Status    string `json:"status"`
	DaysBack  int    `json:"days_back"`
	Limit     int    `json:"limit"`
	Offset    int    `json:"offset"`
	OrderBy   string `json:"order_by"`
	OrderDesc bool   `json:"order_desc"`
}

// ItemSearchParams represents query parameters for searching items.
type ItemSearchParams struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

// SyncRunListParams represents query parameters for listing sync runs.
type SyncRunListParams struct {
	Limit int `json:"limit"`
}

// DefaultOrderListParams returns default values for order list params.
func DefaultOrderListParams() OrderListParams {
	return OrderListParams{
		Limit:     50,
		Offset:    0,
		OrderBy:   "processed_at",
		OrderDesc: true,
	}
}

// DefaultItemSearchParams returns default values for item search params.
func DefaultItemSearchParams() ItemSearchParams {
	return ItemSearchParams{
		Limit: 50,
	}
}

// DefaultSyncRunListParams returns default values for sync run list params.
func DefaultSyncRunListParams() SyncRunListParams {
	return SyncRunListParams{
		Limit: 20,
	}
}
