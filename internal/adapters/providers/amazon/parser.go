package amazon

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// ParseCLIOutput parses the JSON output from amazon-order-scraper
func ParseCLIOutput(r io.Reader) (*CLIOutput, error) {
	var output CLIOutput
	if err := json.NewDecoder(r).Decode(&output); err != nil {
		return nil, fmt.Errorf("failed to decode CLI output: %w", err)
	}
	return &output, nil
}

// ParseCLIOutputBytes parses the JSON output from a byte slice
func ParseCLIOutputBytes(data []byte) (*CLIOutput, error) {
	var output CLIOutput
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, fmt.Errorf("failed to decode CLI output: %w", err)
	}
	return &output, nil
}

// ConvertCLIOrder converts a CLIOrder to a ParsedOrder
func ConvertCLIOrder(cliOrder CLIOrder) (*ParsedOrder, error) {
	order := &ParsedOrder{
		ID: cliOrder.OrderID,
	}

	// Parse date (ISO 8601: "2025-12-13")
	if cliOrder.OrderDate != "" {
		date, err := parseDate(cliOrder.OrderDate)
		if err != nil {
			return nil, fmt.Errorf("failed to parse order date %q: %w", cliOrder.OrderDate, err)
		}
		order.Date = date
	}

	// Parse amounts
	var err error
	order.Total, err = parseAmount(cliOrder.Total)
	if err != nil {
		return nil, fmt.Errorf("failed to parse total %q: %w", cliOrder.Total, err)
	}

	// Parse optional amounts - only fail on non-empty invalid values
	if cliOrder.Subtotal != "" {
		order.Subtotal, err = parseAmount(cliOrder.Subtotal)
		if err != nil {
			return nil, fmt.Errorf("failed to parse subtotal %q: %w", cliOrder.Subtotal, err)
		}
	}

	if cliOrder.Tax != "" {
		order.Tax, err = parseAmount(cliOrder.Tax)
		if err != nil {
			return nil, fmt.Errorf("failed to parse tax %q: %w", cliOrder.Tax, err)
		}
	}

	if cliOrder.Shipping != "" {
		order.Shipping, err = parseAmount(cliOrder.Shipping)
		if err != nil {
			return nil, fmt.Errorf("failed to parse shipping %q: %w", cliOrder.Shipping, err)
		}
	}

	// Parse items - fail on any item parse error to avoid silent data loss
	for i, cliItem := range cliOrder.Items {
		item, err := convertCLIItem(cliItem)
		if err != nil {
			return nil, fmt.Errorf("failed to parse item %d (%q): %w", i, cliItem.Name, err)
		}
		order.Items = append(order.Items, item)
	}

	// Parse transactions - fail on any transaction parse error
	for i, cliTx := range cliOrder.Transactions {
		tx, err := convertCLITransaction(cliTx)
		if err != nil {
			return nil, fmt.Errorf("failed to parse transaction %d: %w", i, err)
		}
		order.Transactions = append(order.Transactions, tx)
	}

	return order, nil
}

// convertCLIItem converts a CLIOrderItem to a ParsedOrderItem
func convertCLIItem(cliItem CLIOrderItem) (*ParsedOrderItem, error) {
	price, err := parseAmount(cliItem.Price)
	if err != nil {
		return nil, fmt.Errorf("failed to parse item price %q: %w", cliItem.Price, err)
	}

	quantity := cliItem.Quantity
	if quantity == 0 {
		quantity = 1 // Default to 1 if not specified
	}

	return &ParsedOrderItem{
		Name:     cliItem.Name,
		Price:    price,
		Quantity: quantity,
	}, nil
}

// convertCLITransaction converts a CLITransaction to a ParsedTransaction
func convertCLITransaction(cliTx CLITransaction) (*ParsedTransaction, error) {
	tx := &ParsedTransaction{
		Type:        cliTx.Type,
		Last4:       cliTx.Last4,
		Description: cliTx.Description,
	}

	// Parse date
	if cliTx.Date != "" {
		date, err := parseDate(cliTx.Date)
		if err != nil {
			return nil, fmt.Errorf("failed to parse transaction date %q: %w", cliTx.Date, err)
		}
		tx.Date = date
	}

	// Parse amount
	amount, err := parseAmount(cliTx.Amount)
	if err != nil {
		return nil, fmt.Errorf("failed to parse transaction amount %q: %w", cliTx.Amount, err)
	}
	tx.Amount = amount

	return tx, nil
}

// parseAmount parses a currency string like "$116.20" or "$1,234.56" to float64
func parseAmount(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}

	// Remove currency symbol, commas, and whitespace
	cleaned := strings.TrimSpace(s)
	cleaned = strings.TrimPrefix(cleaned, "$")
	cleaned = strings.TrimPrefix(cleaned, "-$") // Handle negative amounts
	cleaned = strings.ReplaceAll(cleaned, ",", "")

	// Check if original was negative
	isNegative := strings.HasPrefix(strings.TrimSpace(s), "-")

	amount, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount: %w", err)
	}

	if isNegative {
		amount = -amount
	}

	return amount, nil
}

// parseDate parses an ISO 8601 date string like "2025-12-13"
func parseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty date string")
	}

	// Try ISO 8601 format first (expected format)
	t, err := time.Parse("2006-01-02", s)
	if err == nil {
		return t, nil
	}

	// Fallback: try with time component
	t, err = time.Parse(time.RFC3339, s)
	if err == nil {
		return t, nil
	}

	// Fallback: try common US format
	t, err = time.Parse("January 2, 2006", s)
	if err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("unable to parse date %q", s)
}
