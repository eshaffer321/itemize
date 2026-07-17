package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	amazon "github.com/eshaffer321/amazon-go"
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

func TestSaveImportedAmazonCookies_DoesNotOverwriteDestinationWhenValidationFails(t *testing.T) {
	cookieFile := filepath.Join(t.TempDir(), "cookies-amazon-wife.json")
	original := []byte(`{"cookies":[{"name":"original","value":"keep-me","domain":".amazon.com","path":"/"}],"updated_at":"2026-01-01T00:00:00Z"}`)
	require.NoError(t, os.WriteFile(cookieFile, original, 0600))

	err := saveImportedAmazonCookies([]*amazon.Cookie{
		{Name: "session-id", Value: "missing-other-essential-cookies", Domain: ".amazon.com", Path: "/"},
	}, AmazonImportOptions{CookieFile: cookieFile})

	require.Error(t, err)
	var authErr *amazonImportAuthCheckError
	assert.ErrorAs(t, err, &authErr)
	after, readErr := os.ReadFile(cookieFile)
	require.NoError(t, readErr)
	assert.JSONEq(t, string(original), string(after))
}

func TestValidateImportedAmazonCookies_ReachesAuthCheck(t *testing.T) {
	err := validateImportedAmazonCookies([]*amazon.Cookie{
		{Name: "session-id", Value: "not-enough-cookies", Domain: ".amazon.com", Path: "/"},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing essential cookies")
	assert.NotContains(t, err.Error(), "unexpected end of JSON input")
}

func TestSaveImportedAmazonCookies_SavesDestinationWhenAuthCheckSkipped(t *testing.T) {
	cookieFile := filepath.Join(t.TempDir(), "cookies-amazon-wife.json")

	err := saveImportedAmazonCookies([]*amazon.Cookie{
		{Name: "session-id", Value: "sid", Domain: ".amazon.com", Path: "/"},
		{Name: "session-token", Value: "token", Domain: ".amazon.com", Path: "/"},
		{Name: "ubid-main", Value: "ubid", Domain: ".amazon.com", Path: "/"},
		{Name: "at-main", Value: "at", Domain: ".amazon.com", Path: "/"},
	}, AmazonImportOptions{CookieFile: cookieFile, SkipAuthCheck: true})

	require.NoError(t, err)
	data, err := os.ReadFile(cookieFile)
	require.NoError(t, err)
	var stored struct {
		Cookies []amazon.Cookie `json:"cookies"`
	}
	require.NoError(t, json.Unmarshal(data, &stored))
	assert.Len(t, stored.Cookies, 4)
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
