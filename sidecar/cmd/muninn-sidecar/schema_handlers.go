package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/asgardehs/muninn-sidecar/internal/lsp"
	"github.com/asgardehs/muninn-sidecar/internal/markdown"
	"github.com/asgardehs/muninn-sidecar/internal/rpc"
	"github.com/asgardehs/muninn-sidecar/internal/schema"
	"github.com/asgardehs/muninn-sidecar/internal/vault"
)

// vaultRefResolver adapts vault.Vault to schema.NoteRefResolver. Note-ref
// fields carry hierarchy names ("foo.bar"); existence is checked against
// the .md file at that name.
type vaultRefResolver struct{ v *vault.Vault }

func (r vaultRefResolver) NoteExists(name string) bool {
	return r.v.NoteExists(name + ".md")
}

func registerSchemaHandlers(d *rpc.Dispatcher, server *lsp.Server) {
	d.Register("schema/list", handleSchemaList(server))
	d.Register("schema/get", handleSchemaGet(server))
	d.Register("schema/applicableTo", handleSchemaApplicable(server))
	d.Register("schema/validate", handleSchemaValidate(server))
	d.Register("schema/listPacks", handleSchemaListPacks())
	d.Register("schema/exportPack", handleSchemaExportPack())
	d.Register("schema/reload", handleSchemaReload(server))
}

// schema/reload re-scans <vault>/.muninn/schemas/ (falling back to the
// embedded generic pack) and replaces the active registry. Used by the
// extension after installSchemaPack writes new YAML to disk.
func handleSchemaReload(server *lsp.Server) rpc.Handler {
	return func(_ context.Context, _ json.RawMessage) (any, *rpc.Error) {
		registry, err := schema.Load(server.Vault().Root())
		if err != nil {
			return nil, &rpc.Error{Code: rpc.CodeSchema, Message: err.Error()}
		}
		server.SetSchemas(registry)
		return map[string]any{"schemaCount": registry.Len()}, nil
	}
}

func requireRegistry(server *lsp.Server) (*schema.Registry, *rpc.Error) {
	r := server.Schemas()
	if r == nil {
		return nil, &rpc.Error{Code: rpc.CodeSchema, Message: "schema registry not loaded"}
	}
	return r, nil
}

func schemaShape(s *schema.Schema) map[string]any {
	return map[string]any{
		"id":          s.ID,
		"label":       s.Label,
		"description": s.Description,
		"pattern":     s.Pattern,
		"priority":    s.Priority,
	}
}

// schema/list

func handleSchemaList(server *lsp.Server) rpc.Handler {
	return func(_ context.Context, _ json.RawMessage) (any, *rpc.Error) {
		r, err := requireRegistry(server)
		if err != nil {
			return nil, err
		}
		all := r.All()
		out := make([]map[string]any, 0, len(all))
		for _, s := range all {
			out = append(out, schemaShape(s))
		}
		return map[string]any{"schemas": out}, nil
	}
}

// schema/get

type schemaGetParams struct {
	ID string `json:"id"`
}

func handleSchemaGet(server *lsp.Server) rpc.Handler {
	return func(_ context.Context, params json.RawMessage) (any, *rpc.Error) {
		r, err := requireRegistry(server)
		if err != nil {
			return nil, err
		}
		var p schemaGetParams
		if uerr := json.Unmarshal(params, &p); uerr != nil {
			return nil, &rpc.Error{Code: rpc.CodeInvalidParams, Message: uerr.Error()}
		}
		s := r.Get(p.ID)
		if s == nil {
			return nil, &rpc.Error{Code: rpc.CodeNotFound, Message: fmt.Sprintf("schema %q not found", p.ID)}
		}
		fields := make([]map[string]any, len(s.Frontmatter))
		for i, f := range s.Frontmatter {
			fields[i] = map[string]any{
				"key":         f.Key,
				"type":        string(f.Type),
				"required":    f.Required,
				"vocabulary":  f.Vocabulary,
				"description": f.Description,
			}
		}
		out := schemaShape(s)
		out["fields"] = fields
		out["templateBody"] = s.Template.Body
		return out, nil
	}
}

// schema/applicableTo

type schemaApplicableParams struct {
	Name string `json:"name"`
}

