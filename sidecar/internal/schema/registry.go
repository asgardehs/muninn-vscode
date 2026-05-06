package schema

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Registry holds the active set of schemas. Construct via Load.
type Registry struct {
	schemas []*Schema
	byID    map[string]*Schema
}

//go:embed builtin
var builtinFS embed.FS

// PackList returns the names of every embedded schema pack (one per
// subdirectory under builtin/).
func PackList() ([]string, error) {
	entries, err := builtinFS.ReadDir("builtin")
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

// PackFiles returns the YAML bytes of each schema in the named pack, keyed by
// filename (e.g., "incident.yml" -> bytes). Used by the extension's
// installSchemaPack command via a passthrough RPC.
func PackFiles(pack string) (map[string][]byte, error) {
	subdir := filepath.Join("builtin", pack)
	entries, err := builtinFS.ReadDir(subdir)
	if err != nil {
		return nil, err
	}
	out := make(map[string][]byte, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yml") {
			continue
		}
		b, err := builtinFS.ReadFile(filepath.Join(subdir, e.Name()))
		if err != nil {
			return nil, err
		}
		out[e.Name()] = b
	}
	return out, nil
}

// Load builds a Registry from the embedded "generic" pack overlaid with any
// user-defined schemas found in <vault>/.muninn/schemas/. Vault YAML wins on
// id collisions, but the embedded defaults remain available when they aren't
// overridden — so dropping a single custom schema doesn't silently delete
// the rest of the generic pack the user was relying on.
func Load(vaultRoot string) (*Registry, error) {
	r := newRegistry()
	if err := r.loadDir(builtinFS, "builtin/generic"); err != nil {
		return nil, fmt.Errorf("load embedded generic pack: %w", err)
	}
	vaultDir := filepath.Join(vaultRoot, ".muninn", "schemas")
	if info, err := os.Stat(vaultDir); err == nil && info.IsDir() {
		if err := r.loadDir(os.DirFS(vaultDir), "."); err != nil {
			return nil, fmt.Errorf("load vault schemas: %w", err)
		}
	}
	return r, nil
}

func newRegistry() *Registry {
	return &Registry{byID: make(map[string]*Schema)}
}

func loadFromFS(fsys fs.FS, dir string) (*Registry, error) {
	r := newRegistry()
	if err := r.loadDir(fsys, dir); err != nil {
		return nil, err
	}
	return r, nil
}

// loadDir merges every *.yml file in dir into r. Existing schemas with the
// same id are replaced; new ids are appended.
func (r *Registry) loadDir(fsys fs.FS, dir string) error {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return fmt.Errorf("read schema dir %q: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yml") {
			continue
		}
		b, err := fs.ReadFile(fsys, filepath.Join(dir, e.Name()))
		if err != nil {
			return fmt.Errorf("read %q: %w", e.Name(), err)
		}
		s, err := Parse(b)
		if err != nil {
			return fmt.Errorf("parse %q: %w", e.Name(), err)
		}
		r.add(s)
	}
	return nil
}

func (r *Registry) add(s *Schema) {
	if existing, ok := r.byID[s.ID]; ok {
		// Replace existing entry so vault overrides embedded.
		for i, ex := range r.schemas {
			if ex == existing {
				r.schemas[i] = s
				break
			}
		}
	} else {
		r.schemas = append(r.schemas, s)
	}
	r.byID[s.ID] = s
}

// All returns every schema in priority-descending, then id-ascending order.
func (r *Registry) All() []*Schema {
	out := append([]*Schema(nil), r.schemas...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority > out[j].Priority
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// Get returns the schema with the given id, or nil.
func (r *Registry) Get(id string) *Schema { return r.byID[id] }

// ApplicableTo returns every schema whose pattern matches the hierarchy name,
// in priority-descending then id-ascending order.
func (r *Registry) ApplicableTo(hierarchyName string) []*Schema {
	var matches []*Schema
	for _, s := range r.schemas {
		if MatchPattern(s.Pattern, hierarchyName) {
			matches = append(matches, s)
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Priority != matches[j].Priority {
			return matches[i].Priority > matches[j].Priority
		}
		return matches[i].ID < matches[j].ID
	})
	return matches
}

// EnumValuesFor returns the enum vocabulary applicable to a (note name,
// frontmatter field) pair, deduped across every schema that matches the
// note. The first applicable schema's vocabulary appears first.
func (r *Registry) EnumValuesFor(hierarchyName, fieldKey string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, s := range r.ApplicableTo(hierarchyName) {
		f := s.FieldByKey(fieldKey)
		if f == nil || f.Type != TypeEnum {
			continue
		}
		for _, v := range f.Vocabulary {
			if !seen[v] {
				seen[v] = true
				out = append(out, v)
			}
		}
	}
	return out
}

// Len reports the total schema count.
func (r *Registry) Len() int { return len(r.schemas) }
