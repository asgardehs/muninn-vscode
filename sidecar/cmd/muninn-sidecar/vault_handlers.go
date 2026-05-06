package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sahilm/fuzzy"
	"gopkg.in/yaml.v3"

	"github.com/asgardehs/muninn-sidecar/internal/hierarchy"
	"github.com/asgardehs/muninn-sidecar/internal/lsp"
	"github.com/asgardehs/muninn-sidecar/internal/markdown"
	"github.com/asgardehs/muninn-sidecar/internal/refactor"
	"github.com/asgardehs/muninn-sidecar/internal/refindex"
	"github.com/asgardehs/muninn-sidecar/internal/rpc"
	"github.com/asgardehs/muninn-sidecar/internal/schema"
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
	d.Register("vault/renameNote", handleRenameNote(server))
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
	Name            string         `json:"name"`
	SchemaID        string         `json:"schemaId"`
	Frontmatter     map[string]any `json:"frontmatter"`
	OpenAfterCreate bool           `json:"openAfterCreate"`
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

		body, schemaID, rerr := renderNote(server, p)
		if rerr != nil {
			return nil, rerr
		}
		if err := v.CreateNote(relPath, body); err != nil {
			return nil, vaultErr(err)
		}
		out := map[string]any{
			"name":            p.Name,
			"path":            relPath,
			"absPath":         v.AbsPath(relPath),
			"created":         true,
			"openAfterCreate": p.OpenAfterCreate,
		}
		if schemaID != "" {
			out["schemaId"] = schemaID
		}
		return out, nil
	}
}

// renderNote builds the file body for createFromHierarchy. When a schema
// applies (explicit p.SchemaID, or the highest-priority match if none was
// passed), its template body and frontmatter defaults drive the output.
// Without a schema we fall back to the v0.1 minimal title-only frontmatter.
func renderNote(server *lsp.Server, p createFromHierarchyParams) (string, string, *rpc.Error) {
	title := titleFromHierarchy(p.Name)
	registry := server.Schemas()

	var s *schema.Schema
	if registry != nil {
		if p.SchemaID != "" {
			s = registry.Get(p.SchemaID)
			if s == nil {
				return "", "", &rpc.Error{Code: rpc.CodeNotFound, Message: fmt.Sprintf("schema %q not found", p.SchemaID)}
			}
		} else if matches := registry.ApplicableTo(p.Name); len(matches) > 0 {
			s = matches[0]
		}
	}

	if s == nil {
		body := fmt.Sprintf("---\ntitle: %s\n---\n\n# %s\n\n", title, title)
		return body, "", nil
	}

	vars := schemaTemplateVars(p.Name)
	fm, ferr := buildFrontmatter(s, p.Frontmatter, title, vars)
	if ferr != nil {
		return "", "", &rpc.Error{Code: rpc.CodeSchema, Message: ferr.Error()}
	}
	bodyTmpl := s.Template.Body
	if bodyTmpl == "" {
		bodyTmpl = "# {{name}}\n\n"
	}
	body, terr := schema.Render(bodyTmpl, vars)
	if terr != nil {
		return "", "", &rpc.Error{Code: rpc.CodeSchema, Message: terr.Error()}
	}
	header, herr := renderFrontmatterYAML(s, fm)
	if herr != nil {
		return "", "", &rpc.Error{Code: rpc.CodeSchema, Message: herr.Error()}
	}
	return header + "\n" + body, s.ID, nil
}

// buildFrontmatter merges (in increasing precedence) schema defaults, a
// title fallback, and any caller-supplied overrides; template strings inside
// defaults are expanded.
func buildFrontmatter(s *schema.Schema, override map[string]any, fallbackTitle string, vars schema.TemplateVars) (map[string]any, error) {
	fm := make(map[string]any)
	for k, v := range s.Template.FrontmatterDefaults {
		expanded, err := expandTemplateValue(v, vars)
		if err != nil {
			return nil, fmt.Errorf("default %q: %w", k, err)
		}
		fm[k] = expanded
	}
	if _, hasTitle := fm["title"]; !hasTitle {
		if s.FieldByKey("title") != nil {
			fm["title"] = fallbackTitle
		}
	}
	for k, v := range override {
		fm[k] = v
	}
	return fm, nil
}

