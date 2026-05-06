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

// NoteExists reports whether a note exists at the given vault-relative path.
func (v *Vault) NoteExists(relPath string) bool {
	_, err := os.Stat(filepath.Join(v.root, relPath))
	return err == nil
}

// CreateNote writes content to a new note at relPath. It creates intermediate
// directories as needed and refuses to overwrite an existing file.
func (v *Vault) CreateNote(relPath, content string) error {
	abs := filepath.Join(v.root, relPath)
	if _, err := os.Stat(abs); err == nil {
		return fmt.Errorf("note %q already exists", relPath)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return fmt.Errorf("create parent dirs: %w", err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write note: %w", err)
	}
	return nil
}

// WriteNote overwrites an existing note with new content. Unlike CreateNote it
// does not refuse if the file already exists — it is intended for in-place
// rewrites (e.g. backlink updates during a rename operation).
func (v *Vault) WriteNote(relPath, content string) error {
	abs := filepath.Join(v.root, relPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return fmt.Errorf("create parent dirs: %w", err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write note: %w", err)
	}
	return nil
}

// RenameNote moves a note from the vault-relative path `from` to `to`. It
// explicitly checks that the destination does not already exist and returns an
// error before attempting the OS rename — this makes cross-platform behaviour
// consistent and keeps Apply's rollback test reliable.
func (v *Vault) RenameNote(from, to string) error {
	absFrom := filepath.Join(v.root, from)
	absTo := filepath.Join(v.root, to)
	if _, err := os.Stat(absTo); err == nil {
		return fmt.Errorf("destination %q already exists", to)
	}
	if err := os.MkdirAll(filepath.Dir(absTo), 0o755); err != nil {
		return fmt.Errorf("create parent dirs: %w", err)
	}
	if err := os.Rename(absFrom, absTo); err != nil {
		return fmt.Errorf("rename %q -> %q: %w", from, to, err)
	}
	return nil
}
