package schema

import (
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
