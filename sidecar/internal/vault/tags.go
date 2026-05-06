package vault

import (
	"sort"
	"strings"

	"github.com/asgardehs/muninn-sidecar/internal/markdown"
)

// CollectTags scans all vault notes and returns a sorted unique list of tags.
func (v *Vault) CollectTags() ([]string, error) {
	files, err := v.ListNotes()
	if err != nil {
		return nil, err
	}

	p := markdown.NewParser()
	seen := make(map[string]bool)

	for _, f := range files {
		content, err := v.ReadNote(f)
		if err != nil {
			continue
		}

		doc := p.Parse(content)
		for _, e := range markdown.ParseFrontmatter(doc.Frontmatter) {
			if strings.EqualFold(e.Key, "tags") {
				seen[e.Value] = true
			}
		}
	}

	tags := make([]string, 0, len(seen))
	for t := range seen {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	return tags, nil
}
