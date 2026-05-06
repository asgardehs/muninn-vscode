package refactor_test

import (
	"strings"
	"testing"

	"github.com/asgardehs/muninn-sidecar/internal/refactor"
	"github.com/asgardehs/muninn-sidecar/internal/refindex"
	"github.com/asgardehs/muninn-sidecar/internal/vault"
	"github.com/asgardehs/muninn-sidecar/internal/wikilink"
)

// Renaming a person updates: (a) the file on disk, (b) every wikilink
// pointing to the old name, (c) every typed-reference frontmatter field
// pointing to the old name. All in one Apply call.
func TestEndToEnd_RenamePersonWithWikilinkAndReferenceField(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, dir, "people.john-smith.md", "---\ntitle: John Smith\n---\n\n# Bio\n")
	mustWrite(t, dir, "trainings.forklift.md",
		"---\ntitle: Forklift\ninstructor: people.john-smith\n---\n\nTaught by [[people.john-smith]].\n")

	v := vault.New(dir)
	wikiIdx := wikilink.NewIndex()
	refIdx := refindex.NewIndex()
	for _, f := range mustListNotes(t, v) {
		wikiIdx.Update(f, wikilink.Extract(mustRead(t, v, f)))
	}
	refIdx.Update("trainings.forklift.md", []refindex.ReferenceEdge{
		{Field: "instructor", Target: "people.john-smith", SchemaID: "training"},
	})

	plan, err := refactor.BuildPlanWithRefs(v, wikiIdx, refIdx,
		"people.john-smith", "people.john-doe")
	if err != nil {
		t.Fatalf("BuildPlanWithRefs: %v", err)
	}
	if err := refactor.Apply(v, plan); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got := mustRead(t, v, "trainings.forklift.md")
	if !strings.Contains(got, "instructor: people.john-doe") {
		t.Errorf("frontmatter reference not rewritten:\n%s", got)
	}
	if !strings.Contains(got, "[[people.john-doe]]") {
		t.Errorf("wikilink not rewritten:\n%s", got)
	}
	if strings.Contains(got, "people.john-smith") {
		t.Errorf("old name still present somewhere:\n%s", got)
	}
}
