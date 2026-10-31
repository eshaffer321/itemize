package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	targetgo "github.com/daslicious/target-go"
	"github.com/eshaffer321/itemize/internal/infrastructure/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTargetSessionPath(t *testing.T) {
	t.Setenv("ITEMIZE_TARGET_DIR", "/srv/itemize/target")
	path, err := TargetSessionPath("partner")
	require.NoError(t, err)
	assert.Equal(t, "/srv/itemize/target/session-partner.json", path)

	t.Setenv("ITEMIZE_TARGET_DIR", "")
	t.Setenv("HOME", "/home/someone")
	path, err = TargetSessionPath("")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("/home/someone", ".itemize", "target", "session-default.json"), path)

	_, err = TargetSessionPath("a/b")
	assert.Error(t, err)
}

func TestPrintTargetOrdersEmpty(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, PrintTargetOrders(&buf, nil))
	var rows []TargetOrderJSON
	require.NoError(t, json.Unmarshal(buf.Bytes(), &rows))
	assert.Empty(t, rows)
	assert.Equal(t, "[]\n", buf.String())
}

func cliTestJWT(sub string, exp time.Time) string {
	payload := fmt.Sprintf(`{"sub":%q,"sut":"R","exp":%d}`, sub, exp.Unix())
	return "eyJhbGciOiJSUzI1NiJ9." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".c2ln"
}

// The renewal itself is tested in target-go (TestEnsureFresh); this checks the wiring and the
// error message ("target auth check failed") scripts can match on to tell an expired login apart.
func TestNewTargetProvider(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ITEMIZE_TARGET_DIR", dir)
	cfg := &config.Config{}

	_, err := NewTargetProvider(cfg, false, "partner")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target auth check failed")

	path := filepath.Join(dir, "session-partner.json")
	valid := &targetgo.Session{Cookie: "accessToken=" + cliTestJWT("1000", time.Now().Add(20*time.Hour)) + "; refreshToken=TGT.old"}
	require.NoError(t, targetgo.SaveSession(path, valid))
	provider, err := NewTargetProvider(cfg, false, "partner")
	require.NoError(t, err, "a token good for the run is used without contacting Target")
	assert.Equal(t, "target", provider.Name())
}
