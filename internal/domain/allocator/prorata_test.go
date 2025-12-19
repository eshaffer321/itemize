package allocator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllocate_BasicProRata(t *testing.T) {
	// 3 items totaling $100, order total $95 (5% discount)
	items := []Item{
		{Name: "Widget A", ListPrice: 50.00},
		{Name: "Widget B", ListPrice: 30.00},
		{Name: "Widget C", ListPrice: 20.00},
	}

	result, err := Allocate(items, 95.00)
	require.NoError(t, err)

	assert.InDelta(t, 0.95, result.Multiplier, 0.001)
	assert.InDelta(t, 95.00, result.TotalAllocated, 0.01)

	// Each item should be 95% of its list price
	assert.InDelta(t, 47.50, result.Allocations[0].AllocatedCost, 0.01)
	assert.InDelta(t, 28.50, result.Allocations[1].AllocatedCost, 0.01)
	assert.InDelta(t, 19.00, result.Allocations[2].AllocatedCost, 0.01)
}

func TestAllocate_WithTaxIncrease(t *testing.T) {
	// Items $100 total, order total $106 (6% tax, no discounts)
	items := []Item{
		{Name: "Item A", ListPrice: 60.00},
		{Name: "Item B", ListPrice: 40.00},
	}

	result, err := Allocate(items, 106.00)
	require.NoError(t, err)

	assert.InDelta(t, 1.06, result.Multiplier, 0.001)
	assert.InDelta(t, 106.00, result.TotalAllocated, 0.01)

	assert.InDelta(t, 63.60, result.Allocations[0].AllocatedCost, 0.01)
	assert.InDelta(t, 42.40, result.Allocations[1].AllocatedCost, 0.01)
}

func TestAllocate_RealAmazonOrder(t *testing.T) {
	// Real order 112-4559127-2161020
	// Items sum to $107.26, bank charges sum to $103.27
	items := []Item{
		{Name: "Hot Wheels", ListPrice: 19.99},
		{Name: "Item 2", ListPrice: 12.72},
		{Name: "Item 3", ListPrice: 16.99},
		{Name: "Item 4", ListPrice: 7.99},
		{Name: "Peppa Pig", ListPrice: 6.49},
		{Name: "Paw Patrol Stickers", ListPrice: 26.99},
		{Name: "Paw Patrol Racers", ListPrice: 16.09},
	}

	orderTotal := 103.27 // What was actually charged to bank

	result, err := Allocate(items, orderTotal)
	require.NoError(t, err)

	// Multiplier should be 103.27 / 107.26 ≈ 0.9628
	assert.InDelta(t, 0.9628, result.Multiplier, 0.001)

	// Total allocated should equal order total
	assert.InDelta(t, orderTotal, result.TotalAllocated, 0.01)

	// Verify all allocations sum to order total
	var sum float64
	for _, a := range result.Allocations {
		sum += a.AllocatedCost
		// Each allocation should be less than list price (since multiplier < 1)
		assert.LessOrEqual(t, a.AllocatedCost, a.ListPrice)
	}
	assert.InDelta(t, orderTotal, sum, 0.01)
}

func TestAllocate_ExactMatch(t *testing.T) {
	// Order total equals item sum (no adjustments)
	items := []Item{
		{Name: "Item A", ListPrice: 25.00},
		{Name: "Item B", ListPrice: 25.00},
	}

	result, err := Allocate(items, 50.00)
	require.NoError(t, err)

	assert.InDelta(t, 1.0, result.Multiplier, 0.001)
	assert.Equal(t, 25.00, result.Allocations[0].AllocatedCost)
	assert.Equal(t, 25.00, result.Allocations[1].AllocatedCost)
}

func TestAllocate_SingleItem(t *testing.T) {
	items := []Item{
		{Name: "Only Item", ListPrice: 42.99},
	}

	result, err := Allocate(items, 45.50)
	require.NoError(t, err)

	assert.Len(t, result.Allocations, 1)
	assert.InDelta(t, 45.50, result.Allocations[0].AllocatedCost, 0.01)
}

func TestAllocate_ZeroPriceItem(t *testing.T) {
	// Free item mixed with paid items
	items := []Item{
		{Name: "Paid Item", ListPrice: 100.00},
		{Name: "Free Gift", ListPrice: 0.00},
	}

	result, err := Allocate(items, 95.00)
	require.NoError(t, err)

	assert.InDelta(t, 95.00, result.Allocations[0].AllocatedCost, 0.01)
	assert.Equal(t, 0.00, result.Allocations[1].AllocatedCost)
}