func handleSchemaApplicable(server *lsp.Server) rpc.Handler {
	return func(_ context.Context, params json.RawMessage) (any, *rpc.Error) {
		r, err := requireRegistry(server)
		if err != nil {
			return nil, err
		}
		var p schemaApplicableParams
		if uerr := json.Unmarshal(params, &p); uerr != nil {
			return nil, &rpc.Error{Code: rpc.CodeInvalidParams, Message: uerr.Error()}
		}
		matches := r.ApplicableTo(p.Name)
		out := make([]map[string]any, 0, len(matches))
		for _, s := range matches {
			out = append(out, schemaShape(s))
		}
		return map[string]any{"schemas": out}, nil
	}
}

// schema/validate

type schemaValidateParams struct {
	Path string `json:"path"`
}

func handleSchemaValidate(server *lsp.Server) rpc.Handler {
	return func(_ context.Context, params json.RawMessage) (any, *rpc.Error) {
		r, err := requireRegistry(server)
		if err != nil {
			return nil, err
		}
		var p schemaValidateParams
		if uerr := json.Unmarshal(params, &p); uerr != nil {
			return nil, &rpc.Error{Code: rpc.CodeInvalidParams, Message: uerr.Error()}
		}
		v := server.Vault()
		content, ferr := v.ReadNote(p.Path)
		if ferr != nil {
			return nil, &rpc.Error{Code: rpc.CodeNotFound, Message: ferr.Error()}
		}
		return map[string]any{
			"violations": collectViolations(r, p.Path, content, vaultRefResolver{v: v}),
		}, nil
	}
}

// collectViolations parses a note's frontmatter and returns every schema
// violation across all applicable schemas.
func collectViolations(r *schema.Registry, relPath, content string, refs schema.NoteRefResolver) []map[string]any {
	doc := markdown.NewParser().Parse(content)
	fm := frontmatterMap(doc.Frontmatter)
	name := relPathToName(relPath)
	var out []map[string]any
	for _, s := range r.ApplicableTo(name) {
		for _, v := range schema.Validate(s, fm, refs) {
			out = append(out, map[string]any{
				"schemaId": s.ID,
				"field":    v.Field,
				"code":     v.Code,
				"severity": string(v.Severity),
				"message":  v.Message,
			})
		}
	}
	return out
}

// frontmatterMap parses raw frontmatter YAML into a generic map. Returns an
// empty map on parse failure so the caller's downstream logic still runs.
func frontmatterMap(raw string) map[string]any {
	if raw == "" {
		return map[string]any{}
	}
	out := make(map[string]any)
	if err := yamlUnmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func relPathToName(relPath string) string {
	if len(relPath) > 3 && relPath[len(relPath)-3:] == ".md" {
		return relPath[:len(relPath)-3]
	}
	return relPath
}

// schema/listPacks

func handleSchemaListPacks() rpc.Handler {
	return func(_ context.Context, _ json.RawMessage) (any, *rpc.Error) {
		packs, err := schema.PackList()
		if err != nil {
			return nil, &rpc.Error{Code: rpc.CodeInternal, Message: err.Error()}
		}
		return map[string]any{"packs": packs}, nil
	}
}

// schema/exportPack

type schemaExportPackParams struct {
	Pack string `json:"pack"`
}

func handleSchemaExportPack() rpc.Handler {
	return func(_ context.Context, params json.RawMessage) (any, *rpc.Error) {
		var p schemaExportPackParams
		if uerr := json.Unmarshal(params, &p); uerr != nil {
			return nil, &rpc.Error{Code: rpc.CodeInvalidParams, Message: uerr.Error()}
		}
		files, err := schema.PackFiles(p.Pack)
		if err != nil {
			return nil, &rpc.Error{Code: rpc.CodeNotFound, Message: err.Error()}
		}
		out := make(map[string]string, len(files))
		for k, v := range files {
			out[k] = string(v)
		}
		return map[string]any{"files": out}, nil
	}
}

// schemaTemplateVars constructs the TemplateVars used by createFromHierarchy.
// Pulled out so we can stub time in tests later if needed.
func schemaTemplateVars(name string) schema.TemplateVars {
	return schema.TemplateVars{
		Now:  time.Now(),
		User: schema.DefaultUser(),
		Name: name,
	}
}
