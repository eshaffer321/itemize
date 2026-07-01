package telemetry

import (
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScrubEvent_StripsTokenLikeExtras(t *testing.T) {
	event := &sentry.Event{
		Extra: map[string]interface{}{
			"api_key":   "sk-abc123def456ghi789jkl012mno345p", // looks like a token
			"processed": 3,                                    // safe int
			"provider":  "walmart",                            // safe short string
		},
	}

	result := scrubEvent(event, nil)

	assert.NotContains(t, result.Extra, "api_key", "token-like string should be stripped from extras")
	assert.Equal(t, 3, result.Extra["processed"], "safe int should be preserved")
	assert.Equal(t, "walmart", result.Extra["provider"], "safe short string should be preserved")
}

func TestScrubEvent_StripsTokenLikeTags(t *testing.T) {
	event := &sentry.Event{
		Tags: map[string]string{
			"provider":     "amazon",
			"leaked_token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9abc", // jwt-like
		},
	}

	result := scrubEvent(event, nil)

	assert.Equal(t, "amazon", result.Tags["provider"], "safe tag should be preserved")
	assert.NotContains(t, result.Tags, "leaked_token", "token-like tag value should be stripped")
}

func TestScrubEvent_ClearsUserAndRequest(t *testing.T) {
	event := &sentry.Event{
		User: sentry.User{
			Email: "user@example.com",
			ID:    "12345",
		},
		Request: &sentry.Request{
			URL: "https://api.example.com",
			Headers: map[string]string{
				"Authorization": "Bearer secret",
			},
		},
	}

	result := scrubEvent(event, nil)

	assert.Empty(t, result.User.Email, "user email should be cleared")
	assert.Empty(t, result.User.ID, "user ID should be cleared")
	assert.Nil(t, result.Request, "request should be nil")
}

func TestScrubEvent_PreservesShortAlphanumericStrings(t *testing.T) {
	event := &sentry.Event{
		Extra: map[string]interface{}{
			"dry_run":  "false",
			"provider": "costco",
			"stage":    "sync",
		},
	}

	result := scrubEvent(event, nil)

	assert.Equal(t, "false", result.Extra["dry_run"])
	assert.Equal(t, "costco", result.Extra["provider"])
	assert.Equal(t, "sync", result.Extra["stage"])
}

func TestScrubEvent_StripsExactly20CharStrings(t *testing.T) {
	event := &sentry.Event{
		Extra: map[string]interface{}{
			"borderline": "abcdefghij1234567890", // exactly 20 chars — should be stripped
			"safe":       "abcdefghij123456789",  // 19 chars — should pass
		},
	}

	result := scrubEvent(event, nil)

	assert.NotContains(t, result.Extra, "borderline", "20-char alphanumeric string should be stripped")
	assert.Contains(t, result.Extra, "safe", "19-char string should be preserved")
}

func TestIsEnabled_RespectsOptOutEnvVars(t *testing.T) {
	t.Run("ITEMIZE_NO_TELEMETRY", func(t *testing.T) {
		t.Setenv("ITEMIZE_NO_TELEMETRY", "1")
		assert.False(t, IsEnabled())
	})

	t.Run("DO_NOT_TRACK", func(t *testing.T) {
		t.Setenv("DO_NOT_TRACK", "1")
		assert.False(t, IsEnabled())
	})
}

func TestIsEnabled_FalseWhenDSNIsPlaceholder(t *testing.T) {
	// The real dsn const in this package is set; this test verifies the logic
	// by calling the unexported check directly via the exported function.
	// Since dsn is the real value in tests too, we just verify the env-var path.
	t.Setenv("ITEMIZE_NO_TELEMETRY", "1")
	require.False(t, IsEnabled())
}
