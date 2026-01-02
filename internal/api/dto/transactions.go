package dto

// TransactionResponse represents a Monarch Money transaction in API responses.
type TransactionResponse struct {
	ID              string                      `json:"id"`
	Date            string                      `json:"date"`
	Amount          float64                     `json:"amount"`
	Pending         bool                        `json:"pending"`
	HideFromReports bool                        `json:"hide_from_reports"`
	PlaidName       string                      `json:"plaid_name,omitempty"`
	Merchant        *MerchantResponse           `json:"merchant,omitempty"`
	Notes           string                      `json:"notes,omitempty"`
	HasSplits       bool                        `json:"has_splits"`
	IsRecurring     bool                        `json:"is_recurring"`
	NeedsReview     bool                        `json:"needs_review"`
	ReviewedAt      string                      `json:"reviewed_at,omitempty"`
	CreatedAt       string                      `json:"created_at"`
	UpdatedAt       string                      `json:"updated_at"`
	Account         *TransactionAccountResponse `json:"account,omitempty"`
	Category        *CategoryResponse           `json:"category,omitempty"`
	Tags            []TagResponse               `json:"tags,omitempty"`
}

// MerchantResponse represents a merchant in API responses.
type MerchantResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// TransactionAccountResponse represents account info for a transaction.
type TransactionAccountResponse struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Mask        string `json:"mask,omitempty"`
	LogoURL     string `json:"logo_url,omitempty"`
}

// CategoryResponse represents a transaction category.
type CategoryResponse struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	Icon             string                 `json:"icon,omitempty"`
	IsSystemCategory bool                   `json:"is_system_category"`
	Group            *CategoryGroupResponse `json:"group,omitempty"`
}

// CategoryGroupResponse represents a category group.
type CategoryGroupResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// TagResponse represents a transaction tag.
type TagResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}

// TransactionDetailResponse includes additional transaction details.
type TransactionDetailResponse struct {
	TransactionResponse
	OriginalMerchant string                       `json:"original_merchant,omitempty"`
	OriginalCategory *CategoryResponse            `json:"original_category,omitempty"`
	Splits           []TransactionSplitResponse   `json:"splits,omitempty"`
}

// TransactionSplitResponse represents a split within a transaction.
type TransactionSplitResponse struct {
	ID       string            `json:"id"`
	Amount   float64           `json:"amount"`
	Merchant *MerchantResponse `json:"merchant,omitempty"`
	Notes    string            `json:"notes,omitempty"`
	Category *CategoryResponse `json:"category,omitempty"`
}

// TransactionListResponse is returned when listing transactions.
type TransactionListResponse struct {
	Transactions []TransactionResponse `json:"transactions"`
	TotalCount   int                   `json:"total_count"`
	Limit        int                   `json:"limit"`
	Offset       int                   `json:"offset"`
	HasMore      bool                  `json:"has_more"`
}
