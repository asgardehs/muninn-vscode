// Package schema implements Muninn's generic schema engine: dot-path pattern
// matching against hierarchy names, frontmatter validation against typed
// fields, and template rendering for schema-driven note creation.
//
// Schemas are pure YAML. The engine itself is domain-neutral; opinionated
// example packs (generic, ehs) ship as embedded YAML for users to install
// into their vault's .muninn/schemas/ directory.
package schema

// FieldType is the value type of a frontmatter field.
type FieldType string

const (
	TypeString      FieldType = "string"
	TypeStringArray FieldType = "string-array"
	TypeDate        FieldType = "date"
	TypeEnum        FieldType = "enum"
	TypeNumber      FieldType = "number"
	TypeBoolean     FieldType = "boolean"
	TypeNoteRef     FieldType = "note-ref"
)

// Field describes one frontmatter field a schema requires or permits.
type Field struct {
	Key         string    `yaml:"key"`
	Type        FieldType `yaml:"type"`
	Required    bool      `yaml:"required"`
	Vocabulary  []string  `yaml:"vocabulary"`
	Description string    `yaml:"description"`
}

// ChildConstraint declares that a child note matching Pattern must conform to
// the named Schema. Pattern is glob over a single hierarchy segment relative
// to the parent (e.g., "*.witness-statement").
type ChildConstraint struct {
	Pattern string `yaml:"pattern"`
	Schema  string `yaml:"schema"`
}

// Template holds the body and frontmatter defaults applied when a note is
// created against this schema. Variables ({{today}}, {{now}}, {{user}},
// {{name}}) are expanded by the template renderer.
type Template struct {
	Body                string         `yaml:"body"`
	FrontmatterDefaults map[string]any `yaml:"frontmatterDefaults"`
}

// Schema is one schema definition.
type Schema struct {
	ID          string            `yaml:"id"`
	Label       string            `yaml:"label"`
	Description string            `yaml:"description"`
	Pattern     string            `yaml:"pattern"`
	Priority    int               `yaml:"priority"`
	Frontmatter []Field           `yaml:"frontmatter"`
	Children    []ChildConstraint `yaml:"children"`
	Template    Template          `yaml:"template"`
}

// FieldByKey returns the named field, or nil if not declared.
func (s *Schema) FieldByKey(key string) *Field {
	for i := range s.Frontmatter {
		if s.Frontmatter[i].Key == key {
			return &s.Frontmatter[i]
		}
	}
	return nil
}
