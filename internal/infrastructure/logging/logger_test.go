package logging

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLoggerCreatesPrivateLogFile(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "itemize.log")

	logger := NewLogger(config.LoggingConfig{
		Level:    "debug",
		FilePath: logPath,
	})
	t.Cleanup(CloseLogFile)

	logger.Debug("private file check")

	info, err := os.Stat(logPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestIsTerminalAndFDFitsInt(t *testing.T) {
	assert.False(t, isTerminal(bytes.NewBuffer(nil)))

	file, err := os.CreateTemp(t.TempDir(), "not-a-terminal")
	require.NoError(t, err)
	defer file.Close()

	assert.False(t, isTerminal(file))
	assert.True(t, fdFitsInt(0))
	assert.False(t, fdFitsInt(^uintptr(0)))
}

func TestMavenHandlerFormattingBranches(t *testing.T) {
	var buf bytes.Buffer
	handler := NewMavenHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	handler.useColors = true

	withAttrs := handler.WithAttrs([]slog.Attr{
		slog.String("system", "sync"),
		slog.String("request_id", "abc123"),
	})
	record := slog.NewRecord(time.Date(2026, 6, 20, 12, 34, 56, 0, time.UTC), slog.LevelWarn, "hello", 0)
	record.AddAttrs(slog.String("system", "ignored"), slog.Int("count", 2))

	require.NoError(t, withAttrs.Handle(context.Background(), record))
	output := buf.String()
	assert.Contains(t, output, "[WARN]")
	assert.Contains(t, output, "[sync]")
	assert.Contains(t, output, "request_id=abc123")
	assert.Contains(t, output, "count=2")
	assert.NotContains(t, output, "system=ignored")
}

func TestNewLoggerFallsBackWhenLogFileCannotOpen(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory write modes are not portable on Windows")
	}

	dir := t.TempDir()
	blockedPath := filepath.Join(dir, "blocked", "itemize.log")
	logger := NewLogger(config.LoggingConfig{FilePath: blockedPath})
	require.NotNil(t, logger)

	CloseLogFile()
	_, err := os.Stat(blockedPath)
	assert.True(t, os.IsNotExist(err))
}

func TestMavenHandlerWriteError(t *testing.T) {
	handler := NewMavenHandler(errorWriter{}, nil)
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "boom", 0)

	require.Error(t, handler.Handle(context.Background(), record))
}

type errorWriter struct{}

func (errorWriter) Write(_ []byte) (int, error) {
	return 0, io.ErrClosedPipe
}
