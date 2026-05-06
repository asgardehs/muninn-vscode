package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"

	"github.com/asgardehs/muninn-sidecar/internal/markdown"
	"github.com/asgardehs/muninn-sidecar/internal/wikilink"
)

func (s *Server) handleHover(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.HoverParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	text := s.getDoc(params.TextDocument.URI)
	if text == "" {
		return reply(ctx, nil, nil)
	}

	link := findWikilinkAt(text, params.Position.Line, params.Position.Character)
	if link == nil {
		return reply(ctx, nil, nil)
	}

	relPath := s.resolveTarget(link.Target)
	if relPath == "" {
		return reply(ctx, &protocol.Hover{
			Contents: protocol.MarkupContent{
				Kind:  protocol.Markdown,
				Value: fmt.Sprintf("**%s** — *not found*", link.Target),
			},
		}, nil)
	}

	// Read the target note and show a preview.
	content, err := s.vault.ReadNote(relPath)
	if err != nil {
		return reply(ctx, nil, nil)
	}

	var preview string
	if link.Fragment != "" {
		preview = notePreviewFromHeading(content, link.Target, link.Fragment)
	} else {
		preview = notePreview(content, link.Target)
	}

	return reply(ctx, &protocol.Hover{
		Contents: protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: preview,
		},
	}, nil)
}

// notePreview returns a short markdown preview of a note.
func notePreview(content, title string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**%s**\n\n", title))

	// Skip frontmatter and show first few lines of body.
	body := content
	if strings.HasPrefix(content, "---") {
		rest := content[3:]
		if idx := strings.Index(rest, "\n---"); idx != -1 {
			body = strings.TrimSpace(rest[idx+4:])
		}
	}

	lines := strings.Split(body, "\n")
	shown := 0
	for _, line := range lines {
		if shown >= 5 {
			sb.WriteString("\n...")
			break
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" && shown == 0 {
			continue
		}
		sb.WriteString(trimmed)
		sb.WriteString("\n")
		shown++
	}

	return sb.String()
}

// notePreviewFromHeading returns a preview starting at the named heading.
func notePreviewFromHeading(content, title, fragment string) string {
	headings := markdown.ExtractHeadings(content)
	normFrag := wikilink.NormalizeFragment(fragment)

	// Find the matching heading.
	var matchIdx int = -1
	var matchLevel int
	for i, h := range headings {
		if strings.ToLower(h.Text) == normFrag {
			matchIdx = i
			matchLevel = h.Level
			break
		}
	}

	if matchIdx == -1 {
		// Heading not found, fall back to standard preview.
		return notePreview(content, title)
	}

	lines := strings.Split(content, "\n")
	startLine := headings[matchIdx].Line

	// Determine end: next heading of same or higher level, or EOF.
	endLine := len(lines)
	for i := matchIdx + 1; i < len(headings); i++ {
		if headings[i].Level <= matchLevel {
			endLine = headings[i].Line
			break
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**%s** > **%s**\n\n", title, fragment))

	shown := 0
	for i := startLine; i < endLine && shown < 5; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" && shown == 0 {
			continue
		}
		sb.WriteString(trimmed)
		sb.WriteString("\n")
		shown++
	}

	if shown >= 5 {
		sb.WriteString("...")
	}

	return sb.String()
}
