package lsp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"

	"github.com/asgardehs/muninn-sidecar/internal/markdown"
)

// handleDocumentSymbol returns a nested symbol tree of headings for the outline view.
func (s *Server) handleDocumentSymbol(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DocumentSymbolParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	text := s.getDoc(params.TextDocument.URI)
	if text == "" {
		return reply(ctx, nil, nil)
	}

	headings := markdown.ExtractHeadings(text)
	if len(headings) == 0 {
		return reply(ctx, []protocol.DocumentSymbol{}, nil)
	}

	symbols := buildSymbolTree(headings, text)
	return reply(ctx, symbols, nil)
}

// buildSymbolTree constructs a nested DocumentSymbol tree from headings.
// H1 contains H2, H2 contains H3, etc.
func buildSymbolTree(headings []markdown.Heading, text string) []protocol.DocumentSymbol {
	lineCount := uint32(len(strings.Split(text, "\n")))

	// Precompute the end line for each heading's section.
	// A section ends at the next heading of the same or higher level, or EOF.
	endLines := make([]uint32, len(headings))
	for i, h := range headings {
		endLines[i] = lineCount - 1
		for j := i + 1; j < len(headings); j++ {
			if headings[j].Level <= h.Level {
				// End at the line before the next same/higher-level heading.
				if headings[j].Line > 0 {
					endLines[i] = uint32(headings[j].Line - 1)
				} else {
					endLines[i] = 0
				}
				break
			}
		}
	}

	// Build the tree using a stack.
	type entry struct {
		symbol *protocol.DocumentSymbol
		level  int
	}

	var roots []protocol.DocumentSymbol
	var stack []entry

	for i, h := range headings {
		sym := protocol.DocumentSymbol{
			Name: h.Text,
			Kind: protocol.SymbolKindField,
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(h.Line), Character: 0},
				End:   protocol.Position{Line: endLines[i], Character: 0},
			},
			SelectionRange: protocol.Range{
				Start: protocol.Position{Line: uint32(h.Line), Character: 0},
				End:   protocol.Position{Line: uint32(h.Line), Character: 0},
			},
		}

		// Pop stack until we find a parent with lower level.
		for len(stack) > 0 && stack[len(stack)-1].level >= h.Level {
			stack = stack[:len(stack)-1]
		}

		if len(stack) == 0 {
			roots = append(roots, sym)
			stack = append(stack, entry{symbol: &roots[len(roots)-1], level: h.Level})
		} else {
			parent := stack[len(stack)-1].symbol
			parent.Children = append(parent.Children, sym)
			stack = append(stack, entry{
				symbol: &parent.Children[len(parent.Children)-1],
				level:  h.Level,
			})
		}
	}

	return roots
}

// handleWorkspaceSymbol provides fuzzy search across all vault notes and headings.
func (s *Server) handleWorkspaceSymbol(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.WorkspaceSymbolParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	query := strings.ToLower(params.Query)
	const maxResults = 50

	files, err := s.vault.ListNotes()
	if err != nil {
		return reply(ctx, nil, nil)
	}

	var symbols []protocol.SymbolInformation

	for _, f := range files {
		if len(symbols) >= maxResults {
			break
		}

		name := strings.TrimSuffix(filepath.Base(f), ".md")

		// Match note title.
		if query == "" || strings.Contains(strings.ToLower(name), query) {
			symbols = append(symbols, protocol.SymbolInformation{
				Name: name,
				Kind: protocol.SymbolKindFile,
				Location: protocol.Location{
					URI: s.relPathToURI(f),
					Range: protocol.Range{
						Start: protocol.Position{Line: 0, Character: 0},
						End:   protocol.Position{Line: 0, Character: 0},
					},
				},
			})
		}

		// Only scan headings if query is non-empty and at least 2 chars.
		if len(query) < 2 {
			continue
		}

		headings := s.noteHeadings(f)
		for _, h := range headings {
			if len(symbols) >= maxResults {
				break
			}
			if strings.Contains(strings.ToLower(h.Text), query) {
				symbols = append(symbols, protocol.SymbolInformation{
					Name:          h.Text,
					Kind:          protocol.SymbolKindField,
					ContainerName: name,
					Location: protocol.Location{
						URI: s.relPathToURI(f),
						Range: protocol.Range{
							Start: protocol.Position{Line: uint32(h.Line), Character: 0},
							End:   protocol.Position{Line: uint32(h.Line), Character: 0},
						},
					},
				})
			}
		}
	}

	return reply(ctx, symbols, nil)
}
