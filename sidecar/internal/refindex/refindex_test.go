package refindex

import (
	"testing"

	"github.com/asgardehs/muninn-sidecar/internal/schema"
)

func TestIndex_UpdateAndQuery(t *testing.T) {
	idx := NewIndex()
	idx.Update("trainings.forklift.md", []ReferenceEdge{
		{Field: "instructor", Target: "people.john-smith", SchemaID: "training"},
	})
	idx.Update("meetings.kickoff.md", []ReferenceEdge{
		{Field: "facilitator", Target: "people.john-smith", SchemaID: "meeting"},
	})

	got := idx.RefsTo("people.john-smith")
	if len(got) != 2 {
		t.Fatalf("expected 2 incoming refs, got %d", len(got))
	}
}

func TestIndex_UpdateReplaces(t *testing.T) {
	idx := NewIndex()
	idx.Update("a.md", []ReferenceEdge{{Field: "x", Target: "old"}})
	idx.Update("a.md", []ReferenceEdge{{Field: "x", Target: "new"}})

	if len(idx.RefsTo("old")) != 0 {
		t.Error("old target should have no refs after Update replaces them")
	}
	if len(idx.RefsTo("new")) != 1 {
		t.Error("new target should have 1 ref")
	}
}

func TestIndex_Remove(t *testing.T) {
	idx := NewIndex()
	idx.Update("a.md", []ReferenceEdge{{Field: "x", Target: "t"}})
	idx.Remove("a.md")
	if len(idx.RefsTo("t")) != 0 {
		t.Error("Remove should drop edges")
	}
}

func TestExtractEdges_PicksReferenceFieldsOnly(t *testing.T) {
	s := &schema.Schema{
		ID: "training",
		Frontmatter: []schema.Field{
			{Key: "title", Type: schema.TypeString},
			{Key: "instructor", Type: schema.TypeReference, Target: "people.**"},
			{Key: "location", Type: schema.TypeString},
			{Key: "facilitator", Type: schema.TypeNoteRef, Target: "people.**"},
		},
	}
	fm := map[string]any{
		"title":       "Forklift",
		"instructor":  "people.john-smith",
		"location":    "Boston",
		"facilitator": "people.jane-doe",
	}
	got := ExtractEdges(s, fm)
	if len(got) != 2 {
		t.Fatalf("expected 2 edges (one per reference-typed field), got %d: %v", len(got), got)
	}
	// Both TypeReference and TypeNoteRef-with-target qualify as typed references.
	// Without target, TypeNoteRef is untyped — covered by wikilink index, not us.
}

func TestExtractEdges_SkipsEmptyValues(t *testing.T) {
	s := &schema.Schema{
		Frontmatter: []schema.Field{
			{Key: "x", Type: schema.TypeReference, Target: "**"},
		},
	}
	if got := ExtractEdges(s, map[string]any{"x": ""}); len(got) != 0 {
		t.Errorf("empty string should produce no edge")
	}
	if got := ExtractEdges(s, map[string]any{}); len(got) != 0 {
		t.Errorf("missing key should produce no edge")
	}
}
