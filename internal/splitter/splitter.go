package splitter

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	walmart "github.com/eshaffer321/walmart-client"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/categorizer"
)

// SplitStrategy defines how to split transactions
type SplitStrategy struct {
	GroupByCategory      bool    // Group items by category
	IncludeTax          bool    // Include proportional tax in splits
	MinSplitAmount      float64 // Minimum amount for a split (combine smaller ones)
	IncludeItemDetails  bool    // Include item names in notes
}

// DefaultStrategy returns the default split strategy
func DefaultStrategy() SplitStrategy {
	return SplitStrategy{
		GroupByCategory:     true,
		IncludeTax:         true,
		MinSplitAmount:     1.00, // Don't create splits under $1
		IncludeItemDetails: true,
	}
}

// SplitResult represents the result of splitting a transaction
type SplitResult struct {
	TransactionID string                      `json:"transaction_id"`
	OrderID       string                      `json:"order_id"`
	OriginalAmount float64                    `json:"original_amount"`
	Splits        []*monarch.TransactionSplit `json:"splits"`
	Summary       string                      `json:"summary"`
	HasTip        bool                        `json:"has_tip"`      // Whether this order includes a driver tip
	TipAmount     float64                     `json:"tip_amount"`   // Amount of the driver tip
}

// ItemWithCategory represents an item with its categorization
type ItemWithCategory struct {
	Item       walmart.OrderItem
	CategoryID string
	CategoryName string
	Price      float64
}

// Splitter handles transaction splitting
type Splitter struct {
	categorizer *categorizer.Categorizer
	categories  map[string]*monarch.TransactionCategory
	catList     []categorizer.Category // For passing to categorizer
	strategy    SplitStrategy
}

// NewSplitter creates a new splitter
func NewSplitter(cat *categorizer.Categorizer, categories []*monarch.TransactionCategory, strategy SplitStrategy) *Splitter {
	// Build category map for quick lookup
	catMap := make(map[string]*monarch.TransactionCategory)
	var catList []categorizer.Category
	
	for _, c := range categories {
		catMap[c.ID] = c
		if !c.IsDisabled {
			catList = append(catList, categorizer.Category{
				ID:   c.ID,
				Name: c.Name,
			})
		}
	}
	
	return &Splitter{
		categorizer: cat,
		categories:  catMap,
		catList:     catList,
		strategy:    strategy,
	}
}

// SplitTransaction splits a Walmart transaction into categorized splits
func (s *Splitter) SplitTransaction(ctx context.Context, order *walmart.Order, transaction *monarch.Transaction) (*SplitResult, error) {
	result := &SplitResult{
		TransactionID:  transaction.ID,
		OrderID:        order.ID,
		OriginalAmount: math.Abs(transaction.Amount),
	}

	// Extract all items from order
	items := s.extractItems(order)
	if len(items) == 0 {
		return nil, fmt.Errorf("no items found in order")
	}

	// Categorize all items
	categorizedItems, err := s.categorizeItems(ctx, items)
	if err != nil {
		return nil, fmt.Errorf("failed to categorize items: %w", err)
	}

	// Calculate totals for tax distribution
	subtotal := 0.0
	for _, item := range categorizedItems {
		subtotal += item.Price
	}

	// Calculate tax and tip (if order has price details)
	tax := 0.0
	tipAmount := 0.0
	if order.PriceDetails != nil {
		if order.PriceDetails.SubTotal != nil && order.PriceDetails.TaxTotal != nil {
			tax = order.PriceDetails.TaxTotal.Value
		}
		// Check for driver tip
		if order.PriceDetails.DriverTip != nil && order.PriceDetails.DriverTip.Value > 0 {
			tipAmount = order.PriceDetails.DriverTip.Value
			result.HasTip = true
			result.TipAmount = tipAmount
		}
	}

	// Group items by category if strategy says so
	var splits []*monarch.TransactionSplit
	if s.strategy.GroupByCategory {
		splits = s.createGroupedSplits(categorizedItems, subtotal, tax)
	} else {
		splits = s.createIndividualSplits(categorizedItems, subtotal, tax)
	}

	// Add driver tip as a separate split if present
	if tipAmount > 0 {
		// Let OpenAI decide the category, but we'll default to a service/fee category
		// First look for appropriate categories
		tipCategoryID := ""
		tipCategoryName := ""
		
		// Look for service/fee related categories
		preferredCategories := []string{"Shopping", "Fees & Charges", "Services", "Other"}
		for _, preferred := range preferredCategories {
			for _, cat := range s.categories {
				if cat.Name == preferred {
					tipCategoryID = cat.ID
					tipCategoryName = cat.Name
					break
				}
			}
			if tipCategoryID != "" {
				break
			}
		}
		
		// If no preferred category found, use Groceries as fallback
		if tipCategoryID == "" {
			for _, cat := range s.categories {
				if cat.Name == "Groceries" {
					tipCategoryID = cat.ID
					tipCategoryName = cat.Name
					break
				}
			}
		}
		
		// Create tip split as a separate line item
		tipSplit := &monarch.TransactionSplit{
			CategoryID: tipCategoryID,
			Amount:     -tipAmount, // Negative for expense
			Notes:      fmt.Sprintf("Delivery Driver Tip"),
			Merchant: &monarch.Merchant{
				Name: "Walmart Delivery Tip",
			},
		}
		splits = append(splits, tipSplit)
		
		fmt.Printf("     Added delivery tip split: $%.2f in %s\n", tipAmount, tipCategoryName)
	}

	// Combine small splits if needed
	if s.strategy.MinSplitAmount > 0 {
		splits = s.combineSmallSplits(splits)
	}

	// Ensure splits sum to original amount (handle rounding)
	// Pass the transaction amount with its sign (negative for expenses)
	s.adjustForRounding(splits, transaction.Amount)

	result.Splits = splits
	result.Summary = s.generateSummary(splits)

	return result, nil
}

