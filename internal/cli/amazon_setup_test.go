package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareAmazonSetup_CreatesDefaultProfileDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	profileDir, err := PrepareAmazonSetup("wife")

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".itemize", "amazon", "wife"), profileDir)
	info, err := os.Stat(profileDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	assert.Equal(t, os.FileMode(0700), info.Mode().Perm())
}

func TestPrepareAmazonSetup_RequiresAccount(t *testing.T) {
	_, err := PrepareAmazonSetup("")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "-account")
}

func TestPrepareAmazonSetup_RejectsUnsafeAccountName(t *testing.T) {
	_, err := PrepareAmazonSetup("../wife")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "letters, numbers, dashes, and underscores")
}

func TestPrintAmazonUsage_PutsSetupBeforeSyncAndAdvancedFlags(t *testing.T) {
	var out bytes.Buffer

	PrintAmazonUsage(&out)

	help := out.String()
	setup := strings.Index(help, "First-time setup")
	sync := strings.Index(help, "Sync options")
	advanced := strings.Index(help, "Advanced authentication")
	require.NotEqual(t, -1, setup)
	require.NotEqual(t, -1, sync)
	require.NotEqual(t, -1, advanced)
	assert.Less(t, setup, sync)
	assert.Less(t, sync, advanced)
	assert.Contains(t, help, "itemize amazon setup -account <name>")
	assert.Contains(t, help, "Creates a browser profile and opens Chromium")
}
