// Package wikilink handles extraction and indexing of [[wikilinks]].
package wikilink

import (
	"regexp"
	"strings"
)

// WikiLink represents a single wikilink found in a document.
type WikiLink struct {
	Target   string // the link target (note name, without fragment)
	Fragment string // optional heading fragment (without #)
	Alias    string // display text if using [[target|alias]] syntax
	Start    int    // byte offset of [[ in the text
	End      int    // byte offset past ]] in the text
}

// wikiLinkRe matches [[target]], [[target#fragment]], [[target|alias]],
// and [[target#fragment|alias]] patterns.
var wikiLinkRe = regexp.MustCompile(`\[\[([^\[\]|#]+?)(?:#([^\[\]|]+?))?(?:\|([^\[\]]+?))?\]\]`)

// Extract finds all wikilinks in the given text.
func Extract(text string) []WikiLink {
	matches := wikiLinkRe.FindAllStringSubmatchIndex(text, -1)
	links := make([]WikiLink, 0, len(matches))

	for _, match := range matches {
		target := strings.TrimSpace(text[match[2]:match[3]])
		if target == "" {
			continue
		}

		wl := WikiLink{
			Target: target,
			Start:  match[0],
			End:    match[1],
		}

		// Group 2: fragment (after #)
		if match[4] != -1 {
			wl.Fragment = strings.TrimSpace(text[match[4]:match[5]])
		}

		// Group 3: alias (after |)
		if match[6] != -1 {
			wl.Alias = strings.TrimSpace(text[match[6]:match[7]])
		}

		links = append(links, wl)
	}

	return links
}


// Targets returns deduplicated target names from the given text.
func Targets(text string) []string {
	links := Extract(text)
	seen := make(map[string]bool, len(links))
	targets := make([]string, 0, len(links))

	for _, l := range links {
		normalized := NormalizeTarget(l.Target)
		if !seen[normalized] {
			seen[normalized] = true
			targets = append(targets, l.Target)
		}
	}

	return targets
}

// NormalizeTarget normalizes a wikilink target for matching.
func NormalizeTarget(target string) string {
	return strings.ToLower(strings.TrimSpace(target))
}

// NormalizeFragment normalizes a heading fragment for comparison.
func NormalizeFragment(fragment string) string {
	return strings.ToLower(strings.TrimSpace(fragment))
}
