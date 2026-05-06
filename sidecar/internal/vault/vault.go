// Package vault manages the notes directory on disk.
package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Vault represents a vault root directory containing markdown notes.
// In v0.1 the workspace folder is treated as the vault root directly;
// there is no separate notes/ subdirectory.
type Vault struct {
	root string
}

// New creates a Vault rooted at the given directory.
func New(root string) *Vault {
	return &Vault{root: root}
}

// Root returns the absolute vault root path.
func (v *Vault) Root() string { return v.root }

// AbsPath returns the absolute path for a vault-relative note path.
func (v *Vault) AbsPath(relPath string) string {
	return filepath.Join(v.root, relPath)
}

// ListNotes returns vault-relative paths of every .md file under the root,
// excluding hidden directories (e.g., .git, .muninn). Sorted lexicographically
// by filepath.Walk's traversal order.
func (v *Vault) ListNotes() ([]string, error) {
	var files []string
	err := filepath.Walk(v.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && strings.HasPrefix(info.Name(), ".") && path != v.root {
			return filepath.SkipDir
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".md") {
			rel, err := filepath.Rel(v.root, path)
			if err != nil {
				return err
			}
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking vault: %w", err)
	}
	return files, nil
}

// ReadNote reads the full content of a note by vault-relative path.
func (v *Vault) ReadNote(relPath string) (string, error) {
	data, err := os.ReadFile(filepath.Join(v.root, relPath))
	if err != nil {
		return "", fmt.Errorf("reading note: %w", err)
	}
	return string(data), nil
}
