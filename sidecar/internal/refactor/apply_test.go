package refactor_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/asgardehs/muninn-sidecar/internal/refactor"
	"github.com/asgardehs/muninn-sidecar/internal/vault"
	"github.com/asgardehs/muninn-sidecar/internal/wikilink"
)

func TestApply_MovesFileAndRewritesBacklinks(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, dir, "people.john-smith.md", "# John\n")
	mustWrite(t, dir, "trainings.forklift.md", "Taught by [[people.john-smith]].\n")

	v := vault.New(dir)
	idx := wikilink.NewIndex()
	for _, f := range mustListNotes(t, v) {
		idx.Update(f, wikilink.Extract(mustRead(t, v, f)))
	}

	plan, err := refactor.BuildPlan(v, idx, "people.john-smith", "people.john-doe")
	if err != nil {
		t.Fatal(err)
	}
	if err := refactor.Apply(v, plan); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// File renamed?
	if _, err := os.Stat(filepath.Join(dir, "people.john-doe.md")); err != nil {
		t.Errorf("new file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "people.john-smith.md")); !os.IsNotExist(err) {
		t.Errorf("old file should be gone, stat err = %v", err)
	}

	// Backlink rewritten?
	got := mustRead(t, v, "trainings.forklift.md")
	if !strings.Contains(got, "[[people.john-doe]]") {
		t.Errorf("backlink not rewritten, got: %q", got)
	}
	if strings.Contains(got, "[[people.john-smith]]") {
		t.Errorf("old wikilink still present, got: %q", got)
	}
}

func TestApply_RollsBackOnRenameFailure(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, dir, "old.md", "# Old\n")
	mustWrite(t, dir, "ref.md", "see [[old]]\n")
	// Pre-create the destination so the rename collides at the os.Rename step.
	mustWrite(t, dir, "new.md", "blocking\n")

	v := vault.New(dir)
	// Bypass BuildPlan's collision check — we want to exercise Apply's rollback.
	plan := &refactor.Plan{
		OldName:    "old",
		NewName:    "new",
		RenameFrom: "old.md",
		RenameTo:   "new.md",
		FileEdits: []refactor.FileEdit{
			{Path: "ref.md", OldContent: "see [[old]]\n", NewContent: "see [[new]]\n"},
		},
	}
	if err := refactor.Apply(v, plan); err == nil {
		t.Fatal("expected Apply to fail on rename collision")
	}
	got := mustRead(t, v, "ref.md")
	if got != "see [[old]]\n" {
		t.Errorf("rollback failed, ref.md = %q", got)
	}
}
