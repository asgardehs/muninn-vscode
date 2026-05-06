package refindex

import "testing"

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