func TestAllocate_AllFreeItems(t *testing.T) {
	items := []Item{
		{Name: "Free A", ListPrice: 0.00},
		{Name: "Free B", ListPrice: 0.00},
	}

	result, err := Allocate(items, 0.00)
	require.NoError(t, err)

	assert.Equal(t, 0.0, result.Multiplier)
	assert.Equal(t, 0.00, result.TotalAllocated)
}

func TestAllocate_RoundingAdjustment(t *testing.T) {
	// Create a scenario where rounding would cause mismatch
	items := []Item{
		{Name: "Item A", ListPrice: 33.33},
		{Name: "Item B", ListPrice: 33.33},
		{Name: "Item C", ListPrice: 33.34},
	}

	result, err := Allocate(items, 100.00)
	require.NoError(t, err)

	// Total should be exactly $100.00 after rounding adjustment
	assert.Equal(t, 100.00, result.TotalAllocated)
}

func TestAllocate_SmallAmounts(t *testing.T) {
	items := []Item{
		{Name: "Cheap Item", ListPrice: 0.99},
	}

	result, err := Allocate(items, 1.05)
	require.NoError(t, err)

	assert.InDelta(t, 1.05, result.Allocations[0].AllocatedCost, 0.01)
}

func TestAllocate_LargeOrder(t *testing.T) {
	items := []Item{
		{Name: "Expensive A", ListPrice: 999.99},
		{Name: "Expensive B", ListPrice: 1499.99},
		{Name: "Expensive C", ListPrice: 500.02},
	}

	orderTotal := 2850.00

	result, err := Allocate(items, orderTotal)
	require.NoError(t, err)

	assert.InDelta(t, orderTotal, result.TotalAllocated, 0.01)
}

func TestAllocate_ErrorCases(t *testing.T) {
	t.Run("empty items", func(t *testing.T) {
		_, err := Allocate([]Item{}, 100.00)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no items")
	})

	t.Run("negative order total", func(t *testing.T) {
		items := []Item{{Name: "Item", ListPrice: 10.00}}
		_, err := Allocate(items, -50.00)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "negative")
	})

	t.Run("negative item price", func(t *testing.T) {
		items := []Item{{Name: "Bad Item", ListPrice: -10.00}}
		_, err := Allocate(items, 50.00)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "negative")
	})
}

func TestAllocate_PreservesItemInfo(t *testing.T) {
	items := []Item{
		{Name: "First Item", ListPrice: 25.00},
		{Name: "Second Item", ListPrice: 75.00},
	}

	result, err := Allocate(items, 90.00)
	require.NoError(t, err)

	// Verify item info is preserved
	assert.Equal(t, "First Item", result.Allocations[0].Name)
	assert.Equal(t, 25.00, result.Allocations[0].ListPrice)
	assert.Equal(t, "Second Item", result.Allocations[1].Name)
	assert.Equal(t, 75.00, result.Allocations[1].ListPrice)
}

func TestRoundToCents(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{1.234, 1.23},
		{1.235, 1.24},
		{1.239, 1.24},
		{0.001, 0.00},
		{0.005, 0.01},
		{99.999, 100.00},
	}

	for _, tt := range tests {
		result := roundToCents(tt.input)
		assert.Equal(t, tt.expected, result, "roundToCents(%v)", tt.input)
	}
}

func TestAllocate_MultiplierValues(t *testing.T) {
	tests := []struct {
		name       string
		listTotal  float64
		orderTotal float64
		wantMult   float64
	}{
		{"5% discount", 100.00, 95.00, 0.95},
		{"10% discount", 100.00, 90.00, 0.90},
		{"6% tax", 100.00, 106.00, 1.06},
		{"8.25% tax", 100.00, 108.25, 1.0825},
		{"net zero", 100.00, 100.00, 1.00},
		{"heavy discount", 100.00, 50.00, 0.50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items := []Item{{Name: "Item", ListPrice: tt.listTotal}}
			result, err := Allocate(items, tt.orderTotal)
			require.NoError(t, err)
			assert.InDelta(t, tt.wantMult, result.Multiplier, 0.0001)
		})
	}
}

// Benchmark to ensure allocation is fast
func BenchmarkAllocate(b *testing.B) {
	items := make([]Item, 20)
	for i := range items {
		items[i] = Item{Name: "Item", ListPrice: float64(10 + i)}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Allocate(items, 250.00)
	}
}
