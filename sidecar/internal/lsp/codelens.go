package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"

	"github.com/asgardehs/muninn-sidecar/internal/wikilink"
)

func (s *Server) handleCodeLens(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.CodeLensParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	relPath := s.uriToRelPath(params.TextDocument.URI)
	if relPath == "" {
		return reply(ctx, []protocol.CodeLens{}, nil)
	}

	// Count backlinks to this note.
	noteName := strings.TrimSuffix(filepath.Base(relPath), ".md")
	normalized := wikilink.NormalizeTarget(noteName)
	backlinks := s.linkIdx.Backlinks(normalized)
	count := len(backlinks)

	// Build the references title.
	title := fmt.Sprintf("%d references", count)
	if count == 1 {
		title = "1 reference"
	}

	// Build locations for the command arguments.
	var locations []protocol.Location
	for _, bl := range backlinks {
		content, err := s.vault.ReadNote(bl)
		if err != nil {
			continue
		}
		links := wikilink.Extract(content)
		for _, link := range links {
			if wikilink.NormalizeTarget(link.Target) == normalized {
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

	locJSON, _ := json.Marshal(locations)

	lens := protocol.CodeLens{
		Range: protocol.Range{
			Start: protocol.Position{Line: 0, Character: 0},
			End:   protocol.Position{Line: 0, Character: 0},
		},
		Command: &protocol.Command{
			Title:   title,
			Command: "editor.action.showReferences",
			Arguments: []interface{}{
				string(params.TextDocument.URI),
				map[string]int{"line": 0, "character": 0},
				json.RawMessage(locJSON),
			},
		},
	}

	return reply(ctx, []protocol.CodeLens{lens}, nil)
}
