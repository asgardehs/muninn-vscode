package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go.lsp.dev/protocol"

	"github.com/asgardehs/muninn-sidecar/internal/wikilink"
)

// publishDiagnostics emits broken-wikilink and broken-fragment diagnostics for
// the given document. Phase D will reintroduce schema-violation diagnostics
// once the schema engine lands; for v0.1 the only check is link resolution.
func (s *Server) publishDiagnostics(ctx context.Context, docURI protocol.DocumentURI, text string) {
	var diagnostics []protocol.Diagnostic

	if configEnabled(s.config.DiagnosticsUnresolved) {
		links := wikilink.Extract(text)
		files := s.noteFilenames()

		for _, link := range links {
			normalized := wikilink.NormalizeTarget(link.Target)
			relPath, noteExists := files[normalized]
			if !noteExists {
				line, char := offsetToPosition(text, link.Start)
				endLine, endChar := offsetToPosition(text, link.End)
				diagnostics = append(diagnostics, protocol.Diagnostic{
					Range: protocol.Range{
						Start: protocol.Position{Line: uint32(line), Character: uint32(char)},
						End:   protocol.Position{Line: uint32(endLine), Character: uint32(endChar)},
					},
					Severity: protocol.DiagnosticSeverityWarning,
					Source:   "muninn",
					Message:  fmt.Sprintf("broken link: [[%s]] does not resolve to any note", link.Target),
				})
				continue
			}

			if link.Fragment != "" {
				headings := s.noteHeadings(relPath)
				normFrag := wikilink.NormalizeFragment(link.Fragment)
				found := false
				for _, h := range headings {
					if strings.ToLower(h.Text) == normFrag {
						found = true
						break
					}
				}
				if !found {
					line, char := offsetToPosition(text, link.Start)
					endLine, endChar := offsetToPosition(text, link.End)
					diagnostics = append(diagnostics, protocol.Diagnostic{
						Range: protocol.Range{
							Start: protocol.Position{Line: uint32(line), Character: uint32(char)},
							End:   protocol.Position{Line: uint32(endLine), Character: uint32(endChar)},
						},
						Severity: protocol.DiagnosticSeverityWarning,
						Source:   "muninn",
						Message:  fmt.Sprintf("broken fragment: heading %q not found in [[%s]]", link.Fragment, link.Target),
					})
				}
			}
		}
	}

	params := protocol.PublishDiagnosticsParams{
		URI:         docURI,
		Diagnostics: diagnostics,
	}
	raw, _ := json.Marshal(params)
	_ = s.conn.Notify(ctx, "textDocument/publishDiagnostics", json.RawMessage(raw))
}

// refreshAllDiagnostics resends diagnostics for every open document. Used by
// the sidecar when the vault tree changes (fsnotify) so opened docs reflect
// new note targets immediately.
func (s *Server) refreshAllDiagnostics(ctx context.Context) {
	s.mu.RLock()
	docs := make(map[protocol.DocumentURI]string, len(s.docs))
	for k, v := range s.docs {
		docs[k] = v
	}
	s.mu.RUnlock()

	for docURI, text := range docs {
		if strings.HasSuffix(string(docURI), ".md") {
			s.publishDiagnostics(ctx, docURI, text)
		}
	}
}
