package schema

import "strings"

// MatchPattern reports whether a glob-over-dot-path pattern matches a
// hierarchy name.
//
// Pattern syntax:
//   - Literal segments must match exactly (case-sensitive).
//   - "*" matches exactly one segment.
//   - "**" matches one or more segments.
//
// Examples:
//   "ehs.incidents.*"    matches  "ehs.incidents.2026-05-01"
//                        does NOT match "ehs.incidents" or "ehs.incidents.a.b"
//   "projects.**"        matches  "projects.alpha", "projects.alpha.kickoff"
//                        does NOT match "projects" alone
//   "daily.*"            matches  "daily.2026-05-06"
func MatchPattern(pattern, name string) bool {
	pSegs := splitSegments(pattern)
	nSegs := splitSegments(name)
	return matchSegments(pSegs, nSegs)
}

func splitSegments(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ".")
}

func matchSegments(pat, seg []string) bool {
	for len(pat) > 0 {
		switch pat[0] {
		case "**":
			if len(seg) == 0 {
				return false
			}
			// "**" consumes 1..N segments, then the remaining pattern must
			// match. Try every split.
			for i := 1; i <= len(seg); i++ {
				if matchSegments(pat[1:], seg[i:]) {
					return true
				}
			}
			return false
		case "*":
			if len(seg) == 0 {
				return false
			}
			pat, seg = pat[1:], seg[1:]
		default:
			if len(seg) == 0 || pat[0] != seg[0] {
				return false
			}
			pat, seg = pat[1:], seg[1:]
		}
	}
	return len(seg) == 0
}
