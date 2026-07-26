package cli

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	_, err := validateAndRefreshImportedAmazonCookies([]*amazon.Cookie{
		{Name: "session-id", Value: "not-enough-cookies", Domain: ".amazon.com", Path: "/"},
	}, amazon.BrowserFingerprint{})

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

func TestAmazonCookieDestination(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	explicit := filepath.Join(t.TempDir(), "explicit.json")
	resolved, err := amazonCookieDestination(explicit, "ignored")
	require.NoError(t, err)
	assert.Equal(t, explicit, resolved)

	resolved, err = amazonCookieDestination("", "wife")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".amazon-go", "cookies-wife.json"), resolved)

	resolved, err = amazonCookieDestination("", "")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".amazon-go", "cookies.json"), resolved)
}

func TestSaveImportedAmazonCookies_ReplacesStaleDestinationSnapshot(t *testing.T) {
	cookieFile := filepath.Join(t.TempDir(), "cookies-amazon-wife.json")
	stale := amazon.CookieFile{
		Cookies: []*amazon.Cookie{
			{Name: "expired-antibot", Value: "stale", Domain: ".amazon.com", Path: "/", Expires: time.Now().Add(-time.Hour).Unix()},
			{Name: "session-id", Value: "old-session", Domain: ".amazon.com", Path: "/"},
		},
	}
	data, err := json.Marshal(stale)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cookieFile, data, 0o600))

	err = saveImportedAmazonCookies([]*amazon.Cookie{
		{Name: "session-id", Value: "new-session", Domain: ".amazon.com", Path: "/"},
		{Name: "session-token", Value: "token", Domain: ".amazon.com", Path: "/"},
		{Name: "ubid-main", Value: "ubid", Domain: ".amazon.com", Path: "/"},
		{Name: "at-main", Value: "at", Domain: ".amazon.com", Path: "/"},
	}, AmazonImportOptions{CookieFile: cookieFile, SkipAuthCheck: true})

	require.NoError(t, err)
	data, err = os.ReadFile(cookieFile)
	require.NoError(t, err)
	var stored amazon.CookieFile
	require.NoError(t, json.Unmarshal(data, &stored))
	require.Len(t, stored.Cookies, 4)
	for _, cookie := range stored.Cookies {
		assert.NotEqual(t, "expired-antibot", cookie.Name)
		if cookie.Name == "session-id" {
			assert.Equal(t, "new-session", cookie.Value)
		}
	}
}

func TestSaveImportedAmazonCookies_ValidatesAndPersistsBrowserFingerprintWithRefreshedCookies(t *testing.T) {
	cookieFile := filepath.Join(t.TempDir(), "cookies-amazon-wife.json")
	const browserUserAgent = "Mozilla/5.0 Chrome/143.0.7499.4 Safari/537.36"
	fingerprint := amazon.BrowserFingerprint{
		UserAgent:       browserUserAgent,
		SecCHUA:         `"Chromium";v="143", "Not A(Brand";v="24"`,
		SecCHUAMobile:   "?0",
		SecCHUAPlatform: `"macOS"`,
	}

	var receivedUserAgent string
	var receivedSecCHUA string
	var receivedSecCHUAMobile string
	var receivedSecCHUAPlatform string
	originalTransport := http.DefaultTransport
	http.DefaultTransport = cliRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		receivedUserAgent = req.UserAgent()
		receivedSecCHUA = req.Header.Get("Sec-CH-UA")
		receivedSecCHUAMobile = req.Header.Get("Sec-CH-UA-Mobile")
		receivedSecCHUAPlatform = req.Header.Get("Sec-CH-UA-Platform")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Set-Cookie": []string{"session-token=refreshed-token; Path=/; Domain=.amazon.com"},
			},
			Body:    io.NopCloser(strings.NewReader(`<html><title>Your Orders</title></html>`)),
			Request: req,
		}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	err := saveImportedAmazonCookies([]*amazon.Cookie{
		{Name: "session-id", Value: "session", Domain: ".amazon.com", Path: "/"},
		{Name: "session-token", Value: "exported-token", Domain: ".amazon.com", Path: "/"},
		{Name: "ubid-main", Value: "ubid", Domain: ".amazon.com", Path: "/"},
		{Name: "at-main", Value: "at", Domain: ".amazon.com", Path: "/"},
	}, AmazonImportOptions{CookieFile: cookieFile, BrowserFingerprint: fingerprint})

	require.NoError(t, err)
	assert.Equal(t, browserUserAgent, receivedUserAgent)
	assert.Equal(t, fingerprint.SecCHUA, receivedSecCHUA)
	assert.Equal(t, fingerprint.SecCHUAMobile, receivedSecCHUAMobile)
	assert.Equal(t, fingerprint.SecCHUAPlatform, receivedSecCHUAPlatform)
	data, err := os.ReadFile(cookieFile)
	require.NoError(t, err)
	var stored amazon.CookieFile
	require.NoError(t, json.Unmarshal(data, &stored))
	require.NotNil(t, stored.BrowserFingerprint)
	assert.Equal(t, fingerprint, *stored.BrowserFingerprint)
	for _, cookie := range stored.Cookies {
		if cookie.Name == "session-token" {
			assert.Equal(t, "refreshed-token", cookie.Value)
			return
		}
	}
	t.Fatal("saved session-token cookie not found")
}

type cliRoundTripFunc func(*http.Request) (*http.Response, error)

func (f cliRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestWriteAmazonCookieExportScriptCapturesFullBrowserFingerprint(t *testing.T) {
	scriptPath, err := writeAmazonCookieExportScript(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(scriptPath) })

	script, err := os.ReadFile(scriptPath)
	require.NoError(t, err)
	assert.Contains(t, string(script), "browserFingerprint")
	assert.Contains(t, string(script), "sec_ch_ua_mobile")
	assert.Contains(t, string(script), `"sec-ch-ua-mobile": "?0"`)
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
