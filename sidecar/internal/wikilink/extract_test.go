package wikilink

import "testing"

func TestExtract(t *testing.T) {
	text := "See [[Some Note]] and [[Another|display text]] for details."

	links := Extract(text)
	if len(links) != 2 {
		t.Fatalf("links = %d, want 2", len(links))
	}

	if links[0].Target != "Some Note" {
		t.Errorf("link[0].Target = %q, want %q", links[0].Target, "Some Note")
	}
	if links[0].Fragment != "" {
		t.Errorf("link[0].Fragment = %q, want empty", links[0].Fragment)
	}
	if links[0].Alias != "" {
		t.Errorf("link[0].Alias = %q, want empty", links[0].Alias)
	}

	if links[1].Target != "Another" {
		t.Errorf("link[1].Target = %q, want %q", links[1].Target, "Another")
	}
	if links[1].Alias != "display text" {
		t.Errorf("link[1].Alias = %q, want %q", links[1].Alias, "display text")
	}
}

func TestExtractWithFragment(t *testing.T) {
	text := "See [[Note#Introduction]] and [[Note#Some Heading|click here]]."

	links := Extract(text)
	if len(links) != 2 {
		t.Fatalf("links = %d, want 2", len(links))
	}

	if links[0].Target != "Note" {
		t.Errorf("link[0].Target = %q, want %q", links[0].Target, "Note")
	}
	if links[0].Fragment != "Introduction" {
		t.Errorf("link[0].Fragment = %q, want %q", links[0].Fragment, "Introduction")
	}
	if links[0].Alias != "" {
		t.Errorf("link[0].Alias = %q, want empty", links[0].Alias)
	}

	if links[1].Target != "Note" {
		t.Errorf("link[1].Target = %q, want %q", links[1].Target, "Note")
	}
	if links[1].Fragment != "Some Heading" {
		t.Errorf("link[1].Fragment = %q, want %q", links[1].Fragment, "Some Heading")
	}
	if links[1].Alias != "click here" {
		t.Errorf("link[1].Alias = %q, want %q", links[1].Alias, "click here")
	}
}

func TestNormalizeFragment(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"Introduction", "introduction"},
		{"  Some Heading  ", "some heading"},
	}
	for _, tt := range tests {
		got := NormalizeFragment(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeFragment(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExtractEmpty(t *testing.T) {
	links := Extract("")
	if len(links) != 0 {
		t.Errorf("expected 0 links, got %d", len(links))
	}
}

func TestTargetsDedup(t *testing.T) {
	text := "[[Alpha]] and [[alpha]] and [[Beta]]"
	targets := Targets(text)

	if len(targets) != 2 {
		t.Fatalf("targets = %d, want 2 (deduped)", len(targets))
	}
}

func TestNormalizeTarget(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"Hello World", "hello world"},
		{"  Trimmed  ", "trimmed"},
		{"already-lower", "already-lower"},
	}

	for _, tt := range tests {
		got := NormalizeTarget(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeTarget(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIndex(t *testing.T) {
	idx := NewIndex()

	idx.Update("note-a.md", []WikiLink{
		{Target: "Note B"},
		{Target: "Note C"},
	})
	idx.Update("note-b.md", []WikiLink{
		{Target: "Note A"},
	})

	// Forward links.
	fwd := idx.ForwardLinks("note-a.md")
	if len(fwd) != 2 {
		t.Errorf("forward links = %d, want 2", len(fwd))
	}

	// Backlinks.
	bl := idx.Backlinks("Note B")
	if len(bl) != 1 || bl[0] != "note-a.md" {
		t.Errorf("backlinks for Note B = %v, want [note-a.md]", bl)
	}

	// Remove.
	idx.Remove("note-a.md")
	bl = idx.Backlinks("Note B")
	if len(bl) != 0 {
		t.Errorf("backlinks after remove = %v, want empty", bl)
	}
}
