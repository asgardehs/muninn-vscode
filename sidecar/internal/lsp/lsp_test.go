package lsp

import (
	"testing"

	"go.lsp.dev/protocol"
)

func TestIsWikilinkContext(t *testing.T) {
	tests := []struct {
		text string
		line uint32
		char uint32
		want bool
	}{
		{"See [[", 0, 6, true},
		{"See [[some", 0, 10, true},
		{"See [[done]]", 0, 12, false},
		{"No brackets here", 0, 5, false},
		{"Before [[ and after ]] then [[new", 0, 32, true},
	}

	for _, tt := range tests {
		pos := protocol.Position{Line: tt.line, Character: tt.char}
		got := isWikilinkContext(tt.text, pos)
		if got != tt.want {
			t.Errorf("isWikilinkContext(%q, %d, %d) = %v, want %v", tt.text, tt.line, tt.char, got, tt.want)
		}
	}
}

func TestWikilinkPartial(t *testing.T) {
	tests := []struct {
		text string
		line uint32
		char uint32
		want string
	}{
		{"See [[some", 0, 10, "some"},
		{"See [[", 0, 6, ""},
		{"See [[ab", 0, 8, "ab"},
	}

	for _, tt := range tests {
		pos := protocol.Position{Line: tt.line, Character: tt.char}
		got := wikilinkPartial(tt.text, pos)
		if got != tt.want {
			t.Errorf("wikilinkPartial(%q, %d, %d) = %q, want %q", tt.text, tt.line, tt.char, got, tt.want)
		}
	}
}

func TestFindWikilinkAt(t *testing.T) {
	text := "See [[My Note]] for details"

	// Inside the wikilink.
	link := findWikilinkAt(text, 0, 8)
	if link == nil {
		t.Fatal("expected link at position 8")
	}
	if link.Target != "My Note" {
		t.Errorf("target = %q, want %q", link.Target, "My Note")
	}

	// Outside the wikilink.
	link = findWikilinkAt(text, 0, 2)
	if link != nil {
		t.Error("expected no link at position 2")
	}
}

func TestOffsetToPosition(t *testing.T) {
	text := "line one\nline two\nline three"

	tests := []struct {
		offset   int
		wantLine int
		wantChar int
	}{
		{0, 0, 0},
		{5, 0, 5},
		{9, 1, 0},  // start of "line two"
		{14, 1, 5}, // "t" in "two"
		{18, 2, 0}, // start of "line three"
	}

	for _, tt := range tests {
		line, char := offsetToPosition(text, tt.offset)
		if line != tt.wantLine || char != tt.wantChar {
			t.Errorf("offsetToPosition(%d) = (%d, %d), want (%d, %d)",
				tt.offset, line, char, tt.wantLine, tt.wantChar)
		}
	}
}

func TestIsTagContext(t *testing.T) {
	tests := []struct {
		text string
		line uint32
		char uint32
		want string
		ok   bool
	}{
		{"text #my", 0, 8, "my", true},
		{"#start", 0, 6, "start", true},
		{" #tag", 0, 5, "tag", true},
		{"no tag here", 0, 5, "", false},
		{"See [[Note#heading", 0, 18, "", false}, // inside wikilink
		{"word#mid", 0, 8, "", false},            // no space before #
	}

	for _, tt := range tests {
		pos := protocol.Position{Line: tt.line, Character: tt.char}
		partial, ok := isTagContext(tt.text, pos)
		if ok != tt.ok || partial != tt.want {
			t.Errorf("isTagContext(%q, %d, %d) = (%q, %v), want (%q, %v)",
				tt.text, tt.line, tt.char, partial, ok, tt.want, tt.ok)
		}
	}
}

func TestIsHeadingFragmentContext(t *testing.T) {
	tests := []struct {
		text       string
		line       uint32
		char       uint32
		wantTarget string
		wantOK     bool
	}{
		{"See [[Note#", 0, 11, "Note", true},
		{"See [[Note#Intro", 0, 16, "Note", true},
		{"See [[Note]]", 0, 10, "", false},
		{"See [[Note", 0, 10, "", false},
		{"No link", 0, 3, "", false},
	}

	for _, tt := range tests {
		pos := protocol.Position{Line: tt.line, Character: tt.char}
		target, _, ok := isHeadingFragmentContext(tt.text, pos)
		if ok != tt.wantOK || (ok && target != tt.wantTarget) {
			t.Errorf("isHeadingFragmentContext(%q, %d, %d) target=%q ok=%v, want target=%q ok=%v",
				tt.text, tt.line, tt.char, target, ok, tt.wantTarget, tt.wantOK)
		}
	}
}

func TestFindWikilinkAtWithFragment(t *testing.T) {
	text := "See [[My Note#Intro]] for details"

	link := findWikilinkAt(text, 0, 10)
	if link == nil {
		t.Fatal("expected link at position 10")
	}
	if link.Target != "My Note" {
		t.Errorf("target = %q, want %q", link.Target, "My Note")
	}
	if link.Fragment != "Intro" {
		t.Errorf("fragment = %q, want %q", link.Fragment, "Intro")
	}
}

func TestNotePreviewFromHeading(t *testing.T) {
	content := "---\ntitle: Test\n---\n\n# Main\n\nIntro text.\n\n## Section\n\nSection content.\nMore content.\n\n## Another"

	preview := notePreviewFromHeading(content, "Test", "Section")
	if preview == "" {
		t.Fatal("expected non-empty preview")
	}
	if !contains(preview, "Section content") {
		t.Error("preview should contain section content")
	}
	if contains(preview, "Intro text") {
		t.Error("preview should NOT contain content before the heading")
	}
}

func TestNotePreview(t *testing.T) {
	content := `---
title: "Test"
---

# Test

First paragraph.
Second line.`

	preview := notePreview(content, "Test")
	if preview == "" {
		t.Fatal("expected non-empty preview")
	}
	if !contains(preview, "**Test**") {
		t.Error("preview should contain bold title")
	}
	if !contains(preview, "First paragraph") {
		t.Error("preview should contain body content")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsHelper(s, substr)
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
