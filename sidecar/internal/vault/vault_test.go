package vault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeNote(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestListNotesIncludesNestedAndExcludesHidden(t *testing.T) {
	root := t.TempDir()
	writeNote(t, root, "foo.md", "# Foo")
	writeNote(t, root, "foo.bar.md", "# Foo Bar")
	writeNote(t, root, "sub/nested.md", "# Nested")
	writeNote(t, root, ".muninn/schemas/incident.yml", "id: incident")
	writeNote(t, root, ".git/HEAD", "ref: refs/heads/main")
	// Non-.md files should be ignored.
	writeNote(t, root, "ignore-me.txt", "not markdown")

	v := New(root)
	notes, err := v.ListNotes()
	if err != nil {
		t.Fatalf("ListNotes: %v", err)
	}

	want := map[string]bool{
		"foo.md":         true,
		"foo.bar.md":     true,
		"sub/nested.md":  true,
	}
	if len(notes) != len(want) {
		t.Errorf("got %d notes, want %d: %v", len(notes), len(want), notes)
	}
	for _, n := range notes {
		if !want[n] {
			t.Errorf("unexpected note in result: %q", n)
		}
		if strings.HasPrefix(n, ".") {
			t.Errorf("hidden path leaked into results: %q", n)
		}
	}
}

func TestReadNote(t *testing.T) {
	root := t.TempDir()
	writeNote(t, root, "hello.md", "# Hello\n\nWorld")

	v := New(root)
	got, err := v.ReadNote("hello.md")
	if err != nil {
		t.Fatalf("ReadNote: %v", err)
	}
	if got != "# Hello\n\nWorld" {
		t.Errorf("got %q", got)
	}
}

func TestReadNoteMissing(t *testing.T) {
	v := New(t.TempDir())
	if _, err := v.ReadNote("nope.md"); err == nil {
		t.Error("expected error for missing note")
	}
}

func TestRootAndAbsPath(t *testing.T) {
	root := t.TempDir()
	v := New(root)
	if v.Root() != root {
		t.Errorf("Root: got %q, want %q", v.Root(), root)
	}
	want := filepath.Join(root, "foo", "bar.md")
	if v.AbsPath("foo/bar.md") != want {
		t.Errorf("AbsPath: got %q, want %q", v.AbsPath("foo/bar.md"), want)
	}
}
