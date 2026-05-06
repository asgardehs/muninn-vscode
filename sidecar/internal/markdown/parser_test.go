package markdown

import (
	"testing"
)

func TestParseFrontmatterAndTitle(t *testing.T) {
	source := `---
title: "My Note"
tags: [go, testing]
---

# Heading

Some content here.
`
	p := NewParser()
	doc := p.Parse(source)

	if doc.Frontmatter == "" {
		t.Fatal("expected frontmatter")
	}
	if doc.Title != "My Note" {
		t.Errorf("title = %q, want %q", doc.Title, "My Note")
	}
	if doc.Body == "" {
		t.Fatal("expected body")
	}
}

func TestParseTitleFromHeading(t *testing.T) {
	source := "# Hello World\n\nSome text."
	p := NewParser()
	doc := p.Parse(source)

	if doc.Title != "Hello World" {
		t.Errorf("title = %q, want %q", doc.Title, "Hello World")
	}
}

func TestParseNoFrontmatter(t *testing.T) {
	source := "Just a plain document.\n"
	p := NewParser()
	doc := p.Parse(source)

	if doc.Frontmatter != "" {
		t.Errorf("frontmatter = %q, want empty", doc.Frontmatter)
	}
	if doc.Body != source {
		t.Errorf("body mismatch")
	}
}

func TestParseFrontmatter(t *testing.T) {
	raw := `title: Test
tags:
  - go
  - testing
author:
  name: Alice`

	entries := ParseFrontmatter(raw)
	if len(entries) == 0 {
		t.Fatal("expected entries")
	}

	// Check that tags are expanded.
	tagCount := 0
	for _, e := range entries {
		if e.Key == "tags" {
			tagCount++
		}
	}
	if tagCount != 2 {
		t.Errorf("tag entries = %d, want 2", tagCount)
	}

	// Check nested key.
	found := false
	for _, e := range entries {
		if e.Key == "author.name" && e.Value == "Alice" {
			found = true
		}
	}
	if !found {
		t.Error("expected author.name = Alice")
	}
}

func TestParseFrontmatterEmpty(t *testing.T) {
	entries := ParseFrontmatter("")
	if entries != nil {
		t.Errorf("expected nil for empty frontmatter")
	}
}