// expandTemplateValue recursively expands template strings inside arbitrary
// frontmatter default values (strings, arrays of strings, nested maps).
func expandTemplateValue(v any, vars schema.TemplateVars) (any, error) {
	switch x := v.(type) {
	case string:
		return schema.Render(x, vars)
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			expanded, err := expandTemplateValue(item, vars)
			if err != nil {
				return nil, err
			}
			out[i] = expanded
		}
		return out, nil
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, item := range x {
			expanded, err := expandTemplateValue(item, vars)
			if err != nil {
				return nil, err
			}
			out[k] = expanded
		}
		return out, nil
	default:
		return v, nil
	}
}

// renderFrontmatterYAML produces a YAML frontmatter block for fm, ordering
// schema-declared fields in declaration order and appending any extras
// alphabetically. Wraps the document in --- delimiters.
func renderFrontmatterYAML(s *schema.Schema, fm map[string]any) (string, error) {
	ordered := make([]string, 0, len(fm))
	written := make(map[string]bool, len(fm))
	for _, f := range s.Frontmatter {
		if _, ok := fm[f.Key]; ok {
			ordered = append(ordered, f.Key)
			written[f.Key] = true
		}
	}
	extras := make([]string, 0)
	for k := range fm {
		if !written[k] {
			extras = append(extras, k)
		}
	}
	sort.Strings(extras)
	ordered = append(ordered, extras...)

	out := "---\n"
	for _, k := range ordered {
		raw, err := yaml.Marshal(map[string]any{k: fm[k]})
		if err != nil {
			return "", err
		}
		out += string(raw)
	}
	out += "---\n"
	return out, nil
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
		linkIdx := server.LinkIndex()
		refIdx := server.RefIndex()
		registry := server.Schemas()
		for _, f := range notes {
			content, err := server.Vault().ReadNote(f)
			if err != nil {
				continue
			}
			linkIdx.Update(f, wikilink.Extract(content))

			parsed := markdown.NewParser().Parse(content)
			fmEntries := markdown.ParseFrontmatter(parsed.Frontmatter)
			fmMap := make(map[string]any, len(fmEntries))
			for _, e := range fmEntries {
				fmMap[e.Key] = e.Value
			}
			noteName := strings.TrimSuffix(f, ".md")
			var sch *schema.Schema
			if registry != nil {
				if matches := registry.ApplicableTo(noteName); len(matches) > 0 {
					sch = matches[0]
				}
			}
			refIdx.Update(f, refindex.ExtractEdges(sch, fmMap))
		}
		return map[string]any{"noteCount": len(notes)}, nil
	}
}

// --- vault/renameNote ---

type renameNoteParams struct {
	OldName string `json:"oldName"`
	NewName string `json:"newName"`
}

func handleRenameNote(server *lsp.Server) rpc.Handler {
	return func(_ context.Context, params json.RawMessage) (any, *rpc.Error) {
		var p renameNoteParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &rpc.Error{Code: rpc.CodeInvalidParams, Message: err.Error()}
		}

		v := server.Vault()
		idx := server.LinkIndex()

		plan, err := refactor.BuildPlan(v, idx, p.OldName, p.NewName)
		if err != nil {
			return nil, &rpc.Error{Code: rpc.CodeVault, Message: err.Error()}
		}
		if err := refactor.Apply(v, plan); err != nil {
			return nil, &rpc.Error{Code: rpc.CodeVault, Message: err.Error()}
		}

		// Force the index to catch up immediately so the caller's next query
		// reflects the rename without waiting for fsnotify.
		notes, _ := v.ListNotes()
		for _, f := range notes {
			content, err := v.ReadNote(f)
			if err != nil {
				continue
			}
			idx.Update(f, wikilink.Extract(content))
		}

		return map[string]any{
			"oldName":     plan.OldName,
			"newName":     plan.NewName,
			"renamedFile": plan.RenameTo,
			"editedFiles": len(plan.FileEdits),
		}, nil
	}
}
