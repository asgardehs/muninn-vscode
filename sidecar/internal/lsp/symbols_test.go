package lsp

import (
	"testing"

	"github.com/asgardehs/muninn-sidecar/internal/markdown"
)

func TestBuildSymbolTreeFlat(t *testing.T) {
	headings := []markdown.Heading{
		{Text: "First", Level: 1, Line: 0},
		{Text: "Second", Level: 1, Line: 5},
		{Text: "Third", Level: 1, Line: 10},
	}

	text := "# First\n\ntext\n\n\n# Second\n\ntext\n\n\n# Third\n"
	symbols := buildSymbolTree(headings, text)

	if len(symbols) != 3 {
		t.Fatalf("got %d root symbols, want 3", len(symbols))
	}
	if symbols[0].Name != "First" {
		t.Errorf("symbols[0].Name = %q, want %q", symbols[0].Name, "First")
	}
}

func TestBuildSymbolTreeNested(t *testing.T) {
	headings := []markdown.Heading{
		{Text: "Chapter", Level: 1, Line: 0},
		{Text: "Section", Level: 2, Line: 2},
		{Text: "Sub", Level: 3, Line: 4},
		{Text: "Chapter 2", Level: 1, Line: 6},
	}

	text := "# Chapter\n\n## Section\n\n### Sub\n\n# Chapter 2\n"
	symbols := buildSymbolTree(headings, text)

	if len(symbols) != 2 {
		t.Fatalf("got %d root symbols, want 2", len(symbols))
	}

	ch1 := symbols[0]
	if ch1.Name != "Chapter" {
		t.Errorf("root[0].Name = %q, want %q", ch1.Name, "Chapter")
	}
	if len(ch1.Children) != 1 {
		t.Fatalf("Chapter children = %d, want 1", len(ch1.Children))
	}

	sec := ch1.Children[0]
	if sec.Name != "Section" {
		t.Errorf("Section.Name = %q, want %q", sec.Name, "Section")
	}
	if len(sec.Children) != 1 {
		t.Fatalf("Section children = %d, want 1", len(sec.Children))
	}
	if sec.Children[0].Name != "Sub" {
		t.Errorf("Sub.Name = %q, want %q", sec.Children[0].Name, "Sub")
	}
}

func TestBuildSymbolTreeEmpty(t *testing.T) {
	symbols := buildSymbolTree(nil, "")
	if len(symbols) != 0 {
		t.Errorf("got %d symbols for empty input, want 0", len(symbols))
	}
}

func TestBuildSymbolTreeSiblings(t *testing.T) {
	headings := []markdown.Heading{
		{Text: "H1", Level: 1, Line: 0},
		{Text: "A", Level: 2, Line: 2},
		{Text: "B", Level: 2, Line: 4},
	}

	text := "# H1\n\n## A\n\n## B\n"
	symbols := buildSymbolTree(headings, text)

	if len(symbols) != 1 {
		t.Fatalf("got %d root symbols, want 1", len(symbols))
	}
	if len(symbols[0].Children) != 2 {
		t.Fatalf("H1 children = %d, want 2", len(symbols[0].Children))
	}
	if symbols[0].Children[0].Name != "A" || symbols[0].Children[1].Name != "B" {
		t.Errorf("got children %q and %q", symbols[0].Children[0].Name, symbols[0].Children[1].Name)
	}
}
