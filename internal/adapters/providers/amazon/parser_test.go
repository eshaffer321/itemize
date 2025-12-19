package amazon

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAmount(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected float64
		wantErr  bool
	}{
		{
			name:     "simple amount",
			input:    "$116.20",
			expected: 116.20,
		},
		{
			name:     "amount with comma",
			input:    "$1,234.56",
			expected: 1234.56,
		},
		{
			name:     "negative amount",
			input:    "-$50.00",
			expected: -50.00,
		},
		{
			name:     "zero",
			input:    "$0.00",
			expected: 0.00,
		},
		{
			name:     "empty string",
			input:    "",
			expected: 0.00,
		},
		{
			name:     "with whitespace",
			input:    "  $99.99  ",
			expected: 99.99,
		},
		{
			name:    "invalid",
			input:   "not a number",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseAmount(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestParseDate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Time
		wantErr  bool
	}{
		{
			name:     "ISO 8601",
			input:    "2025-12-13",
			expected: time.Date(2025, 12, 13, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "US format",
			input:    "December 13, 2025",
			expected: time.Date(2025, 12, 13, 0, 0, 0, 0, time.UTC),
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid format",
			input:   "13/12/2025",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseDate(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestParseCLIOutput(t *testing.T) {
	jsonData := `{
		"orders": [
			{
				"orderId": "114-9989668-3824210",
				"orderDate": "2025-12-13",
				"total": "$116.20",
				"subtotal": "$109.62",
				"tax": "$6.58",
				"shipping": "$0.00",
				"items": [
					{ "name": "SUPFINE Magnetic iPhone Case", "price": "$14.99", "quantity": 1 },
					{ "name": "Phone Tripod", "price": "$25.64", "quantity": 1 }
				],
				"transactions": [
					{
						"date": "2025-12-13",
						"amount": "$116.20",
						"type": "charge",
						"last4": "1211",
						"description": "Prime Visa ****1211"
					}
				]
			}
		]
	}`

	output, err := ParseCLIOutput(strings.NewReader(jsonData))
	require.NoError(t, err)
	require.Len(t, output.Orders, 1)

	order := output.Orders[0]
	assert.Equal(t, "114-9989668-3824210", order.OrderID)
	assert.Equal(t, "2025-12-13", order.OrderDate)
	assert.Equal(t, "$116.20", order.Total)
	assert.Len(t, order.Items, 2)
	assert.Len(t, order.Transactions, 1)
	assert.Equal(t, "1211", order.Transactions[0].Last4)
}

func TestConvertCLIOrder(t *testing.T) {
	cliOrder := CLIOrder{
		OrderID:   "114-9989668-3824210",
		OrderDate: "2025-12-13",
		Total:     "$116.20",
		Subtotal:  "$109.62",
		Tax:       "$6.58",
		Shipping:  "$0.00",
		Items: []CLIOrderItem{
			{Name: "SUPFINE Magnetic iPhone Case", Price: "$14.99", Quantity: 1},
			{Name: "Phone Tripod", Price: "$25.64", Quantity: 2},
		},
		Transactions: []CLITransaction{
			{
				Date:        "2025-12-13",
				Amount:      "$116.20",
				Type:        "charge",
				Last4:       "1211",
				Description: "Prime Visa ****1211",
			},
		},
	}

	order, err := ConvertCLIOrder(cliOrder)
	require.NoError(t, err)

	assert.Equal(t, "114-9989668-3824210", order.ID)
	assert.Equal(t, time.Date(2025, 12, 13, 0, 0, 0, 0, time.UTC), order.Date)
	assert.Equal(t, 116.20, order.Total)
	assert.Equal(t, 109.62, order.Subtotal)
	assert.Equal(t, 6.58, order.Tax)
	assert.Equal(t, 0.0, order.Shipping)

	assert.Len(t, order.Items, 2)
	assert.Equal(t, "SUPFINE Magnetic iPhone Case", order.Items[0].Name)
	assert.Equal(t, 14.99, order.Items[0].Price)
	assert.Equal(t, 1, order.Items[0].Quantity)
	assert.Equal(t, "Phone Tripod", order.Items[1].Name)
	assert.Equal(t, 25.64, order.Items[1].Price)
	assert.Equal(t, 2, order.Items[1].Quantity)

	assert.Len(t, order.Transactions, 1)
	assert.Equal(t, 116.20, order.Transactions[0].Amount)
	assert.Equal(t, "charge", order.Transactions[0].Type)
	assert.Equal(t, "1211", order.Transactions[0].Last4)
}

func TestConvertCLIOrder_MissingOptionalFields(t *testing.T) {
	// Test with minimal data - should handle missing optional fields gracefully
	cliOrder := CLIOrder{
		OrderID:   "114-0000000-0000000",
		OrderDate: "2025-12-13",
		Total:     "$50.00",
		// Subtotal, Tax, Shipping are empty
		Items: []CLIOrderItem{
			{Name: "Test Item", Price: "$50.00", Quantity: 0}, // Quantity 0 should default to 1
		},
	}

	order, err := ConvertCLIOrder(cliOrder)
	require.NoError(t, err)

	assert.Equal(t, 50.00, order.Total)
	assert.Equal(t, 0.0, order.Subtotal) // Empty becomes 0
	assert.Equal(t, 0.0, order.Tax)
	assert.Equal(t, 0.0, order.Shipping)
	assert.Equal(t, 1, order.Items[0].Quantity) // 0 defaults to 1
}

func TestConvertCLIOrder_InvalidDate(t *testing.T) {
	cliOrder := CLIOrder{
		OrderID:   "114-0000000-0000000",
		OrderDate: "invalid-date",
		Total:     "$50.00",
	}

	_, err := ConvertCLIOrder(cliOrder)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse order date")
}

func TestConvertCLIOrder_InvalidTotal(t *testing.T) {
	cliOrder := CLIOrder{
		OrderID:   "114-0000000-0000000",
		OrderDate: "2025-12-13",
		Total:     "not-a-number",
	}

	_, err := ConvertCLIOrder(cliOrder)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse total")
}

func TestParseCLIOutputBytes(t *testing.T) {
	jsonData := []byte(`{"orders": [{"orderId": "test-123", "orderDate": "2025-01-01", "total": "$10.00"}]}`)

	output, err := ParseCLIOutputBytes(jsonData)
	require.NoError(t, err)
	assert.Len(t, output.Orders, 1)
	assert.Equal(t, "test-123", output.Orders[0].OrderID)
}

func TestParseCLIOutputBytes_InvalidJSON(t *testing.T) {
	jsonData := []byte(`not valid json`)

	_, err := ParseCLIOutputBytes(jsonData)
	assert.Error(t, err)
}

func TestConvertCLIOrder_InvalidSubtotal(t *testing.T) {
	cliOrder := CLIOrder{
		OrderID:   "114-0000000-0000000",
		OrderDate: "2025-12-13",
		Total:     "$50.00",
		Subtotal:  "invalid", // Non-empty invalid value should error
	}

	_, err := ConvertCLIOrder(cliOrder)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse subtotal")
}

func TestConvertCLIOrder_InvalidItemPrice(t *testing.T) {
	cliOrder := CLIOrder{
		OrderID:   "114-0000000-0000000",
		OrderDate: "2025-12-13",
		Total:     "$50.00",
		Items: []CLIOrderItem{
			{Name: "Bad Item", Price: "invalid", Quantity: 1},
		},
	}

	_, err := ConvertCLIOrder(cliOrder)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse item")
	assert.Contains(t, err.Error(), "Bad Item")
}

func TestConvertCLIOrder_InvalidTransactionAmount(t *testing.T) {
	cliOrder := CLIOrder{
		OrderID:   "114-0000000-0000000",
		OrderDate: "2025-12-13",
		Total:     "$50.00",
		Transactions: []CLITransaction{
			{Date: "2025-12-13", Amount: "invalid", Type: "charge"},
		},
	}

	_, err := ConvertCLIOrder(cliOrder)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse transaction")
}
