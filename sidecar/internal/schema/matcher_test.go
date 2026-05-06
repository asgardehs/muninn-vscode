package schema

import "testing"

func TestMatchPattern(t *testing.T) {
	cases := []struct {
		pattern string
		name    string
		want    bool
	}{
		// Literal exact
		{"foo", "foo", true},
		{"foo", "bar", false},
		{"foo.bar", "foo.bar", true},

		// Single-segment *
		{"daily.*", "daily.2026-05-06", true},
		{"daily.*", "daily", false},
		{"daily.*", "daily.a.b", false},
		{"ehs.incidents.*", "ehs.incidents.2026-05-01", true},
		{"ehs.incidents.*", "ehs.incidents", false},
		{"ehs.incidents.*", "ehs.incidents.x.y", false},

		// Recursive **
		{"projects.**", "projects.alpha", true},
		{"projects.**", "projects.alpha.kickoff", true},
		{"projects.**", "projects", false},
		{"**.kickoff", "alpha.kickoff", true},
		{"**.kickoff", "x.y.z.kickoff", true},
		{"**.kickoff", "kickoff", false},

		// Mixed
		{"a.*.c", "a.b.c", true},
		{"a.*.c", "a.b.d", false},
		{"a.**.c", "a.b.c", true},
		{"a.**.c", "a.b.x.c", true},
		{"a.**.c", "a.c", false},

		// Empty pattern matches empty name only
		{"", "", true},
		{"", "anything", false},
	}
	for _, tc := range cases {
		if got := MatchPattern(tc.pattern, tc.name); got != tc.want {
			t.Errorf("MatchPattern(%q, %q) = %v, want %v", tc.pattern, tc.name, got, tc.want)
		}
	}
}
