package schema

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestEmbeddedGenericPackLoads(t *testing.T) {
	r, err := loadFromFS(builtinFS, "builtin/generic")
	if err != nil {
		t.Fatalf("load embedded generic: %v", err)
	}
	want := []string{"daily", "decision", "meeting", "reference", "til"}
	got := make([]string, 0, r.Len())
	for _, s := range r.All() {
		got = append(got, s.ID)
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ids=%v, want %v", got, want)
	}
}

func TestApplicableToOrdering(t *testing.T) {
	r, err := loadFromFS(builtinFS, "builtin/generic")
	if err != nil {
		t.Fatal(err)
	}
	matches := r.ApplicableTo("daily.2026-05-06")
	if len(matches) != 1 || matches[0].ID != "daily" {
		t.Errorf("daily applicable: %+v", matches)
	}

	matches = r.ApplicableTo("meetings.team.standup")
	if len(matches) != 1 || matches[0].ID != "meeting" {
		t.Errorf("meetings applicable: %+v", matches)
	}

	matches = r.ApplicableTo("foo.bar")
	if len(matches) != 0 {
		t.Errorf("foo.bar should match nothing, got %+v", matches)
	}
}

func TestEnumValuesFor(t *testing.T) {
	r, err := loadFromFS(builtinFS, "builtin/generic")
	if err != nil {
		t.Fatal(err)
	}
	got := r.EnumValuesFor("decisions.api-versioning", "status")
	want := []string{"proposed", "accepted", "deprecated", "superseded"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("status enum: got %v, want %v", got, want)
	}

	// Field that isn't an enum returns nothing.
	got = r.EnumValuesFor("decisions.api-versioning", "title")
	if len(got) != 0 {
		t.Errorf("title (not enum) should yield nothing, got %v", got)
	}
}

func TestPackList(t *testing.T) {
	packs, err := PackList()
	if err != nil {
		t.Fatal(err)
	}
	wantPacks := map[string]bool{"generic": true, "ehs": true}
	for _, p := range packs {
		if !wantPacks[p] {
			t.Errorf("unexpected pack %q in PackList result %v", p, packs)
		}
		delete(wantPacks, p)
	}
	for p := range wantPacks {
		t.Errorf("expected pack %q in PackList result, got %v", p, packs)
	}
}

func TestPackFiles(t *testing.T) {
	files, err := PackFiles("generic")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := files["daily.yml"]; !ok {
		t.Errorf("expected daily.yml in generic pack, got %v", keysOf(files))
	}
}

func TestEhsPackParses(t *testing.T) {
	files, err := PackFiles("ehs")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"audit.yml", "incident.yml", "inspection.yml", "jha.yml", "training.yml"}
	for _, name := range want {
		body, ok := files[name]
		if !ok {
			t.Errorf("EHS pack missing %s", name)
			continue
		}
		s, err := Parse(body)
		if err != nil {
			t.Errorf("parse %s: %v", name, err)
			continue
		}
		if s.ID == "" || s.Pattern == "" {
			t.Errorf("%s: id=%q pattern=%q", name, s.ID, s.Pattern)
		}
	}
}

func keysOf(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestLoadLayersVaultOnEmbedded asserts the property Adam called out on
// 2026-05-06: dropping a custom schema into <vault>/.muninn/schemas/ must
// not silently delete the embedded generic pack the user was relying on.
func TestLoadLayersVaultOnEmbedded(t *testing.T) {
	dir := t.TempDir()
	schemas := filepath.Join(dir, ".muninn", "schemas")
	if err := os.MkdirAll(schemas, 0o755); err != nil {
		t.Fatal(err)
	}

	// Override the embedded "daily" schema with custom fields and priority.
	override := []byte(`
id: daily
label: Custom Daily Override
pattern: "daily.*"
priority: 999
frontmatter:
  - key: title
    type: string
    required: true
`)
	if err := os.WriteFile(filepath.Join(schemas, "daily.yml"), override, 0o644); err != nil {
		t.Fatal(err)
	}

	// Add a brand-new schema not in the embedded pack.
	custom := []byte(`
id: workout
label: Workout Log
pattern: "fitness.workouts.*"
priority: 50
frontmatter:
  - key: title
    type: string
    required: true
`)
	if err := os.WriteFile(filepath.Join(schemas, "workout.yml"), custom, 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Custom override won.
	daily := r.Get("daily")
	if daily == nil {
		t.Fatal("daily schema missing after overlay")
	}
	if daily.Label != "Custom Daily Override" {
		t.Errorf("daily.Label = %q, want Custom Daily Override", daily.Label)
	}
	if daily.Priority != 999 {
		t.Errorf("daily.Priority = %d, want 999", daily.Priority)
	}

	// Brand-new schema is present.
	if r.Get("workout") == nil {
		t.Errorf("workout schema not loaded from vault")
	}

	// Other embedded schemas are still there — they weren't dropped.
	for _, id := range []string{"meeting", "decision", "til", "reference"} {
		if r.Get(id) == nil {
			t.Errorf("embedded schema %q dropped after vault overlay", id)
		}
	}
}

func TestLoadFallsBackToEmbeddedWhenNoVaultDir(t *testing.T) {
	dir := t.TempDir() // no .muninn/schemas inside
	r, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, id := range []string{"daily", "meeting", "decision", "til", "reference"} {
		if r.Get(id) == nil {
			t.Errorf("embedded schema %q missing in fresh-vault load", id)
		}
	}
}