// extractItems extracts all items from a Walmart order
func (s *Splitter) extractItems(order *walmart.Order) []walmart.OrderItem {
	var items []walmart.OrderItem
	
	for _, group := range order.Groups {
		items = append(items, group.Items...)
	}
	
	return items
}

// categorizeItems categorizes all items using the AI categorizer
func (s *Splitter) categorizeItems(ctx context.Context, items []walmart.OrderItem) ([]*ItemWithCategory, error) {
	var categorized []*ItemWithCategory
	
	// Build list of items for categorization
	var categorizerItems []categorizer.Item
	for _, item := range items {
		if item.ProductInfo != nil {
			price := 0.0
			if item.PriceInfo != nil && item.PriceInfo.LinePrice != nil {
				price = item.PriceInfo.LinePrice.Value
			}
			categorizerItems = append(categorizerItems, categorizer.Item{
				Name:  item.ProductInfo.Name,
				Price: price,
			})
		}
	}

	// Get category IDs from categorizer (use pre-built list)
	result, err := s.categorizer.CategorizeItems(ctx, categorizerItems, s.catList)
	if err != nil {
		return nil, err
	}

	// Match items with categories
	for i, item := range items {
		if item.ProductInfo == nil {
			continue
		}

		catID := ""
		if i < len(result.Categorizations) {
			catID = result.Categorizations[i].CategoryID
		}
		
		catName := "Uncategorized"
		if cat, ok := s.categories[catID]; ok {
			catName = cat.Name
		}

		price := 0.0
		if item.PriceInfo != nil && item.PriceInfo.LinePrice != nil {
			price = item.PriceInfo.LinePrice.Value
		}

		categorized = append(categorized, &ItemWithCategory{
			Item:         item,
			CategoryID:   catID,
			CategoryName: catName,
			Price:        price,
		})
	}

	return categorized, nil
}

// createGroupedSplits creates splits grouped by category
func (s *Splitter) createGroupedSplits(items []*ItemWithCategory, subtotal, tax float64) []*monarch.TransactionSplit {
	// Group items by category
	groups := make(map[string][]*ItemWithCategory)
	for _, item := range items {
		groups[item.CategoryID] = append(groups[item.CategoryID], item)
	}

	// Create a split for each category
	var splits []*monarch.TransactionSplit
	for catID, catItems := range groups {
		// Calculate category subtotal
		catSubtotal := 0.0
		var itemNames []string
		
		for _, item := range catItems {
			catSubtotal += item.Price
			if item.Item.ProductInfo != nil && s.strategy.IncludeItemDetails {
				itemName := item.Item.ProductInfo.Name
				if item.Item.Quantity > 1 {
					itemName = fmt.Sprintf("%s (x%.3g) - $%.2f", itemName, item.Item.Quantity, item.Price)
				} else {
					itemName = fmt.Sprintf("%s - $%.2f", itemName, item.Price)
				}
				itemNames = append(itemNames, itemName)
			}
		}

		// Calculate proportional tax
		catTax := 0.0
		if s.strategy.IncludeTax && tax > 0 && subtotal > 0 {
			catTax = (catSubtotal / subtotal) * tax
		}

		// Create split
		split := &monarch.TransactionSplit{
			CategoryID: catID,
			Amount:     -(catSubtotal + catTax), // Negative for expense
			Notes:      s.formatNotes(itemNames, len(catItems)),
		}

		// Set merchant name from first item's category
		if len(catItems) > 0 {
			split.Merchant = &monarch.Merchant{
				Name: fmt.Sprintf("Walmart - %s", catItems[0].CategoryName),
			}
		}

		splits = append(splits, split)
	}

	return splits
}

