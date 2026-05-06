package schema

import (
	"testing"
)

func mustLoad(t *testing.T) *Registry {
	t.Helper()
	r, err := loadFromFS(builtinFS, "builtin/generic")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return r
}

func TestValidateMissingRequired(t *testing.T) {
	r := mustLoad(t)
	decision := r.Get("decision")
	violations := Validate(decision, map[string]any{}, nil)
	if len(violations) == 0 {
		t.Fatal("expected violations for empty frontmatter")
	}
	keys := map[string]bool{}
	for _, v := range violations {
		if v.Code == "missing-required" {
			keys[v.Field] = true
		}
	}
	for _, want := range []string{"title", "date", "status"} {
		if !keys[want] {
			t.Errorf("missing-required for %q not surfaced", want)
		}
	}
}

func TestValidateInvalidEnum(t *testing.T) {
	r := mustLoad(t)
	violations := Validate(r.Get("decision"), map[string]any{
		"title":  "test",
		"date":   "2026-05-06",
		"status": "in-flight",
	}, nil)
	found := false
	for _, v := range violations {
		if v.Code == "invalid-enum-value" && v.Field == "status" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected invalid-enum-value for status, got %+v", violations)
	}
}

func TestValidateAcceptsValidFrontmatter(t *testing.T) {
	r := mustLoad(t)
	violations := Validate(r.Get("decision"), map[string]any{
		"title":  "Pick API versioning scheme",
		"date":   "2026-05-06",
		"status": "accepted",
	}, nil)
	if len(violations) != 0 {
		t.Errorf("expected zero violations, got %+v", violations)
	}
}

func TestValidateDateFormat(t *testing.T) {
	r := mustLoad(t)
	violations := Validate(r.Get("decision"), map[string]any{
		"title":  "test",
		"date":   "May 6, 2026",
		"status": "proposed",
	}, nil)
	found := false
	for _, v := range violations {
		if v.Field == "date" && v.Code == "type-mismatch" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected type-mismatch for malformed date, got %+v", violations)
	}
}

type fakeRefs struct{ exists map[string]bool }

func (f fakeRefs) NoteExists(target string) bool { return f.exists[target] }

func TestValidateNoteRef(t *testing.T) {
	s := &Schema{
		ID:      "issue",
		Pattern: "issues.*",
		Frontmatter: []Field{
			{Key: "parent", Type: TypeNoteRef, Required: false},
		},
	}
	refs := fakeRefs{exists: map[string]bool{"projects.alpha": true}}

	// Resolved.
	if v := Validate(s, map[string]any{"parent": "projects.alpha"}, refs); len(v) != 0 {
		t.Errorf("resolved ref produced violations: %+v", v)
	}
	// Unresolved.
	v := Validate(s, map[string]any{"parent": "projects.gone"}, refs)
	if len(v) == 0 || v[0].Code != "note-ref-unresolved" {
		t.Errorf("expected note-ref-unresolved, got %+v", v)
	}
}
