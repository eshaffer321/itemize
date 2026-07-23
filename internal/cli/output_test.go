package cli

import (
	"io"
	"os"
	"testing"

	"github.com/eshaffer321/itemize/internal/application/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrintSyncSummaryReportsAmazonRefundOutcome(t *testing.T) {
	previous := os.Stdout
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = writer
	t.Cleanup(func() { os.Stdout = previous })

	PrintSyncSummary(&sync.Result{RefundProcessedCount: 3, RefundSkippedCount: 1}, nil, false)
	require.NoError(t, writer.Close())
	os.Stdout = previous
	output, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())

	assert.Contains(t, string(output), "Amazon refunds: Categorized=3 Left untouched=1")
	assert.Contains(t, string(output), "Sync completed successfully.")
}

func TestPrintSyncSummaryDoesNotReportSuccessWhenOrdersFailed(t *testing.T) {
	previous := os.Stdout
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = writer
	t.Cleanup(func() { os.Stdout = previous })

	PrintSyncSummary(&sync.Result{ProcessedCount: 1, ErrorCount: 2}, nil, false)
	require.NoError(t, writer.Close())
	os.Stdout = previous
	output, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())

	assert.Contains(t, string(output), "Sync completed with 2 errors.")
	assert.NotContains(t, string(output), "Sync completed successfully.")
}
