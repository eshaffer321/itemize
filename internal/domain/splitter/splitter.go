package splitter

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/domain/categorizer"
)

// Categorizer interface for dependency injection
type Categorizer interface {
	CategorizeItems(ctx context.Context, items []categorizer.Item, categories []categorizer.Category) (*categorizer.CategorizationResult, error)
}

// Splitter creates transaction splits from categorized orders
type Splitter struct {
	categorizer Categorizer
	lastResult  *categorizer.CategorizationResult // Cache last categorization
	lastOrderID string                            // Track which order was cached
}

// NewSplitter creates a new splitter
func NewSplitter(cat Categorizer) *Splitter {
	return &Splitter{
		categorizer: cat,
	}
}

// CreateSplits creates transaction splits for a multi-category order
//
// Returns:
//   - nil, nil: Single category detected - caller should update transaction category instead
//   - splits, nil: Multiple categories - splits ready to apply
//   - nil, error: Error occurred during categorization or split creation
//
// For single-category orders, the caller should use Monarch's Update API to set
// the category and notes rather than creating splits.
func (s *Splitter) CreateSplits(
	ctx context.Context,
	order providers.Order,
	transaction *monarch.Transaction,
	categories []categorizer.Category,
	monarchCategories []*monarch.TransactionCategory,
) ([]*monarch.TransactionSplit, error) {
	// Convert items for categorization
	items := make([]categorizer.Item, len(order.GetItems()))
	for i, orderItem := range order.GetItems() {
		items[i] = categorizer.Item{
			Name:     orderItem.GetName(),
			Price:    orderItem.GetPrice(),
			Quantity: int(orderItem.GetQuantity()),
		}
	}

	// Get categories from AI (or use cache if same order)
	var result *categorizer.CategorizationResult
	if s.lastOrderID == order.GetID() && s.lastResult != nil {
		// Use cached result
		result = s.lastResult
	} else {
		// Fetch new categorization
		var err error
		result, err = s.categorizer.CategorizeItems(ctx, items, categories)
		if err != nil {
			return nil, err
		}
		// Cache the result
		s.lastResult = result
		s.lastOrderID = order.GetID()
	}

	// Group items by category to detect single vs multi-category
	categoryGroups := make(map[string]bool)
	for _, cat := range result.Categorizations {
		categoryGroups[cat.CategoryID] = true
	}

	// If only one category, return nil (caller should update transaction instead)
	if len(categoryGroups) == 1 {
		return nil, nil
	}

	// Multiple categories - create splits
	return s.createMultiCategorySplits(order, transaction, result)
}

