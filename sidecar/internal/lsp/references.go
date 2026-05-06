package lsp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"

	"github.com/asgardehs/muninn-sidecar/internal/wikilink"
)

func (s *Server) handleReferences(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.ReferenceParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	// Derive the target name from the current file.
	relPath := s.uriToRelPath(params.TextDocument.URI)
	if relPath == "" {
		return reply(ctx, nil, nil)
	}

	targetName := strings.TrimSuffix(filepath.Base(relPath), ".md")
	normalized := wikilink.NormalizeTarget(targetName)

	// Find all files that link to this target.
	backlinks := s.linkIdx.Backlinks(normalized)

	var locations []protocol.Location
	for _, bl := range backlinks {
		// Find the wikilink position in the linking file.
		content, err := s.vault.ReadNote(bl)
		if err != nil {
			continue
		}

		links := wikilink.Extract(content)
		for _, link := range links {
			if wikilink.NormalizeTarget(link.Target) == normalized {
				// Convert byte offset to line/character.
				line, char := offsetToPosition(content, link.Start)
				endLine, endChar := offsetToPosition(content, link.End)

				locations = append(locations, protocol.Location{
					URI: s.relPathToURI(bl),
					Range: protocol.Range{
						Start: protocol.Position{Line: uint32(line), Character: uint32(char)},
						End:   protocol.Position{Line: uint32(endLine), Character: uint32(endChar)},
					},
				})
			}
		}
	}

	return reply(ctx, locations, nil)
}

// offsetToPosition converts a byte offset in text to a line/character position.
func offsetToPosition(text string, offset int) (line, char int) {
	for i, r := range text {
		if i >= offset {
			return line, char
		}
		if r == '\n' {
			line++
			char = 0
		} else {
			char++
		}
	}
	return line, char
}
