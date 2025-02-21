package clean

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/stretchr/testify/require"
)

func TestCacheCLI(t *testing.T) {
	tmpDir := "/tmp/op-deployer-cache"
	require.NoError(t, os.MkdirAll(tmpDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644))

	lgr := testlog.Logger(t, slog.LevelDebug)

	varDir := filepath.Join("/var/folders", fmt.Sprintf("op-deployer-artifacts-%d", 12345))
	var notPermitted bool
	if err := os.MkdirAll(varDir, 0755); err != nil {
		if os.IsPermission(err) {
			notPermitted = true
			lgr.Warn("Skipping /var/folders check due to permissions-issue when trying to create the directory")
		} else {
			t.Fatal(err)
		}
	}
	if !notPermitted {
		if err := os.WriteFile(filepath.Join(varDir, "test.txt"), []byte("test"), 0644); err != nil {
			require.NoError(t, err)
		}
	}

	err := CleanCache(lgr)
	if err != nil {
		t.Fatal(err)
	}

	require.NoDirExists(t, tmpDir)
	if !notPermitted {
		require.NoDirExists(t, varDir)
	}
}
