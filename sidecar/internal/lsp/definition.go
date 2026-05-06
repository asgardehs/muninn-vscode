package lsp

import (
	"context"
	"encoding/json"
	"strings"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"

	"github.com/asgardehs/muninn-sidecar/internal/wikilink"
)

func (s *Server) handleDefinition(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DefinitionParams
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
		return reply(ctx, nil, nil)
	}

	// Default to top of file; if fragment present, jump to that heading.
	startLine := uint32(0)
	if link.Fragment != "" {
		headings := s.noteHeadings(relPath)
		normFrag := wikilink.NormalizeFragment(link.Fragment)
		for _, h := range headings {
			if strings.ToLower(h.Text) == normFrag {
				startLine = uint32(h.Line)
				break
			}
		}
	}

	return reply(ctx, protocol.Location{
		URI: s.relPathToURI(relPath),
		Range: protocol.Range{
			Start: protocol.Position{Line: startLine, Character: 0},
			End:   protocol.Position{Line: startLine, Character: 0},
		},
	}, nil)
}
