// Package lsp implements the Muninn LSP server for knowledge base navigation.
package lsp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asgardehs/muninn-sidecar/internal/markdown"
	"github.com/asgardehs/muninn-sidecar/internal/refindex"
	"github.com/asgardehs/muninn-sidecar/internal/schema"
	"github.com/asgardehs/muninn-sidecar/internal/vault"
	"github.com/asgardehs/muninn-sidecar/internal/wikilink"
)

// muninnConfig holds user-configurable settings passed from the VS Code extension.
type muninnConfig struct {
	CodeLensEnabled       *bool `json:"codeLens.enabled"`
	DiagnosticsUnresolved *bool `json:"diagnostics.unresolvedLinks"`
	SemanticTokensEnabled *bool `json:"semanticTokens.enabled"`
}

// configEnabled returns the bool value of a *bool config field, defaulting to true.
func configEnabled(b *bool) bool {
	if b == nil {
		return true
	}
	return *b
}

// Server is the Muninn LSP server.
type Server struct {
	vault   *vault.Vault
	schemas *schema.Registry
	conn    jsonrpc2.Conn

	config muninnConfig

	mu   sync.RWMutex
	docs map[protocol.DocumentURI]string

	linkIdx  *wikilink.Index
	refIdx   *refindex.Index
}

// New creates a new LSP server bound to the given vault.
func New(v *vault.Vault) *Server {
	return &Server{
		vault:   v,
		docs:    make(map[protocol.DocumentURI]string),
		linkIdx: wikilink.NewIndex(),
		refIdx:  refindex.NewIndex(),
	}
}

// LinkIndex exposes the wikilink index so the sidecar can refresh it from
// filesystem watcher events that fire outside of LSP didSave.
func (s *Server) LinkIndex() *wikilink.Index { return s.linkIdx }

// RefIndex exposes the reference index so the sidecar can refresh it alongside
// the wikilink index on filesystem watcher events and vault/refresh calls.
func (s *Server) RefIndex() *refindex.Index { return s.refIdx }

// Vault exposes the underlying vault for sidecar coordination (e.g., the
// fsnotify watcher needs to read changed files to update the index).
func (s *Server) Vault() *vault.Vault { return s.vault }

// Schemas returns the schema registry, or nil if none has been set. RPC and
// LSP handlers should nil-check before use; absence means no schemas were
// loaded (vault has no .muninn/schemas/ and embedded fallback failed).
func (s *Server) Schemas() *schema.Registry { return s.schemas }

// SetSchemas attaches a schema registry. Called at sidecar startup once the
// registry is loaded; safe to call again to swap registries when the user
// reloads schemas.
func (s *Server) SetSchemas(r *schema.Registry) { s.schemas = r }

// RefreshOpenDiagnostics resends diagnostics for every currently-open
// document. The sidecar's filesystem watcher calls this after the wikilink
// index changes so previously-broken links resolve immediately in open
// editors without waiting for the next keystroke.
func (s *Server) RefreshOpenDiagnostics(ctx context.Context) {
	s.refreshAllDiagnostics(ctx)
}

// ServeOn drives the LSP server using the supplied jsonrpc2 stream. The stream
// is provided by the sidecar's transport multiplexer (the "lsp" channel of the
// shared stdio pipe). Returns when the connection is closed.
func (s *Server) ServeOn(ctx context.Context, stream jsonrpc2.Stream) error {
	s.conn = jsonrpc2.NewConn(stream)
	s.conn.Go(ctx, s.handle)
	<-s.conn.Done()
	return nil
}

