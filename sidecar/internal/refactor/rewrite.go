package refactor

import (
	"strings"

	"github.com/asgardehs/muninn-sidecar/internal/wikilink"
)

// RewriteWikilinks returns text with every [[oldName]] (and [[oldName#frag]],
// [[oldName|alias]], [[oldName#frag|alias]]) rewritten to use newName.
// Matching on the target portion is case-insensitive so that "Old.Note" and
// "old.note" are both rewritten — this matches the wikilink index's
// NormalizeTarget behavior.
func RewriteWikilinks(text, oldName, newName string) string {
	links := wikilink.Extract(text)
	if len(links) == 0 {
		return text
	}
	oldKey := wikilink.NormalizeTarget(oldName)

	var b strings.Builder
	b.Grow(len(text))
	cursor := 0
	for _, l := range links {
		if wikilink.NormalizeTarget(l.Target) != oldKey {
			continue
		}
		b.WriteString(text[cursor:l.Start])
		b.WriteString("[[")
		b.WriteString(newName)
		if l.Fragment != "" {
			b.WriteByte('#')
			b.WriteString(l.Fragment)
		}
		if l.Alias != "" {
			b.WriteByte('|')
			b.WriteString(l.Alias)
		}
		b.WriteString("]]")
		cursor = l.End
	}
	b.WriteString(text[cursor:])
	return b.String()
}
