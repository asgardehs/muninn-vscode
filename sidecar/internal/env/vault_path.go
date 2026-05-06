// Package env resolves the vault root path using a layered precedence chain.
//
// In v0.1 the VS Code workspace folder is the vault root by default — no
// .muninn/notes subdirectory, no Heimdall lookup. The Resolve function below
// supports two override paths for advanced or Asgard-integrated setups:
//
//  1. MUNINN_VAULT_PATH env var (highest priority — useful for tests/scripts)
//  2. Heimdall config muninn.vault_path (only consulted when useHeimdall=true)
//  3. The provided workspace path (default)
package env

import (
	"log/slog"
	"os"

	"github.com/asgardehs/heimdall"
)

// Resolve returns the vault root for the given workspace folder.
// Heimdall is consulted only when useHeimdall is true.
func Resolve(workspacePath string, useHeimdall bool) string {
	if p := os.Getenv("MUNINN_VAULT_PATH"); p != "" {
		return p
	}
	if useHeimdall {
		if p := heimdallVaultPath(); p != "" {
			return p
		}
	}
	return workspacePath
}

// heimdallVaultPath reads vault_path from Heimdall if its DB exists.
// Returns empty string on any error or if the key is unset.
func heimdallVaultPath() string {
	dbPath := heimdall.DefaultDBPath()
	if _, err := os.Stat(dbPath); err != nil {
		return ""
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	h, err := heimdall.Open(logger)
	if err != nil {
		return ""
	}
	defer h.Close()
	entry, err := h.Get("muninn", "vault_path")
	if err != nil || entry.Value == "" {
		return ""
	}
	return entry.Value
}
