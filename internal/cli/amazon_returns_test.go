package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"

	amazonprovider "github.com/eshaffer321/itemize/internal/adapters/providers/amazon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingReturnWriter struct{}

func (failingReturnWriter) Write([]byte) (int, error) {
	return 0, errors.New("disk full")
}

func TestPrintAmazonReturnsWritesStableJSON(t *testing.T) {
	records := []amazonprovider.ReturnRecord{{
		OrderID:        "112-1111111-2222222",
		RMAID:          "D8ExampleRRMA",
		CreatedAt:      time.Date(2026, time.June, 29, 0, 0, 0, 0, time.UTC),
		RefundAmount:   11.36,
		HasRefundTotal: true,
		Status:         "Return in transit",
		Items: []amazonprovider.ReturnedItem{{
			ASIN:  "B0EXAMPLE1",
			Name:  "Insulated Sporty Cup",
			Price: 10.72,
		}},
	}}

	var output bytes.Buffer
	require.NoError(t, PrintAmazonReturns(&output, records))

	var decoded []map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &decoded))
	require.Len(t, decoded, 1)
	assert.Equal(t, "112-1111111-2222222", decoded[0]["order_id"])
	assert.Equal(t, 11.36, decoded[0]["refund_amount"])
	assert.NotContains(t, decoded[0], "status_url")
}

func TestPrintAmazonReturnsReportsWriterFailure(t *testing.T) {
	err := PrintAmazonReturns(failingReturnWriter{}, []amazonprovider.ReturnRecord{{RMAID: "DwriteRRMA"}})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write Amazon return ledger")
}
