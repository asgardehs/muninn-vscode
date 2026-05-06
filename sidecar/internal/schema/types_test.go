package schema

import "testing"

func TestField_HasTargetAndReferenceType(t *testing.T) {
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
