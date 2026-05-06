package schema

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Severity classifies a violation.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// Violation is one schema-validation failure. Field is empty for note-level
// problems (e.g., "no schema applies"). Code is a short machine identifier
// suitable for an LSP diagnostic code.
type Violation struct {
	Field    string
	Code     string
	Severity Severity
	Message  string
}

// NoteRefResolver reports whether a wikilink target resolves to a real note.
// Plumbed in by callers so the validator can stay decoupled from the wikilink
// index.
type NoteRefResolver interface {
	NoteExists(target string) bool
}

// Validate checks a frontmatter map against a schema. Returns the violations
// found. The wikilink resolver is optional — when nil, note-ref fields skip
// the existence check.
func Validate(s *Schema, fm map[string]any, refs NoteRefResolver) []Violation {
	var out []Violation

	for _, f := range s.Frontmatter {
		val, present := fm[f.Key]
		if !present || isEmpty(val) {
			if f.Required {
				out = append(out, Violation{
					Field:    f.Key,
					Code:     "missing-required",
					Severity: SeverityError,
					Message:  fmt.Sprintf("required field %q is missing", f.Key),
				})
			}
			continue
		}
		if v := validateField(&f, val, refs); v != nil {
			out = append(out, *v)
		}
	}
	return out
}

func validateField(f *Field, val any, refs NoteRefResolver) *Violation {
	switch f.Type {
	case TypeString:
		if _, ok := val.(string); !ok {
			return typeError(f, val, "string")
		}
	case TypeStringArray:
		arr, ok := val.([]any)
		if !ok {
			return typeError(f, val, "array of strings")
		}
		for _, item := range arr {
			if _, ok := item.(string); !ok {
				return typeError(f, val, "array of strings")
			}
		}
	case TypeDate:
		s, ok := val.(string)
		if !ok {
			return typeError(f, val, "date (YYYY-MM-DD)")
		}
		if _, err := time.Parse("2006-01-02", s); err != nil {
			return typeError(f, val, "date (YYYY-MM-DD)")
		}
	case TypeEnum:
		s, ok := val.(string)
		if !ok {
			return typeError(f, val, "enum string")
		}
		if !inVocabulary(s, f.Vocabulary) {
			return &Violation{
				Field:    f.Key,
				Code:     "invalid-enum-value",
				Severity: SeverityWarning,
				Message: fmt.Sprintf("%q is not in %s vocabulary {%s}",
					s, f.Key, strings.Join(f.Vocabulary, ", ")),
			}
		}
	case TypeNumber:
		switch v := val.(type) {
		case int, int64, float64:
			_ = v
		case string:
			if _, err := strconv.ParseFloat(v, 64); err != nil {
				return typeError(f, val, "number")
			}
		default:
			return typeError(f, val, "number")
		}
	case TypeBoolean:
		if _, ok := val.(bool); !ok {
			return typeError(f, val, "boolean")
		}
	case TypeNoteRef:
		s, ok := val.(string)
		if !ok {
			return typeError(f, val, "note-ref string")
		}
		if refs != nil && !refs.NoteExists(s) {
			return &Violation{
				Field:    f.Key,
				Code:     "note-ref-unresolved",
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("note-ref %q does not resolve to any note", s),
			}
		}
	case TypeReference:
		s, ok := val.(string)
		if !ok {
			return typeError(f, val, "reference dot-path string")
		}
		if f.Target != "" && !MatchPattern(f.Target, s) {
			return &Violation{
				Field:    f.Key,
				Code:     "reference-target-mismatch",
				Severity: SeverityError,
				Message:  fmt.Sprintf("%s = %q does not match target pattern %q", f.Key, s, f.Target),
			}
		}
		if refs != nil && !refs.NoteExists(s) {
			return &Violation{
				Field:    f.Key,
				Code:     "reference-unresolved",
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("reference %q does not resolve to any note", s),
			}
		}
	}
	return nil
}

func typeError(f *Field, got any, want string) *Violation {
	return &Violation{
		Field:    f.Key,
		Code:     "type-mismatch",
		Severity: SeverityWarning,
		Message:  fmt.Sprintf("%s should be %s, got %T", f.Key, want, got),
	}
}

func isEmpty(v any) bool {
	if v == nil {
		return true
	}
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x) == ""
	case []any:
		return len(x) == 0
	case map[string]any:
		return len(x) == 0
	}
	return false
}

func inVocabulary(s string, vocab []string) bool {
	for _, v := range vocab {
		if v == s {
			return true
		}
	}
	return false
}
