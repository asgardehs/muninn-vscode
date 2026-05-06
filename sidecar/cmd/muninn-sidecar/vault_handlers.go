package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sahilm/fuzzy"

	"github.com/asgardehs/muninn-sidecar/internal/hierarchy"
	"github.com/asgardehs/muninn-sidecar/internal/lsp"
	"github.com/asgardehs/muninn-sidecar/internal/markdown"
	"github.com/asgardehs/muninn-sidecar/internal/rpc"
	"github.com/asgardehs/muninn-sidecar/internal/vault"
	"github.com/asgardehs/muninn-sidecar/internal/wikilink"
)

// registerVaultHandlers attaches the v0.1 vault RPC surface to the dispatcher.
// Each handler builds a fresh hierarchy.Tree from vault.ListNotes() per call.
// For typical PKM vaults (sub-10k notes) this is sub-millisecond; we'll cache
// if profiling shows it bites.
func registerVaultHandlers(d *rpc.Dispatcher, server *lsp.Server) {
	d.Register("vault/lookup", handleLookup(server))
	d.Register("vault/listRoots", handleListRoots(server))
	d.Register("vault/listChildren", handleListChildren(server))
	d.Register("vault/listSiblings", handleListSiblings(server))
	d.Register("vault/getNote", handleGetNote(server))
	d.Register("vault/createFromHierarchy", handleCreateFromHierarchy(server))
	d.Register("vault/refresh", handleRefresh(server))
}

// --- shared helpers ---

func loadTree(v *vault.Vault) (*hierarchy.Tree, error) {
	notes, err := v.ListNotes()
	if err != nil {
		return nil, err
	}
	return hierarchy.Build(notes), nil
}

func nodeShape(n *hierarchy.Node) map[string]any {
	out := map[string]any{
		"name":     n.Name,
		"isStub":   n.IsStub,
		"children": len(n.Children),
	}
	if !n.IsStub {
		out["path"] = n.File
	}
	return out
}

func vaultErr(err error) *rpc.Error {
	return &rpc.Error{Code: rpc.CodeVault, Message: err.Error()}
}

// --- vault/lookup ---

type lookupParams struct {
	Query        string `json:"query"`
	Limit        int    `json:"limit"`
	IncludeStubs bool   `json:"includeStubs"`
}

type lookupMatch struct {
	Name   string `json:"name"`
	Path   string `json:"path,omitempty"`
	Title  string `json:"title,omitempty"`
	Exists bool   `json:"exists"`
	IsStub bool   `json:"isStub"`
	Score  int    `json:"score"`
}

func handleLookup(server *lsp.Server) rpc.Handler {
	return func(_ context.Context, params json.RawMessage) (any, *rpc.Error) {
		var p lookupParams
		if len(params) > 0 {
			if err := json.Unmarshal(params, &p); err != nil {
				return nil, &rpc.Error{Code: rpc.CodeInvalidParams, Message: err.Error()}
			}
		}
		if p.Limit <= 0 {
			p.Limit = 50
		}

		tree, err := loadTree(server.Vault())
		if err != nil {
			return nil, vaultErr(err)
		}
		all := tree.Names()
		if !p.IncludeStubs {
			filtered := make([]string, 0, len(all))
			for _, n := range all {
				if !tree.Get(n).IsStub {
					filtered = append(filtered, n)
				}
			}
			all = filtered
		}

		var ranked []fuzzy.Match
		if strings.TrimSpace(p.Query) == "" {
			ranked = make([]fuzzy.Match, 0, len(all))
			for i, n := range all {
				ranked = append(ranked, fuzzy.Match{Str: n, Index: i, Score: 0})
			}
		} else {
			ranked = fuzzy.Find(p.Query, all)
		}
		if len(ranked) > p.Limit {
			ranked = ranked[:p.Limit]
		}

		matches := make([]lookupMatch, 0, len(ranked))
		for _, m := range ranked {
			n := tree.Get(m.Str)
			match := lookupMatch{
				Name:   n.Name,
				Exists: !n.IsStub,
				IsStub: n.IsStub,
				Score:  m.Score,
			}
			if !n.IsStub {
				match.Path = n.File
				if title := readTitleQuick(server.Vault(), n.File); title != "" {
					match.Title = title
				}
			}
			matches = append(matches, match)
		}
		return map[string]any{"matches": matches}, nil
	}
}

func readTitleQuick(v *vault.Vault, relPath string) string {
	content, err := v.ReadNote(relPath)
	if err != nil {
		return ""
	}
	doc := markdown.NewParser().Parse(content)
	return doc.Title
}

// --- vault/listRoots, listChildren, listSiblings ---

type listChildrenParams struct {
	Name string `json:"name"`
}

func handleListRoots(server *lsp.Server) rpc.Handler {
	return func(_ context.Context, _ json.RawMessage) (any, *rpc.Error) {
		tree, err := loadTree(server.Vault())
		if err != nil {
			return nil, vaultErr(err)
		}
		roots := tree.Roots()
		out := make([]map[string]any, 0, len(roots))
		for _, n := range roots {
			out = append(out, nodeShape(n))
		}
		return map[string]any{"nodes": out}, nil
	}
}

