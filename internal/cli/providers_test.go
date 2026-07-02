package cli

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/eshaffer321/itemize/internal/infrastructure/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListAmazonAccounts_ReturnsSubdirectoryNames(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "amazon-wife"), 0o700))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "amazon-me"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "not-a-profile.txt"), []byte("x"), 0o600))

	cfg := &config.Config{}
	cfg.Providers.Amazon.BrowserDataDir = dir

	accounts, err := ListAmazonAccounts(cfg)
	require.NoError(t, err)

	sort.Strings(accounts)
	assert.Equal(t, []string{"amazon-me", "amazon-wife"}, accounts, "should list only directories, not files")
}

func TestListAmazonAccounts_MissingDirReturnsEmptyNotError(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.Amazon.BrowserDataDir = filepath.Join(t.TempDir(), "does-not-exist")

	accounts, err := ListAmazonAccounts(cfg)

	require.NoError(t, err, "a browser data dir that was never created is a normal first-run state, not an error")
	assert.Empty(t, accounts)
}
