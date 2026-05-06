package markdown

import "strings"

// Heading represents a markdown ATX heading with position information.
type Heading struct {
	Text       string // heading text without # prefix
	Level      int    // 1-6
	Line       int    // 0-based line number in the document
	ByteOffset int    // byte offset of the '#' character in the full text
}

// ParseHeadingLine checks if a line is an ATX heading.
// Returns the heading text, level, and whether it matched.
func ParseHeadingLine(line string) (text string, level int, ok bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "#") {
		return "", 0, false
	}

	for _, r := range trimmed {
		if r == '#' {
			level++
		} else {
			break
		}
	}

	if level < 1 || level > 6 {
		return "", 0, false
	}

	// Require space after # marks.
	if len(trimmed) > level && trimmed[level] != ' ' {
		return "", 0, false
	}

	text = strings.TrimSpace(trimmed[level:])
	if text == "" {
		return "", 0, false
	}

	return text, level, true
}

// ExtractHeadings scans markdown text and returns all ATX headings
// with their positions. Headings inside fenced code blocks are skipped.
func ExtractHeadings(text string) []Heading {
	lines := strings.Split(text, "\n")
	var headings []Heading
	inFence := false
	offset := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			offset += len(line) + 1
			continue
		}

		if !inFence {
			if hText, level, ok := ParseHeadingLine(line); ok {
				headings = append(headings, Heading{
					Text:       hText,
					Level:      level,
					Line:       i,
					ByteOffset: offset,
				})
			}
		}

		offset += len(line) + 1
	}

	return headings
}
