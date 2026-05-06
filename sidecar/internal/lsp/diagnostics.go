package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"go.lsp.dev/protocol"
	"gopkg.in/yaml.v3"

	"github.com/asgardehs/muninn-sidecar/internal/markdown"
	"github.com/asgardehs/muninn-sidecar/internal/schema"
	"github.com/asgardehs/muninn-sidecar/internal/wikilink"
)

// publishDiagnostics emits broken-wikilink, broken-fragment, YAML-parse, and
// schema-violation diagnostics for the given document.
func (s *Server) publishDiagnostics(ctx context.Context, docURI protocol.DocumentURI, text string) {
	var diagnostics []protocol.Diagnostic

	if configEnabled(s.config.DiagnosticsUnresolved) {
		diagnostics = append(diagnostics, s.brokenLinkDiagnostics(text)...)
	}
	diagnostics = append(diagnostics, s.schemaDiagnostics(docURI, text)...)

	params := protocol.PublishDiagnosticsParams{
		URI:         docURI,
		Diagnostics: diagnostics,
	}
	raw, _ := json.Marshal(params)
	_ = s.conn.Notify(ctx, "textDocument/publishDiagnostics", json.RawMessage(raw))
}

// brokenLinkDiagnostics flags wikilinks that don't resolve to a vault note,
// and heading fragments that don't exist in the resolved target.
func (s *Server) brokenLinkDiagnostics(text string) []protocol.Diagnostic {
	var out []protocol.Diagnostic
	links := wikilink.Extract(text)
	files := s.noteFilenames()

	for _, link := range links {
		normalized := wikilink.NormalizeTarget(link.Target)
		relPath, noteExists := files[normalized]
		if !noteExists {
			line, char := offsetToPosition(text, link.Start)
			endLine, endChar := offsetToPosition(text, link.End)
			out = append(out, protocol.Diagnostic{
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
				out = append(out, protocol.Diagnostic{
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
	return out
}

// schemaDiagnostics runs every applicable schema against the note's
// frontmatter and turns violations into LSP diagnostics. Malformed YAML is
// surfaced as a single error and suppresses schema validation so the user
// isn't flooded with cascading false positives while typing.
func (s *Server) schemaDiagnostics(docURI protocol.DocumentURI, text string) []protocol.Diagnostic {
	registry := s.schemas
	if registry == nil {
		return nil
	}
	relPath := s.uriToRelPath(docURI)
	if relPath == "" || !strings.HasSuffix(relPath, ".md") {
		return nil
	}
	name := strings.TrimSuffix(relPath, ".md")
	matches := registry.ApplicableTo(name)
	if len(matches) == 0 {
		return nil
	}

	doc := markdown.NewParser().Parse(text)
	if doc.Frontmatter == "" {
		// No frontmatter at all. If any applicable schema has required
		// fields, that's a violation; report against line 0.
		var out []protocol.Diagnostic
		for _, sch := range matches {
			for _, v := range schema.Validate(sch, map[string]any{}, s.refResolver()) {
				out = append(out, makeSchemaDiagnostic(v, sch.ID, 0))
			}
		}
		return out
	}

	var fm map[string]any
	if err := yaml.Unmarshal([]byte(doc.Frontmatter), &fm); err != nil {
		return []protocol.Diagnostic{{
			Range: protocol.Range{
				Start: protocol.Position{Line: 0, Character: 0},
				End:   protocol.Position{Line: 0, Character: 0},
			},
			Severity: protocol.DiagnosticSeverityError,
			Source:   "muninn",
			Message:  fmt.Sprintf("invalid frontmatter YAML: %v", err),
		}}
	}
	if fm == nil {
		fm = map[string]any{}
	}

	frontmatterStartLine := 1
	var out []protocol.Diagnostic
	for _, sch := range matches {
		for _, v := range schema.Validate(sch, fm, s.refResolver()) {
			line := findFrontmatterFieldLine(text, v.Field, frontmatterStartLine)
			out = append(out, makeSchemaDiagnostic(v, sch.ID, line))
		}
	}
	return out
}

// findFrontmatterFieldLine scans the raw text for the first line beginning
// with "<field>:" inside the frontmatter block. Returns a 0-based line
// number, or fallback if not found.
func findFrontmatterFieldLine(text, field string, fallback int) int {
	if field == "" {
		return fallback
	}
	lines := strings.Split(text, "\n")
	prefix := field + ":"
	inFM := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			if !inFM {
				inFM = true
				continue
			}
			break
		}
		if !inFM {
			continue
		}
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), prefix) {
			return i
		}
	}
	return fallback
}

func makeSchemaDiagnostic(v schema.Violation, schemaID string, line int) protocol.Diagnostic {
	severity := protocol.DiagnosticSeverityWarning
	switch v.Severity {
	case schema.SeverityError:
		severity = protocol.DiagnosticSeverityError
	case schema.SeverityInfo:
		severity = protocol.DiagnosticSeverityInformation
	}
	return protocol.Diagnostic{
		Range: protocol.Range{
			Start: protocol.Position{Line: uint32(line), Character: 0},
			End:   protocol.Position{Line: uint32(line), Character: 0},
		},
		Severity: severity,
		Source:   "muninn-schema",
		Code:     fmt.Sprintf("%s/%s", schemaID, v.Code),
		Message:  v.Message,
	}
}

// schemaRefResolver adapts the LSP server's wikilink/vault state to
// schema.NoteRefResolver. Note-ref fields contain hierarchy names; we resolve
// by checking whether {name}.md exists in the vault.
type schemaRefResolver struct{ s *Server }

func (r schemaRefResolver) NoteExists(name string) bool {
	return r.s.vault.NoteExists(filepath.Clean(name + ".md"))
}

func (s *Server) refResolver() schema.NoteRefResolver { return schemaRefResolver{s: s} }

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
