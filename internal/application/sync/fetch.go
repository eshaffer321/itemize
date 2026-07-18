package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/eshaffer321/itemize/internal/adapters/providers"
	"github.com/eshaffer321/itemize/internal/domain/categorizer"
	"github.com/eshaffer321/itemize/internal/infrastructure/storage"
	"github.com/eshaffer321/monarch-go/v2/pkg/monarch"
)

// Data fetching functions for the sync orchestrator.
// These handle retrieving orders, transactions, and categories from external sources.

// fetchOrders fetches orders from the provider based on the given options
func (o *Orchestrator) fetchOrders(ctx context.Context, opts Options) ([]providers.Order, error) {
	if opts.OrderID != "" {
		started := time.Now()
		order, err := o.provider.GetOrderDetails(ctx, opts.OrderID)
		orders := make([]providers.Order, 0, 1)
		if order != nil {
			orders = append(orders, order)
		}
		var response any
		if o.storage != nil {
			response = summarizeOrdersForFetchLog(orders)
		}
		o.logProviderFetch("order_details", map[string]any{
			"order_id": opts.OrderID,
		}, response, err, time.Since(started), len(orders), 0)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch order %s: %w", opts.OrderID, err)
		}
		return orders, nil
	}

	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -opts.LookbackDays)

	o.logger.Debug("Fetching orders",
		"start_date", startDate.Format("2006-01-02"),
		"end_date", endDate.Format("2006-01-02"),
	)

	fetchOpts := providers.FetchOptions{
		StartDate:      startDate,
		EndDate:        endDate,
		MaxOrders:      opts.MaxOrders,
		IncludeDetails: true,
	}
	started := time.Now()
	orders, err := o.provider.FetchOrders(ctx, fetchOpts)
	var response any
	if o.storage != nil {
		response = summarizeOrdersForFetchLog(orders)
	}
	o.logProviderFetch("orders", map[string]any{
		"start_date":      startDate.Format("2006-01-02"),
		"end_date":        endDate.Format("2006-01-02"),
		"max_orders":      opts.MaxOrders,
		"include_details": true,
	}, response, err, time.Since(started), len(orders), 0)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch orders: %w", err)
	}

	o.logger.Debug("Fetched orders", "count", len(orders))

	return orders, nil
}

// fetchMonarchTransactions fetches and filters transactions from Monarch
func (o *Orchestrator) fetchMonarchTransactions(ctx context.Context, opts Options) ([]*monarch.Transaction, error) {
	o.logger.Debug("Fetching Monarch transactions")

	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -opts.LookbackDays)

	request := map[string]any{
		"start_date": startDate.AddDate(0, 0, -7).Format("2006-01-02"),
		"end_date":   endDate.Format("2006-01-02"),
		"limit":      500,
	}
	started := time.Now()
	txList, err := o.clients.Monarch.Transactions.Query().
		Between(startDate.AddDate(0, 0, -7), endDate). // Add buffer for date matching
		Limit(500).
		Execute(ctx)
	if err != nil {
		o.logProviderFetch("monarch_transactions", request, nil, err, time.Since(started), 0, 0)
		return nil, fmt.Errorf("failed to fetch transactions: %w", err)
	}

	// Filter for provider transactions (excluding splits)
	var providerTransactions []*monarch.Transaction
	providerName := strings.ToLower(o.provider.DisplayName())
	for _, tx := range txList.Transactions {
		// Skip split transactions - only process parent transactions
		if tx.IsSplitTransaction {
			continue
		}
		if tx.Merchant != nil && strings.Contains(strings.ToLower(tx.Merchant.Name), providerName) {
			providerTransactions = append(providerTransactions, tx)
		}
	}

	o.logger.Debug("Fetched transactions",
		"total", len(txList.Transactions),
		"provider_transactions", len(providerTransactions),
	)
	o.logProviderFetch("monarch_transactions", request, map[string]any{
		"total_count":           len(txList.Transactions),
		"provider_count":        len(providerTransactions),
		"provider_transactions": summarizeTransactionsForFetchLog(providerTransactions),
	}, nil, time.Since(started), 0, len(providerTransactions))

	return providerTransactions, nil
}