// handle dispatches JSON-RPC requests to the appropriate handler.
func (s *Server) handle(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	switch req.Method() {
	case "initialize":
		return s.handleInitialize(ctx, reply, req)
	case "initialized":
		return reply(ctx, nil, nil)
	case "shutdown":
		return reply(ctx, nil, nil)
	case "exit":
		// Phase B note: do NOT call os.Exit here. The sidecar runs the LSP
		// server alongside a separate JSON-RPC channel; killing the process
		// from within an LSP handler would also abort RPC. Instead the parent
		// (extension) closes its stdin, the mux observes EOF, and the whole
		// runtime shuts down cleanly.
		return reply(ctx, nil, nil)
	case "textDocument/didOpen":
		return s.handleDidOpen(ctx, reply, req)
	case "textDocument/didChange":
		return s.handleDidChange(ctx, reply, req)
	case "textDocument/didSave":
		return s.handleDidSave(ctx, reply, req)
	case "textDocument/didClose":
		return s.handleDidClose(ctx, reply, req)
	case "textDocument/completion":
		return s.handleCompletion(ctx, reply, req)
	case "textDocument/definition":
		return s.handleDefinition(ctx, reply, req)
	case "textDocument/references":
		return s.handleReferences(ctx, reply, req)
	case "textDocument/hover":
		return s.handleHover(ctx, reply, req)
	case "textDocument/documentSymbol":
		return s.handleDocumentSymbol(ctx, reply, req)
	case "workspace/symbol":
		return s.handleWorkspaceSymbol(ctx, reply, req)
	case "textDocument/semanticTokens/full":
		if !configEnabled(s.config.SemanticTokensEnabled) {
			return reply(ctx, &protocol.SemanticTokens{}, nil)
		}
		return s.handleSemanticTokensFull(ctx, reply, req)
	case "textDocument/codeLens":
		if !configEnabled(s.config.CodeLensEnabled) {
			return reply(ctx, []protocol.CodeLens{}, nil)
		}
		return s.handleCodeLens(ctx, reply, req)
	case "textDocument/codeAction":
		return s.handleCodeAction(ctx, reply, req)
	default:
		return reply(ctx, nil, jsonrpc2.ErrMethodNotFound)
	}
}

// handleInitialize responds with server capabilities.
func (s *Server) handleInitialize(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var initParams struct {
		InitializationOptions muninnConfig `json:"initializationOptions"`
	}
	if err := json.Unmarshal(req.Params(), &initParams); err == nil {
		s.config = initParams.InitializationOptions
	}

	s.buildLinkIndex()

	result := protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			TextDocumentSync: protocol.TextDocumentSyncOptions{
				OpenClose: true,
				Change:    protocol.TextDocumentSyncKindFull,
				Save: &protocol.SaveOptions{
					IncludeText: true,
				},
			},
			CompletionProvider: &protocol.CompletionOptions{
				TriggerCharacters: []string{"[", "#"},
			},
			DefinitionProvider:      true,
			ReferencesProvider:      true,
			HoverProvider:           true,
			DocumentSymbolProvider:  true,
			WorkspaceSymbolProvider: true,
			SemanticTokensProvider: semanticTokensProviderOptions{
				Legend: protocol.SemanticTokensLegend{
					TokenTypes:     semanticTokenTypes,
					TokenModifiers: semanticTokenModifiers,
				},
				Full: true,
			},
			CodeLensProvider: &protocol.CodeLensOptions{},
			CodeActionProvider: &protocol.CodeActionOptions{
				CodeActionKinds: []protocol.CodeActionKind{protocol.QuickFix},
			},
			// RenameProvider and ExecuteCommandProvider intentionally omitted.
			// Cross-vault rename lives in the RPC channel (vault/renameNote, v0.2),
			// not in LSP rename. ExecuteCommand has no commands to advertise.
		},
		ServerInfo: &protocol.ServerInfo{
			Name:    "muninn",
			Version: "0.2.0",
		},
	}
	return reply(ctx, result, nil)
}