// createIndividualSplits creates a split for each item (not grouped)
func (s *Splitter) createIndividualSplits(items []*ItemWithCategory, subtotal, tax float64) []*monarch.TransactionSplit {
	var splits []*monarch.TransactionSplit
	
	for _, item := range items {
		// Calculate proportional tax for this item
		itemTax := 0.0
		if s.strategy.IncludeTax && tax > 0 && subtotal > 0 {
			itemTax = (item.Price / subtotal) * tax
		}

		notes := ""
		if item.Item.ProductInfo != nil {
			notes = item.Item.ProductInfo.Name
			if item.Item.Quantity > 1 {
				notes = fmt.Sprintf("%s (x%.0f)", notes, item.Item.Quantity)
			}
		}

		split := &monarch.TransactionSplit{
			CategoryID: item.CategoryID,
			Amount:     -(item.Price + itemTax), // Negative for expense
			Notes:      notes,
			Merchant: &monarch.Merchant{
				Name: fmt.Sprintf("Walmart - %s", item.CategoryName),
			},
		}

		splits = append(splits, split)
	}

	return splits
}

// combineSmallSplits combines splits under the minimum amount threshold
func (s *Splitter) combineSmallSplits(splits []*monarch.TransactionSplit) []*monarch.TransactionSplit {
	var result []*monarch.TransactionSplit
	var smallSplits []*monarch.TransactionSplit
	smallTotal := 0.0

	for _, split := range splits {
		if math.Abs(split.Amount) < s.strategy.MinSplitAmount {
			smallSplits = append(smallSplits, split)
			smallTotal += split.Amount
		} else {
			result = append(result, split)
		}
	}

	// If we have small splits, combine them
	if len(smallSplits) > 0 {
		// Find the most common category among small splits
		catCounts := make(map[string]int)
		for _, split := range smallSplits {
			catCounts[split.CategoryID]++
		}

		maxCount := 0
		bestCatID := ""
		for catID, count := range catCounts {
			if count > maxCount {
				maxCount = count
				bestCatID = catID
			}
		}

		// Create combined split
		combined := &monarch.TransactionSplit{
			CategoryID: bestCatID,
			Amount:     smallTotal,
			Notes:      fmt.Sprintf("Combined %d small items", len(smallSplits)),
			Merchant: &monarch.Merchant{
				Name: "Walmart - Various",
			},
		}

		result = append(result, combined)
	}

	return result
}

// adjustForRounding ensures splits sum to exactly the original amount
func (s *Splitter) adjustForRounding(splits []*monarch.TransactionSplit, targetTotal float64) {
	if len(splits) == 0 {
		return
	}

	// Calculate current total (sum of all splits with their signs)
	currentTotal := 0.0
	for _, split := range splits {
		currentTotal += split.Amount
	}

	// The target should be negative for expenses
	// diff is what we need to add to current total to reach target
	diff := targetTotal - currentTotal
	
	
	if math.Abs(diff) > 0.001 { // Adjust any difference larger than a tenth of a cent
		// Find largest split to absorb the difference
		largestIdx := 0
		largestAmount := 0.0
		
		for i, split := range splits {
			if math.Abs(split.Amount) > largestAmount {
				largestAmount = math.Abs(split.Amount)
				largestIdx = i
			}
		}

		// Add the difference to the largest split
		splits[largestIdx].Amount += diff
	}
}

// formatNotes formats the notes field for a split
func (s *Splitter) formatNotes(itemNames []string, itemCount int) string {
	if len(itemNames) == 0 {
		return fmt.Sprintf("%d items", itemCount)
	}

	if len(itemNames) == 1 {
		return itemNames[0]
	}

	if len(itemNames) <= 3 {
		return strings.Join(itemNames, ", ")
	}

	// For many items, show first 2 and count
	return fmt.Sprintf("%s, %s, and %d more items", 
		itemNames[0], itemNames[1], len(itemNames)-2)
}

// generateSummary generates a summary of the splits
func (s *Splitter) generateSummary(splits []*monarch.TransactionSplit) string {
	if len(splits) == 0 {
		return "No splits created"
	}

	if len(splits) == 1 {
		return "Transaction not split (single category)"
	}

	total := 0.0
	for _, split := range splits {
		total += math.Abs(split.Amount)
	}

	return fmt.Sprintf("Split into %d categories totaling $%.2f", len(splits), total)
}

// roundToCents rounds a float to 2 decimal places
func roundToCents(amount float64) float64 {
	return math.Round(amount*100) / 100
}