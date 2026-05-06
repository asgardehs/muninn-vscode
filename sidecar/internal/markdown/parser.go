// Package markdown provides CommonMark parsing with frontmatter extraction.
package markdown

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// Document represents a parsed markdown document.
type Document struct {
	// Frontmatter is the raw YAML frontmatter string (without delimiters).
	Frontmatter string

	// Body is the markdown content after the frontmatter.
	Body string

	// Title extracted from frontmatter or first heading.
	Title string
}

// Parser wraps a configured goldmark instance.
type Parser struct {
	md goldmark.Markdown
}

// NewParser creates a markdown parser with CommonMark + GFM extensions.
func NewParser() *Parser {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)
	return &Parser{md: md}
}

// Parse splits a markdown document into frontmatter and body,
// and extracts the title.
func (p *Parser) Parse(source string) Document {
	doc := Document{}

	fm, body := extractFrontmatter(source)
	doc.Frontmatter = fm
	doc.Body = body
	doc.Title = extractTitle(fm, body)

	return doc
}

// extractFrontmatter splits YAML frontmatter from markdown body.
// Frontmatter must start at the very beginning of the document with "---".
func extractFrontmatter(source string) (frontmatter, body string) {
	if !strings.HasPrefix(source, "---") {
		return "", source
	}

	rest := source[3:]
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	} else if len(rest) > 1 && rest[0] == '\r' && rest[1] == '\n' {
		rest = rest[2:]
	}

	closeIdx := strings.Index(rest, "\n---")
	if closeIdx == -1 {
		return "", source
	}

	fm := rest[:closeIdx]
	body = rest[closeIdx+4:]

	if len(body) > 0 && body[0] == '\n' {
		body = body[1:]
	} else if len(body) > 1 && body[0] == '\r' && body[1] == '\n' {
		body = body[2:]
	}

	return strings.TrimSpace(fm), body
}

// extractTitle tries to get a title from frontmatter, falling back to
// the first ATX heading in the body.
func extractTitle(frontmatter, body string) string {
	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "title:") {
			val := strings.TrimPrefix(line, "title:")
			val = strings.TrimSpace(val)
			val = strings.Trim(val, `"'`)
			if val != "" {
				return val
			}
		}
	}

	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}

	return ""
}
