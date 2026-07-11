package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eshaffer321/itemize/internal/infrastructure/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListAmazonAccounts_ReturnsAmazonGoCookieAccounts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".amazon-go")
	require.NoError(t, os.Mkdir(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cookies-amazon-wife.json"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cookies-amazon-me.json"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cookies.json"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "not-a-profile.txt"), []byte("x"), 0o600))
	cfg := &config.Config{}

	accounts, err := ListAmazonAccounts(cfg)
	require.NoError(t, err)

	assert.Equal(t, []string{"amazon-me", "amazon-wife"}, accounts, "should list only amazon-go account cookie files")
}

func TestListAmazonAccounts_MissingDirReturnsEmptyNotError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := &config.Config{}

	accounts, err := ListAmazonAccounts(cfg)

	require.NoError(t, err, "an amazon-go cookie dir that was never created is a normal first-run state, not an error")
	assert.Empty(t, accounts)
}

func TestResolveAmazonAccount_UsesSinglePositionalAccount(t *testing.T) {
	cfg := &config.Config{}
	account, err := ResolveAmazonAccount(cfg, "", []string{"amazon-wife"})

	require.NoError(t, err)
	assert.Equal(t, "amazon-wife", account)
}

func TestResolveAmazonAccount_RejectsUnexpectedExtraArgs(t *testing.T) {
	cfg := &config.Config{}
	_, err := ResolveAmazonAccount(cfg, "", []string{"amazon-wife", "extra"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected arguments")
	assert.Contains(t, err.Error(), "-account amazon-wife")
}

func TestResolveAmazonAccount_FlagBeatsConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.Amazon.AccountName = "from-config"

	account, err := ResolveAmazonAccount(cfg, "from-flag", nil)

	require.NoError(t, err)
	assert.Equal(t, "from-flag", account)
}

func TestResolveAmazonAccount_RejectsPositionalAccountWhenFlagSet(t *testing.T) {
	cfg := &config.Config{}
	_, err := ResolveAmazonAccount(cfg, "from-flag", []string{"from-positional"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "use either -account or a positional account")
}
