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

func TestParse_RejectsReferenceWithoutTarget(t *testing.T) {
	yaml := []byte(`
id: test
pattern: "trainings.*"
frontmatter:
  - key: instructor
    type: reference
    required: true
`)
	if _, err := Parse(yaml); err == nil {
		t.Fatal("expected parse error for reference without target")
	}
}

func TestParse_RejectsReferenceWithEmptyTarget(t *testing.T) {
	yaml := []byte(`
id: test
pattern: "t.*"
frontmatter:
  - key: x
    type: reference
    target: ""
`)
	if _, err := Parse(yaml); err == nil {
		t.Fatal("expected parse error for reference with empty target")
	}
}
