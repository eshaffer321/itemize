package sync

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/domain/categorizer"
)

// Data fetching functions for the sync orchestrator.
// These handle retrieving orders, transactions, and categories from external sources.

// fetchOrders fetches orders from the provider based on the given options
func (o *Orchestrator) fetchOrders(ctx context.Context, opts Options) ([]providers.Order, error) {
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -opts.LookbackDays)

	o.logger.Debug("Fetching orders",
		"start_date", startDate.Format("2006-01-02"),
		"end_date", endDate.Format("2006-01-02"),
	)

	orders, err := o.provider.FetchOrders(ctx, providers.FetchOptions{
		StartDate:      startDate,
		EndDate:        endDate,
		MaxOrders:      opts.MaxOrders,
		IncludeDetails: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch orders: %w", err)
	}

	o.logger.Debug("Fetched orders", "count", len(orders))

	return orders, nil
}

// fetchMonarchTransactions fetches and filters transactions from Monarch Money
func (o *Orchestrator) fetchMonarchTransactions(ctx context.Context, opts Options) ([]*monarch.Transaction, error) {
	o.logger.Debug("Fetching Monarch transactions")

	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -opts.LookbackDays)

	txList, err := o.clients.Monarch.Transactions.Query().
		Between(startDate.AddDate(0, 0, -7), endDate). // Add buffer for date matching
		Limit(500).
		Execute(ctx)
	if err != nil {
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

	return providerTransactions, nil
}

// fetchCategories fetches categories from Monarch and converts to categorizer format
func (o *Orchestrator) fetchCategories(ctx context.Context) ([]categorizer.Category, []*monarch.TransactionCategory, error) {
	o.logger.Debug("Loading Monarch categories")

	categories, err := o.clients.Monarch.Transactions.Categories().List(ctx)
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
