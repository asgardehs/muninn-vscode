// Package refactor implements vault-wide note renames. A rename moves a
// file on disk and rewrites every [[wikilink]] (and, after Phase C, every
// schema-declared reference field) that points to it. Refactor lives in
// the RPC channel rather than LSP — see docs/specs/v0.2.0.md.
package refactor

import (
	"fmt"
	"strings"

	"github.com/asgardehs/muninn-sidecar/internal/vault"
	"github.com/asgardehs/muninn-sidecar/internal/wikilink"
)

// FileEdit is a planned edit to one file's contents.
type FileEdit struct {
	Path       string // vault-relative path
	OldContent string
	NewContent string
}

// Plan is the full set of changes a single rename will perform. Build it,
// inspect it, then Apply it. Plans are inert — building one does not touch
// the filesystem.
type Plan struct {
	OldName    string     // hierarchy name before rename, e.g. "people.john-smith"
	NewName    string     // hierarchy name after rename
	RenameFrom string     // vault-relative file path to move from
	RenameTo   string     // vault-relative file path to move to
	FileEdits  []FileEdit // per-file content rewrites (backlink sources)
}

// ValidateNames checks the rename inputs for shape problems before any
// filesystem work. Reject identical names (forbid cycles per v0.2.0 scope —
// see docs/specs/v0.2.0.md), reject names containing path separators or
// whitespace, reject leading/trailing/repeated dots.
func ValidateNames(oldName, newName string) error {
	if strings.TrimSpace(oldName) == "" {
		return fmt.Errorf("old name is required")
	}
	if strings.TrimSpace(newName) == "" {
		return fmt.Errorf("new name is required")
	}
	if oldName == newName {
		return fmt.Errorf("new name is identical to old name")
	}
	if err := validateHierarchyName(newName); err != nil {
		return fmt.Errorf("new name %q: %w", newName, err)
	}
	return nil
}

// BuildPlan returns the Plan that would rename a note from oldName to newName.
// It does not touch the filesystem. Inputs are validated via ValidateNames.
// The caller must have an up-to-date wikilink index (refactor does not refresh it).
func BuildPlan(v *vault.Vault, idx *wikilink.Index, oldName, newName string) (*Plan, error) {
	if err := ValidateNames(oldName, newName); err != nil {
		return nil, err
	}

	oldRel := oldName + ".md"
	newRel := newName + ".md"
	if !v.NoteExists(oldRel) {
		return nil, fmt.Errorf("note %q does not exist", oldName)
	}
	if v.NoteExists(newRel) {
		return nil, fmt.Errorf("note %q already exists at target name", newName)
	}

	plan := &Plan{
		OldName:    oldName,
		NewName:    newName,
		RenameFrom: oldRel,
		RenameTo:   newRel,
	}

	for _, src := range idx.Backlinks(oldName) {
		if src == oldRel {
			continue // a note's self-references move with it
		}
		oldContent, err := v.ReadNote(src)
		if err != nil {
			return nil, fmt.Errorf("read backlink source %q: %w", src, err)
		}
		newContent := RewriteWikilinks(oldContent, oldName, newName)
		if newContent == oldContent {
			continue // no actual change (defensive)
		}
		plan.FileEdits = append(plan.FileEdits, FileEdit{
			Path:       src,
			OldContent: oldContent,
			NewContent: newContent,
		})
	}
	return plan, nil
}

// Apply executes a Plan against the vault. Strategy:
//  1. Write each FileEdit's NewContent.
//  2. Rename the file.
//  3. If anything fails, roll back any writes by restoring OldContent.
//
// On success the wikilink index will catch up via fsnotify; the caller can
// also force a refresh.
func Apply(v *vault.Vault, plan *Plan) error {
	written := make([]FileEdit, 0, len(plan.FileEdits))
	for _, edit := range plan.FileEdits {
		if err := v.WriteNote(edit.Path, edit.NewContent); err != nil {
			rollback(v, written)
			return fmt.Errorf("write %q: %w", edit.Path, err)
		}
		written = append(written, edit)
	}
	if err := v.RenameNote(plan.RenameFrom, plan.RenameTo); err != nil {
		rollback(v, written)
		return fmt.Errorf("rename %q -> %q: %w", plan.RenameFrom, plan.RenameTo, err)
	}
	return nil
}

func rollback(v *vault.Vault, written []FileEdit) {
	for _, e := range written {
		_ = v.WriteNote(e.Path, e.OldContent)
	}
}

func validateHierarchyName(name string) error {
	if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") {
		return fmt.Errorf("name must not start or end with '.'")
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("name must not contain consecutive dots")
	}
	for _, r := range name {
		switch {
		case r == '.', r == '-', r == '_':
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		default:
			return fmt.Errorf("name contains invalid character %q", r)
		}
	}
	return nil
}
