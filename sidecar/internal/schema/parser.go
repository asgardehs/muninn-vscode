package schema

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Parse decodes one Schema from YAML bytes.
func Parse(b []byte) (*Schema, error) {
	var s Schema
	if err := yaml.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("yaml unmarshal: %w", err)
	}
	if err := validate(&s); err != nil {
		return nil, err
	}
	return &s, nil
}

func validate(s *Schema) error {
	if s.ID == "" {
		return fmt.Errorf("schema is missing required 'id'")
	}
	if s.Pattern == "" {
		return fmt.Errorf("schema %q is missing required 'pattern'", s.ID)
	}
	for i, f := range s.Frontmatter {
		if f.Key == "" {
			return fmt.Errorf("schema %q frontmatter[%d] missing 'key'", s.ID, i)
		}
		if f.Type == "" {
			return fmt.Errorf("schema %q field %q missing 'type'", s.ID, f.Key)
		}
		switch f.Type {
		case TypeString, TypeStringArray, TypeDate, TypeEnum,
			TypeNumber, TypeBoolean, TypeNoteRef, TypeReference:
		default:
			return fmt.Errorf("schema %q field %q has unknown type %q",
				s.ID, f.Key, f.Type)
		}
		if f.Type == TypeEnum && len(f.Vocabulary) == 0 {
			return fmt.Errorf("schema %q field %q is enum but has empty vocabulary",
				s.ID, f.Key)
		}
		if f.Type == TypeReference && strings.TrimSpace(f.Target) == "" {
			return fmt.Errorf("schema %q field %q is reference but has no target pattern",
				s.ID, f.Key)
		}
	}
	return nil
}
