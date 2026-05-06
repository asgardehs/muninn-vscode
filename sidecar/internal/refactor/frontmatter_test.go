package refactor

import "testing"

func TestRewriteFrontmatterValue_SimpleScalar(t *testing.T) {
	in := `---
title: Forklift
instructor: people.john-smith
date: 2026-05-06
---

body
`
	want := `---
title: Forklift
instructor: people.john-doe
date: 2026-05-06
---

body
`
	got, ok := RewriteFrontmatterValue(in, "instructor", "people.john-smith", "people.john-doe")
	if !ok {
		t.Fatal("expected rewrite to succeed")
	}
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestRewriteFrontmatterValue_QuotedScalar(t *testing.T) {
	in := `---
instructor: "people.john-smith"
---
`
	got, ok := RewriteFrontmatterValue(in, "instructor", "people.john-smith", "people.john-doe")
	if !ok {
		t.Fatal("expected rewrite to succeed")
	}
	if got != `---
instructor: "people.john-doe"
---
` {
		t.Errorf("quoted scalar not rewritten cleanly:\n%q", got)
	}
}

func TestRewriteFrontmatterValue_ValueMismatchNoOp(t *testing.T) {
	in := `---
instructor: someone-else
---
`
	got, ok := RewriteFrontmatterValue(in, "instructor", "people.john-smith", "people.john-doe")
	if ok {
		t.Error("expected ok=false when value does not match")
	}
	if got != in {
		t.Error("input should be returned unchanged")
	}
}

func TestRewriteFrontmatterValue_NoFrontmatter(t *testing.T) {
	in := "no frontmatter here\n"
	_, ok := RewriteFrontmatterValue(in, "x", "a", "b")
	if ok {
		t.Error("expected ok=false when no frontmatter")
	}
}
