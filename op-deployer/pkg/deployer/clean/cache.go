package clean

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum/go-ethereum/log"
	"github.com/urfave/cli/v2"
)

func CacheCLI(cliCtx *cli.Context) error {
	logCfg := oplog.ReadCLIConfig(cliCtx)
	l := oplog.NewLogger(oplog.AppOut(cliCtx), logCfg)
	oplog.SetGlobalLogHandler(l.Handler())

	return CleanCache(l)
}

func CleanCache(l log.Logger) error {
	if err := os.RemoveAll("/tmp/op-deployer-cache"); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove cache directory: %w", err)
	}

	varFolders := "/var/folders"
	err := filepath.Walk(varFolders, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}
		if strings.Contains(info.Name(), "op-deployer") {
			if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
				l.Error("Failed to remove file", "path", path, "err", err)
			}
		}
		return nil
	})
	if err != nil {
		l.Error("Error walking /var/folders", "err", err)
	}

	return nil
}
