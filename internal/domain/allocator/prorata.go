// Package allocator provides cost allocation logic for order items.
//
// The pro-rata allocator distributes the actual order total across items
// proportionally to their list prices. This handles discounts, coupons,
// points, and tax in one simple ratio:
//
//	multiplier = order_total / sum(item_list_prices)
//	item_cost = item_list_price * multiplier
package allocator

import (
	"errors"
	"math"
)

// Item represents an item to allocate costs to.
type Item struct {
	Name      string
	ListPrice float64
}

// Allocation represents the allocated cost for a single item.
type Allocation struct {
	Name          string
	ListPrice     float64
	AllocatedCost float64
}

// Result contains the allocation results.
type Result struct {
	Multiplier     float64
	Allocations    []Allocation
	TotalAllocated float64
}

// Allocate distributes orderTotal across items proportionally to their list prices.
// Returns an error if items is empty or orderTotal is negative.
func Allocate(items []Item, orderTotal float64) (*Result, error) {
	if len(items) == 0 {
		return nil, errors.New("no items to allocate")
	}
	if orderTotal < 0 {
		return nil, errors.New("order total cannot be negative")
	}

	// Step 1: Sum list prices
	var totalListPrice float64
	for _, item := range items {
		if item.ListPrice < 0 {
			return nil, errors.New("item list price cannot be negative")
		}
		totalListPrice += item.ListPrice
	}

	if totalListPrice == 0 {
		// All items are free - distribute nothing
		allocations := make([]Allocation, len(items))
		for i, item := range items {
			allocations[i] = Allocation{
				Name:          item.Name,
				ListPrice:     0,
				AllocatedCost: 0,
			}
		}
		return &Result{
			Multiplier:     0,
			Allocations:    allocations,
			TotalAllocated: 0,
		}, nil
	}

	// Step 2: Calculate multiplier
	multiplier := orderTotal / totalListPrice

	// Step 3: Allocate to each item
	allocations := make([]Allocation, len(items))
	var totalAllocated float64

	for i, item := range items {
		allocated := roundToCents(item.ListPrice * multiplier)
		allocations[i] = Allocation{
			Name:          item.Name,
			ListPrice:     item.ListPrice,
			AllocatedCost: allocated,
		}
		totalAllocated += allocated
	}

	// Step 4: Fix rounding - adjust largest item if total is off
	diff := roundToCents(orderTotal - totalAllocated)
	if diff != 0 && math.Abs(diff) < 0.10 {
		// Find largest item and adjust
		maxIdx := 0
		for i, a := range allocations {
			if a.AllocatedCost > allocations[maxIdx].AllocatedCost {
				maxIdx = i
			}
		}
		allocations[maxIdx].AllocatedCost = roundToCents(allocations[maxIdx].AllocatedCost + diff)
		totalAllocated = roundToCents(totalAllocated + diff)
	}

	return &Result{
		Multiplier:     multiplier,
		Allocations:    allocations,
		TotalAllocated: totalAllocated,
	}, nil
}

// roundToCents rounds a float to 2 decimal places.
func roundToCents(amount float64) float64 {
	return math.Round(amount*100) / 100
}
