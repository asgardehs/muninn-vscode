package schema

import (
	"strings"
	"testing"
	"time"
)

func TestRender(t *testing.T) {
	now := time.Date(2026, 5, 6, 14, 30, 0, 0, time.UTC)
	vars := TemplateVars{Now: now, User: "adam", Name: "ehs.incidents.2026-05-01"}

	cases := []struct {
		in   string
		want string
	}{
		{"date: {{today}}", "date: 2026-05-06"},
		{"reporter: {{user}}", "reporter: adam"},
		{"id: {{name}}", "id: ehs.incidents.2026-05-01"},
		{"created: {{now}}", "created: 2026-05-06T14:30:00Z"},
		{"no vars here", "no vars here"},
		{"{{ today }} (whitespace)", "2026-05-06 (whitespace)"},
		{"two: {{user}} and {{name}}", "two: adam and ehs.incidents.2026-05-01"},
	}
	for _, tc := range cases {
		got, err := Render(tc.in, vars)
		if err != nil {
			t.Errorf("Render(%q) error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("Render(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRenderUnknownVariableRejected(t *testing.T) {
	_, err := Render("{{nope}}", TemplateVars{Now: time.Now()})
	if err == nil || !strings.Contains(err.Error(), "unknown template variable") {
		t.Errorf("expected unknown-variable error, got %v", err)
	}
}
