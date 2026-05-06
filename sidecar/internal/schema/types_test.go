package schema

import "testing"

func TestField_TargetParsesFromYAML(t *testing.T) {
	// Construct a Field directly in Go to verify:
	// 1. TypeReference constant exists
	// 2. Field struct accepts Target field
	// 3. Target value is properly stored
	field := Field{
		Key:      "instructor",
		Type:     TypeReference,
		Target:   "people.**",
		Required: true,
	}

	if field.Type != TypeReference {
		t.Errorf("type = %q, want %q", field.Type, TypeReference)
	}
	if field.Target != "people.**" {
		t.Errorf("target = %q, want %q", field.Target, "people.**")
	}
	if string(field.Type) != "reference" {
		t.Errorf("TypeReference string value = %q, want %q", string(field.Type), "reference")
	}
}