// fetchCategories fetches categories from Monarch and converts to categorizer format
func (o *Orchestrator) fetchCategories(ctx context.Context) ([]categorizer.Category, []*monarch.TransactionCategory, error) {
	o.logger.Debug("Loading Monarch categories")

	started := time.Now()
	categories, err := o.clients.Monarch.Transactions.Categories().List(ctx)
	o.logProviderFetch("monarch_categories", map[string]any{}, map[string]any{
		"category_count": len(categories),
	}, err, time.Since(started), 0, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load categories: %w", err)
	}

	o.logger.Debug("Loaded categories", "count", len(categories))

	// Convert to categorizer format
	catCategories := make([]categorizer.Category, len(categories))
	for i, cat := range categories {
		catCategories[i] = categorizer.Category{
			ID:   cat.ID,
			Name: cat.Name,
		}
	}

	return catCategories, categories, nil
}

type orderFetchSummary struct {
	ID           string  `json:"id"`
	Date         string  `json:"date"`
	Total        float64 `json:"total"`
	Subtotal     float64 `json:"subtotal"`
	Tax          float64 `json:"tax"`
	Tip          float64 `json:"tip"`
	FeeTotal     float64 `json:"fee_total"`
	ItemCount    int     `json:"item_count"`
	RawOrderJSON string  `json:"raw_order_json,omitempty"`
}

type transactionFetchSummary struct {
	ID                 string  `json:"id"`
	Date               string  `json:"date"`
	Amount             float64 `json:"amount"`
	MerchantName       string  `json:"merchant_name,omitempty"`
	PlaidName          string  `json:"plaid_name,omitempty"`
	Pending            bool    `json:"pending"`
	HasSplits          bool    `json:"has_splits"`
	IsSplitTransaction bool    `json:"is_split_transaction"`
}

func summarizeOrdersForFetchLog(orders []providers.Order) []orderFetchSummary {
	summaries := make([]orderFetchSummary, 0, len(orders))
	for _, order := range orders {
		summary := orderFetchSummary{
			ID:        order.GetID(),
			Date:      order.GetDate().Format("2006-01-02"),
			Total:     order.GetTotal(),
			Subtotal:  order.GetSubtotal(),
			Tax:       order.GetTax(),
			Tip:       order.GetTip(),
			FeeTotal:  order.GetFees(),
			ItemCount: len(order.GetItems()),
		}
		if raw := order.GetRawData(); raw != nil {
			if data, err := json.Marshal(raw); err == nil {
				summary.RawOrderJSON = string(data)
			}
		}
		summaries = append(summaries, summary)
	}
	return summaries
}

func summarizeTransactionsForFetchLog(txns []*monarch.Transaction) []transactionFetchSummary {
	summaries := make([]transactionFetchSummary, 0, len(txns))
	for _, tx := range txns {
		summary := transactionFetchSummary{
			ID:                 tx.ID,
			Date:               tx.Date.Format("2006-01-02"),
			Amount:             tx.Amount,
			PlaidName:          tx.PlaidName,
			Pending:            tx.Pending,
			HasSplits:          tx.HasSplits,
			IsSplitTransaction: tx.IsSplitTransaction,
		}
		if tx.Merchant != nil {
			summary.MerchantName = tx.Merchant.Name
		}
		summaries = append(summaries, summary)
	}
	return summaries
}

func (o *Orchestrator) logProviderFetch(fetchType string, request, response any, fetchErr error, duration time.Duration, orderCount, transactionCount int) {
	if o.storage == nil {
		return
	}
	requestJSON, _ := json.Marshal(request)
	responseJSON, _ := json.Marshal(response)
	errText := ""
	if fetchErr != nil {
		errText = fetchErr.Error()
	}
	if err := o.storage.LogProviderFetch(&storage.ProviderFetchLog{
		RunID:            o.runID,
		Provider:         o.provider.DisplayName(),
		FetchType:        fetchType,
		RequestJSON:      string(requestJSON),
		ResponseJSON:     string(responseJSON),
		Error:            errText,
		DurationMs:       duration.Milliseconds(),
		OrderCount:       orderCount,
		TransactionCount: transactionCount,
	}); err != nil {
		o.logger.Warn("Failed to log provider fetch", "fetch_type", fetchType, "error", err)
	}
}
