package cli

import (
	"encoding/json"
	"fmt"
	"io"

	amazonprovider "github.com/eshaffer321/itemize/internal/adapters/providers/amazon"
)

// PrintAmazonReturns writes the direct Amazon return ledger as stable JSON.
func PrintAmazonReturns(w io.Writer, returns []amazonprovider.ReturnRecord) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(returns); err != nil {
		return fmt.Errorf("failed to write Amazon return ledger: %w", err)
	}
	return nil
}
