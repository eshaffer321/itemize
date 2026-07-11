package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExplainAmazonCookieExportError_WhenProfileOpensSignIn(t *testing.T) {
	err := explainAmazonCookieExportError(errors.New(`Error: profile did not open an authenticated Amazon orders page (title="Amazon Sign-In", login=true)`))

	assert.Contains(t, err.Error(), "browser profile is not logged into Amazon")
	assert.Contains(t, err.Error(), "rerun without -headless")
}

func TestExplainAmazonCookieExportError_WhenBrowserCrashes(t *testing.T) {
	err := explainAmazonCookieExportError(errors.New("browserType.launchPersistentContext: Target page, context or browser has been closed\nsignal=SIGTRAP"))

	assert.Contains(t, err.Error(), "chromium closed before itemize could read Amazon cookies")
	assert.Contains(t, err.Error(), "-headless")
}

func TestCleanStaleChromiumSingletons_RemovesMarkersForDeadPid(t *testing.T) {
	dir := t.TempDir()
	requireSymlink(t, "host-999999", filepath.Join(dir, "SingletonLock"))
	requireSymlink(t, "/tmp/chrome/SingletonSocket", filepath.Join(dir, "SingletonSocket"))
	requireSymlink(t, "cookie", filepath.Join(dir, "SingletonCookie"))

	orig := chromiumProcessExists
	chromiumProcessExists = func(pid int) bool {
		assert.Equal(t, 999999, pid)
		return false
	}
	t.Cleanup(func() { chromiumProcessExists = orig })

	require.NoError(t, cleanStaleChromiumSingletons(dir))

	assert.NoFileExists(t, filepath.Join(dir, "SingletonLock"))
	assert.NoFileExists(t, filepath.Join(dir, "SingletonSocket"))
	assert.NoFileExists(t, filepath.Join(dir, "SingletonCookie"))
}

func TestCleanStaleChromiumSingletons_KeepsMarkersForLivePid(t *testing.T) {
	dir := t.TempDir()
	requireSymlink(t, "host-12345", filepath.Join(dir, "SingletonLock"))
	requireSymlink(t, "/tmp/chrome/SingletonSocket", filepath.Join(dir, "SingletonSocket"))
	requireSymlink(t, "cookie", filepath.Join(dir, "SingletonCookie"))

	orig := chromiumProcessExists
	chromiumProcessExists = func(pid int) bool {
		assert.Equal(t, 12345, pid)
		return true
	}
	t.Cleanup(func() { chromiumProcessExists = orig })

	require.NoError(t, cleanStaleChromiumSingletons(dir))

	assert.FileExists(t, filepath.Join(dir, "SingletonLock"))
	assert.FileExists(t, filepath.Join(dir, "SingletonSocket"))
	assert.FileExists(t, filepath.Join(dir, "SingletonCookie"))
}

func requireSymlink(t *testing.T, target, path string) {
	t.Helper()
	require.NoError(t, os.Symlink(target, path))
}