// buildLinkIndex loads all notes and populates the wikilink and reference indexes.
func (s *Server) buildLinkIndex() {
	files, err := s.vault.ListNotes()
	if err != nil {
		return
	}
	for _, f := range files {
		content, err := s.vault.ReadNote(f)
		if err != nil {
			continue
		}
		links := wikilink.Extract(content)
		s.linkIdx.Update(f, links)

		parsed := markdown.NewParser().Parse(content)
		fmEntries := markdown.ParseFrontmatter(parsed.Frontmatter)
		fmMap := make(map[string]any, len(fmEntries))
		for _, e := range fmEntries {
			fmMap[e.Key] = e.Value
		}
		noteName := strings.TrimSuffix(f, ".md")
		var sch *schema.Schema
		if s.schemas != nil {
			if matches := s.schemas.ApplicableTo(noteName); len(matches) > 0 {
				sch = matches[0]
			}
		}
		s.refIdx.Update(f, refindex.ExtractEdges(sch, fmMap))
	}
}

// BuildLinkIndex is the eager public entry point. The sidecar calls this at
// startup so that RPC consumers (e.g., vault/getNote backlinks) see a populated
// index even before an LSP client connects and triggers initialize.
func (s *Server) BuildLinkIndex() { s.buildLinkIndex() }

// --- Document sync ---

func (s *Server) handleDidOpen(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidOpenTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}
	s.mu.Lock()
	s.docs[params.TextDocument.URI] = params.TextDocument.Text
	s.mu.Unlock()
	s.publishDiagnostics(ctx, params.TextDocument.URI, params.TextDocument.Text)
	return reply(ctx, nil, nil)
}

func (s *Server) handleDidChange(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidChangeTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}
	if len(params.ContentChanges) > 0 {
		text := params.ContentChanges[len(params.ContentChanges)-1].Text
		s.mu.Lock()
		s.docs[params.TextDocument.URI] = text
		s.mu.Unlock()
		s.publishDiagnostics(ctx, params.TextDocument.URI, text)
	}
	return reply(ctx, nil, nil)
}

func (s *Server) handleDidSave(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidSaveTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}
	relPath := s.uriToRelPath(params.TextDocument.URI)
	if relPath != "" {
		text := params.Text
		if text == "" {
			text, _ = s.vault.ReadNote(relPath)
		}
		if text != "" {
			links := wikilink.Extract(text)
			s.linkIdx.Update(relPath, links)
		}
	}
	return reply(ctx, nil, nil)
}

func (s *Server) handleDidClose(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidCloseTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}
	s.mu.Lock()
	delete(s.docs, params.TextDocument.URI)
	s.mu.Unlock()
	return reply(ctx, nil, nil)
}

// --- Helpers ---

func (s *Server) getDoc(docURI protocol.DocumentURI) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.docs[docURI]
}

func (s *Server) uriToRelPath(docURI protocol.DocumentURI) string {
	path := uri.URI(docURI).Filename()
	rel, err := filepath.Rel(s.vault.Root(), path)
	if err != nil {
		return ""
	}
	return rel
}

func (s *Server) relPathToURI(relPath string) protocol.DocumentURI {
	abs := filepath.Join(s.vault.Root(), relPath)
	return protocol.DocumentURI(uri.File(abs))
}

func (s *Server) noteFilenames() map[string]string {
	files, err := s.vault.ListNotes()
	if err != nil {
		return nil
	}
	m := make(map[string]string, len(files))
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".md")
		m[wikilink.NormalizeTarget(name)] = f
	}
	return m
}

func (s *Server) resolveTarget(target string) string {
	normalized := wikilink.NormalizeTarget(target)
	files := s.noteFilenames()
	if relPath, ok := files[normalized]; ok {
		return relPath
	}
	return ""
}

func (s *Server) noteHeadings(relPath string) []markdown.Heading {
	docURI := s.relPathToURI(relPath)
	if text := s.getDoc(docURI); text != "" {
		return markdown.ExtractHeadings(text)
	}
	content, err := s.vault.ReadNote(relPath)
	if err != nil {
		return nil
	}
	return markdown.ExtractHeadings(content)
}

func findWikilinkAt(text string, line, char uint32) *wikilink.WikiLink {
	lines := strings.Split(text, "\n")
	if int(line) >= len(lines) {
		return nil
	}
	lineText := lines[line]
	links := wikilink.Extract(lineText)
	for _, l := range links {
		if int(char) >= l.Start && int(char) <= l.End {
			return &l
		}
	}
	return nil
}
