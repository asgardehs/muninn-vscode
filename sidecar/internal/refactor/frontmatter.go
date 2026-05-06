package refactor

import (
	"regexp"
	"strings"
)

// RewriteFrontmatterValue replaces fieldKey: oldValue with fieldKey: newValue
// inside the YAML frontmatter block bounded by --- ... --- at the top of text.
// Returns (rewritten, true) if a single matching line was found and rewritten,
// or (text, false) if there was no frontmatter, no matching key, or the key's
// value didn't equal oldValue.
//
// Scope: single-line scalar values only (quoted or unquoted). This matches the
// shape TypeReference produces — list/object reference values are out of scope
// for v0.2.0.
func RewriteFrontmatterValue(text, fieldKey, oldValue, newValue string) (string, bool) {
	if !strings.HasPrefix(text, "---\n") {
		return text, false
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return text, false
	}
	fmBlock := text[4 : 4+end]
	rest := text[4+end:]

	// Match: key:<spaces>value<optional trailing whitespace><newline>
	// Value can be unquoted, "double-quoted", or 'single-quoted'.
	re := regexp.MustCompile(`(?m)^(` + regexp.QuoteMeta(fieldKey) + `:\s*)(?:"([^"]*)"|'([^']*)'|([^\n#]+?))(\s*)$`)
	matches := re.FindAllStringSubmatchIndex(fmBlock, -1)
	if len(matches) == 0 {
		return text, false
	}

	for _, m := range matches {
		// Determine which value group matched.
		var valStart, valEnd int
		var quote string
		switch {
		case m[4] >= 0:
			valStart, valEnd, quote = m[4], m[5], `"`
		case m[6] >= 0:
			valStart, valEnd, quote = m[6], m[7], `'`
		case m[8] >= 0:
			valStart, valEnd, quote = m[8], m[9], ""
		default:
			continue
		}
		got := strings.TrimSpace(fmBlock[valStart:valEnd])
		if got != oldValue {
			continue
		}
		var rewritten strings.Builder
		rewritten.WriteString(text[:4]) // leading "---\n"
		rewritten.WriteString(fmBlock[:valStart])
		if quote != "" {
			// Quote already in source — replace inside the quotes.
			rewritten.WriteString(newValue)
		} else {
			rewritten.WriteString(newValue)
		}
		rewritten.WriteString(fmBlock[valEnd:])
		rewritten.WriteString(rest)
		return rewritten.String(), true
	}
	return text, false
}