// createMultiCategorySplits creates splits for orders with multiple categories
func (s *Splitter) createMultiCategorySplits(
	order providers.Order,
	transaction *monarch.Transaction,
	categorizationResult *categorizer.CategorizationResult,
) ([]*monarch.TransactionSplit, error) {
	// Group items by category
	type categoryGroup struct {
		categoryID   string
		categoryName string
		items        []categorizer.Item
		subtotal     float64
	}

	groups := make(map[string]*categoryGroup)
	categoryIDs := make(map[string]string)
	categoryNames := make(map[string]string)

	// Map categorizations back to items
	orderItems := order.GetItems()
	for i, cat := range categorizationResult.Categorizations {
		if i >= len(orderItems) {
			break
		}

		item := categorizer.Item{
			Name:     orderItems[i].GetName(),
			Price:    orderItems[i].GetPrice(),
			Quantity: int(orderItems[i].GetQuantity()),
		}

		if groups[cat.CategoryID] == nil {
			groups[cat.CategoryID] = &categoryGroup{
				categoryID:   cat.CategoryID,
				categoryName: cat.CategoryName,
				items:        []categorizer.Item{},
				subtotal:     0.0,
			}
		}

		groups[cat.CategoryID].items = append(groups[cat.CategoryID].items, item)
		groups[cat.CategoryID].subtotal += item.Price
		categoryIDs[cat.CategoryName] = cat.CategoryID
		categoryNames[cat.CategoryID] = cat.CategoryName
	}

	// Calculate tax proportion
	subtotal := order.GetSubtotal()
	tax := order.GetTax()
	taxRate := 0.0
	if subtotal != 0 {
		taxRate = tax / subtotal
	}

	// Create splits for each category
	var splits []*monarch.TransactionSplit
	for _, group := range groups {
		// Add proportional tax
		categoryTax := group.subtotal * taxRate
		categoryTotal := group.subtotal + categoryTax

		// Match sign to transaction amount (negative for purchases, positive for returns)
		// The transaction.Amount already has the correct sign from Monarch
		// We just need to match our splits to that convention
		if transaction.Amount < 0 {
			// Purchase - splits should be negative
			categoryTotal = -math.Abs(categoryTotal)
		} else {
			// Return/refund - splits should be positive
			categoryTotal = math.Abs(categoryTotal)
		}

		// Build item details for notes
		itemDetails := []string{}
		for _, item := range group.items {
			if item.Quantity > 1 {
				itemDetails = append(itemDetails, fmt.Sprintf("%s (x%d)", item.Name, item.Quantity))
			} else {
				itemDetails = append(itemDetails, item.Name)
			}
		}

		// Create split with detailed notes
		noteContent := strings.Join(itemDetails, ", ")
		if len(group.items) > 3 {
			noteContent = fmt.Sprintf("(%d items) %s", len(group.items), noteContent)
		}

		split := &monarch.TransactionSplit{
			Amount:     categoryTotal,
			CategoryID: group.categoryID,
			Notes:      fmt.Sprintf("%s: %s", group.categoryName, noteContent),
		}

		splits = append(splits, split)
	}

	// Adjust for rounding to ensure splits sum to transaction amount
	totalSplits := 0.0
	for _, split := range splits {
		totalSplits += split.Amount
	}

	diff := transaction.Amount - totalSplits
	if math.Abs(diff) > 0.01 && len(splits) > 0 {
		// Add difference to largest split
		largestIdx := 0
		largestAmount := 0.0
		for i, split := range splits {
			if math.Abs(split.Amount) > largestAmount {
				largestAmount = math.Abs(split.Amount)
				largestIdx = i
			}
		}
		splits[largestIdx].Amount += diff
	}

	return splits, nil
}

// GetSingleCategoryInfo extracts category and notes for single-category orders
// This should only be called after CreateSplits returns nil (indicating single category)
// It uses the cached categorization result from CreateSplits to avoid duplicate AI calls
func (s *Splitter) GetSingleCategoryInfo(
	ctx context.Context,
	order providers.Order,
	categories []categorizer.Category,
) (categoryID string, notes string, err error) {
	// Use cached result if available (should be from CreateSplits call)
	var result *categorizer.CategorizationResult
	if s.lastOrderID == order.GetID() && s.lastResult != nil {
		result = s.lastResult
	} else {
		// Fallback: categorize if not cached (shouldn't happen in normal flow)
		items := make([]categorizer.Item, len(order.GetItems()))
		for i, orderItem := range order.GetItems() {
			items[i] = categorizer.Item{
				Name:     orderItem.GetName(),
				Price:    orderItem.GetPrice(),
				Quantity: int(orderItem.GetQuantity()),
			}
		}
		result, err = s.categorizer.CategorizeItems(ctx, items, categories)
		if err != nil {
			return "", "", err
		}
	}

	if len(result.Categorizations) == 0 {
		return "", "", fmt.Errorf("no categorizations returned")
	}

	// Get the single category ID (all items should have same category)
	categoryID = result.Categorizations[0].CategoryID
	categoryName := result.Categorizations[0].CategoryName

	// Build item details for notes
	orderItems := order.GetItems()
	itemDetails := []string{}
	for _, orderItem := range orderItems {
		if orderItem.GetQuantity() > 1 {
			itemDetails = append(itemDetails, fmt.Sprintf("%s (x%.0f)", orderItem.GetName(), orderItem.GetQuantity()))
		} else {
			itemDetails = append(itemDetails, orderItem.GetName())
		}
	}

	notes = fmt.Sprintf("%s: %s", categoryName, strings.Join(itemDetails, ", "))

	return categoryID, notes, nil
}
