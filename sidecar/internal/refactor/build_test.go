package refactor_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/asgardehs/muninn-sidecar/internal/refactor"
	"github.com/asgardehs/muninn-sidecar/internal/refindex"
	"github.com/asgardehs/muninn-sidecar/internal/vault"
	"github.com/asgardehs/muninn-sidecar/internal/wikilink"
)

func TestBuildPlan_RenamesFileAndRewritesBacklinks(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, dir, "people.john-smith.md", "# John\n")
	mustWrite(t, dir, "trainings.forklift.md", "Taught by [[people.john-smith]].\n")
	mustWrite(t, dir, "meetings.kickoff.md", "Notes — [[people.john-smith#bio]] attended.\n")

	v := vault.New(dir)
	idx := wikilink.NewIndex()
	for _, f := range mustListNotes(t, v) {
		content := mustRead(t, v, f)
		idx.Update(f, wikilink.Extract(content))
	}

	plan, err := refactor.BuildPlan(v, idx, "people.john-smith", "people.john-doe")
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if plan.RenameFrom != "people.john-smith.md" {
		t.Errorf("RenameFrom = %q", plan.RenameFrom)
	}
	if plan.RenameTo != "people.john-doe.md" {
		t.Errorf("RenameTo = %q", plan.RenameTo)
	}
	if len(plan.FileEdits) != 2 {
		t.Fatalf("expected 2 backlink-source edits, got %d", len(plan.FileEdits))
	}
}

// helpers

func mustWrite(t *testing.T, dir, rel, body string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustListNotes(t *testing.T, v *vault.Vault) []string {
	t.Helper()
	notes, err := v.ListNotes()
	if err != nil {
		t.Fatal(err)
	}
	return notes
}

func mustRead(t *testing.T, v *vault.Vault, p string) string {
	t.Helper()
	b, err := v.ReadNote(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestBuildPlan_RewritesReferenceFields(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, dir, "people.john-smith.md", "# John\n")
	mustWrite(t, dir, "trainings.forklift.md",
		"---\ntitle: Forklift\ninstructor: people.john-smith\n---\n\nbody\n")

	v := vault.New(dir)
	wikiIdx := wikilink.NewIndex()
	refIdx := refindex.NewIndex()
	// Pre-seed the refindex with the known edge — in production this is built
	// by the lifecycle wiring (Task C3).
	refIdx.Update("trainings.forklift.md", []refindex.ReferenceEdge{
		{Field: "instructor", Target: "people.john-smith", SchemaID: "training"},
	})

	plan, err := refactor.BuildPlanWithRefs(v, wikiIdx, refIdx, "people.john-smith", "people.john-doe")
	if err != nil {
		t.Fatalf("BuildPlanWithRefs: %v", err)
	}
	if len(plan.FileEdits) != 1 {
		t.Fatalf("expected 1 frontmatter edit, got %d", len(plan.FileEdits))
	}
	edit := plan.FileEdits[0]
	if !strings.Contains(edit.NewContent, "instructor: people.john-doe") {
		t.Errorf("frontmatter not rewritten:\n%s", edit.NewContent)
	}
	if strings.Contains(edit.NewContent, "instructor: people.john-smith") {
		t.Errorf("old reference still present:\n%s", edit.NewContent)
	}
}