func handleListChildren(server *lsp.Server) rpc.Handler {
	return func(_ context.Context, params json.RawMessage) (any, *rpc.Error) {
		var p listChildrenParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &rpc.Error{Code: rpc.CodeInvalidParams, Message: err.Error()}
		}
		tree, err := loadTree(server.Vault())
		if err != nil {
			return nil, vaultErr(err)
		}
		children := tree.Children(p.Name)
		out := make([]map[string]any, 0, len(children))
		for _, c := range children {
			out = append(out, nodeShape(c))
		}
		return map[string]any{"nodes": out}, nil
	}
}

func handleListSiblings(server *lsp.Server) rpc.Handler {
	return func(_ context.Context, params json.RawMessage) (any, *rpc.Error) {
		var p listChildrenParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &rpc.Error{Code: rpc.CodeInvalidParams, Message: err.Error()}
		}
		tree, err := loadTree(server.Vault())
		if err != nil {
			return nil, vaultErr(err)
		}
		sibs := tree.Siblings(p.Name)
		out := make([]map[string]any, 0, len(sibs))
		for _, s := range sibs {
			out = append(out, nodeShape(s))
		}
		return map[string]any{"nodes": out}, nil
	}
}

// --- vault/getNote ---

type getNoteParams struct {
	Name string `json:"name"`
}

func handleGetNote(server *lsp.Server) rpc.Handler {
	return func(_ context.Context, params json.RawMessage) (any, *rpc.Error) {
		var p getNoteParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &rpc.Error{Code: rpc.CodeInvalidParams, Message: err.Error()}
		}
		tree, err := loadTree(server.Vault())
		if err != nil {
			return nil, vaultErr(err)
		}
		n := tree.Get(p.Name)
		if n == nil || n.IsStub {
			return nil, &rpc.Error{Code: rpc.CodeNotFound, Message: fmt.Sprintf("note %q not found", p.Name)}
		}
		content, err := server.Vault().ReadNote(n.File)
		if err != nil {
			return nil, vaultErr(err)
		}
		parsed := markdown.NewParser().Parse(content)
		fm := markdown.ParseFrontmatter(parsed.Frontmatter)
		fmMap := make(map[string]string, len(fm))
		for _, e := range fm {
			fmMap[e.Key] = e.Value
		}
		headings := markdown.ExtractHeadings(content)
		hs := make([]map[string]any, 0, len(headings))
		for _, h := range headings {
			hs = append(hs, map[string]any{
				"text":  h.Text,
				"level": h.Level,
				"line":  h.Line,
			})
		}
		// Backlinks: who links to this note?
		idx := server.LinkIndex()
		backlinks := idx.Backlinks(wikilink.NormalizeTarget(strings.TrimSuffix(filepath.Base(n.File), ".md")))
		return map[string]any{
			"name":        n.Name,
			"path":        n.File,
			"title":       parsed.Title,
			"frontmatter": fmMap,
			"headings":    hs,
			"backlinks":   backlinks,
		}, nil
	}
}

// --- vault/createFromHierarchy ---

type createFromHierarchyParams struct {
	Name             string `json:"name"`
	OpenAfterCreate  bool   `json:"openAfterCreate"`
}

func handleCreateFromHierarchy(server *lsp.Server) rpc.Handler {
	return func(_ context.Context, params json.RawMessage) (any, *rpc.Error) {
		var p createFromHierarchyParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &rpc.Error{Code: rpc.CodeInvalidParams, Message: err.Error()}
		}
		if strings.TrimSpace(p.Name) == "" {
			return nil, &rpc.Error{Code: rpc.CodeInvalidParams, Message: "name is required"}
		}
		relPath := p.Name + ".md"
		v := server.Vault()
		if v.NoteExists(relPath) {
			return nil, &rpc.Error{Code: rpc.CodeVault, Message: fmt.Sprintf("note %q already exists", p.Name)}
		}

		// v0.1 minimal template: title-only frontmatter + heading.
		// Phase D's schema engine will reintroduce richer templates.
		title := titleFromHierarchy(p.Name)
		body := fmt.Sprintf("---\ntitle: %s\n---\n\n# %s\n\n", title, title)
		if err := v.CreateNote(relPath, body); err != nil {
			return nil, vaultErr(err)
		}
		return map[string]any{
			"name":            p.Name,
			"path":            relPath,
			"absPath":         v.AbsPath(relPath),
			"created":         true,
			"openAfterCreate": p.OpenAfterCreate,
		}, nil
	}
}

// titleFromHierarchy converts "projects.alpha.kickoff" into "Kickoff".
// The leaf segment is title-cased on word boundaries (hyphens -> spaces).
func titleFromHierarchy(name string) string {
	leaf := name
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		leaf = name[idx+1:]
	}
	leaf = strings.ReplaceAll(leaf, "-", " ")
	leaf = strings.ReplaceAll(leaf, "_", " ")
	if leaf == "" {
		return name
	}
	parts := strings.Fields(leaf)
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

// --- vault/refresh ---

func handleRefresh(server *lsp.Server) rpc.Handler {
	return func(_ context.Context, _ json.RawMessage) (any, *rpc.Error) {
		notes, err := server.Vault().ListNotes()
		if err != nil {
			return nil, vaultErr(err)
		}
		idx := server.LinkIndex()
		for _, f := range notes {
			content, err := server.Vault().ReadNote(f)
			if err != nil {
				continue
			}
			idx.Update(f, wikilink.Extract(content))
		}
		return map[string]any{"noteCount": len(notes)}, nil
	}
}
