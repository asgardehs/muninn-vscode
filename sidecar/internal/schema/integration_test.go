package schema_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/asgardehs/muninn-sidecar/internal/schema"
)

func TestIntegration_ReferenceFieldRoundtrip(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "training.yml")
	schemaYAML := `
id: training
label: Training
pattern: "trainings.*"
frontmatter:
  - key: title
    type: string
    required: true
  - key: instructor
    type: reference
    target: people.**
    required: true
`
	if err := os.WriteFile(schemaPath, []byte(schemaYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	resolver := stubResolver{}
	good := schema.Validate(s, map[string]any{
		"title":      "Forklift Training",
		"instructor": "people.john-smith",
	}, resolver)
	if len(good) != 0 {
		t.Errorf("expected no violations on valid input, got %v", good)
	}

	bad := schema.Validate(s, map[string]any{
		"title":      "Forklift Training",
		"instructor": "places.warehouse",
	}, resolver)
	if len(bad) != 1 || bad[0].Code != "reference-target-mismatch" {
		t.Errorf("expected target-mismatch violation, got %v", bad)
	}
}

type stubResolver struct{}

func (stubResolver) NoteExists(target string) bool {
	return target == "people.john-smith"
}
