package hierarchy

import (
	"reflect"
	"sort"
	"testing"
)

func TestBuildSimpleHierarchy(t *testing.T) {
	tree := Build([]string{"foo.md", "foo.bar.md", "foo.bar.baz.md"})

	cases := []struct {
		name           string
		expectExists   bool
		expectIsStub   bool
		expectFile     string
		expectChildren []string
		expectParent   string
	}{
		{"foo", true, false, "foo.md", []string{"foo.bar"}, ""},
		{"foo.bar", true, false, "foo.bar.md", []string{"foo.bar.baz"}, "foo"},
		{"foo.bar.baz", true, false, "foo.bar.baz.md", nil, "foo.bar"},
	}
	for _, tc := range cases {
		n := tree.Get(tc.name)
		if (n != nil) != tc.expectExists {
			t.Errorf("%s: exists=%v, want %v", tc.name, n != nil, tc.expectExists)
			continue
		}
		if n.IsStub != tc.expectIsStub {
			t.Errorf("%s: IsStub=%v, want %v", tc.name, n.IsStub, tc.expectIsStub)
		}
		if n.File != tc.expectFile {
			t.Errorf("%s: File=%q, want %q", tc.name, n.File, tc.expectFile)
		}
		if !reflect.DeepEqual(n.Children, tc.expectChildren) {
			t.Errorf("%s: Children=%v, want %v", tc.name, n.Children, tc.expectChildren)
		}
		if n.Parent != tc.expectParent {
			t.Errorf("%s: Parent=%q, want %q", tc.name, n.Parent, tc.expectParent)
		}
	}
}

func TestBuildCreatesStubs(t *testing.T) {
	// Only the leaf has a file; foo and foo.bar should be stubs.
	tree := Build([]string{"foo.bar.baz.md"})

	for _, name := range []string{"foo", "foo.bar", "foo.bar.baz"} {
		n := tree.Get(name)
		if n == nil {
			t.Fatalf("expected node %q", name)
		}
	}
	if !tree.Get("foo").IsStub {
		t.Errorf("foo should be stub")
	}
	if !tree.Get("foo.bar").IsStub {
		t.Errorf("foo.bar should be stub")
	}
	if tree.Get("foo.bar.baz").IsStub {
		t.Errorf("foo.bar.baz should not be stub")
	}
	if tree.Get("foo.bar.baz").File != "foo.bar.baz.md" {
		t.Errorf("foo.bar.baz.File=%q", tree.Get("foo.bar.baz").File)
	}
}

func TestRoots(t *testing.T) {
	tree := Build([]string{"foo.md", "bar.md", "bar.x.md", "baz.y.z.md"})
	got := tree.Roots()
	gotNames := make([]string, len(got))
	for i, n := range got {
		gotNames[i] = n.Name
	}
	want := []string{"bar", "baz", "foo"}
	if !reflect.DeepEqual(gotNames, want) {
		t.Errorf("Roots names = %v, want %v", gotNames, want)
	}
}

func TestChildrenAndSiblings(t *testing.T) {
	tree := Build([]string{
		"projects.md",
		"projects.alpha.md",
		"projects.beta.md",
		"projects.gamma.md",
	})

	// Children of "projects".
	children := tree.Children("projects")
	gotKids := make([]string, len(children))
	for i, c := range children {
		gotKids[i] = c.Name
	}
	wantKids := []string{"projects.alpha", "projects.beta", "projects.gamma"}
	if !reflect.DeepEqual(gotKids, wantKids) {
		t.Errorf("Children = %v, want %v", gotKids, wantKids)
	}

	// Siblings of "projects.beta" (alpha + gamma).
	sibs := tree.Siblings("projects.beta")
	gotSibs := make([]string, len(sibs))
	for i, s := range sibs {
		gotSibs[i] = s.Name
	}
	sort.Strings(gotSibs)
	wantSibs := []string{"projects.alpha", "projects.gamma"}
	if !reflect.DeepEqual(gotSibs, wantSibs) {
		t.Errorf("Siblings = %v, want %v", gotSibs, wantSibs)
	}

	// Siblings of a root return other roots.
	tree2 := Build([]string{"a.md", "b.md", "c.md"})
	sibsB := tree2.Siblings("b")
	if len(sibsB) != 2 || sibsB[0].Name != "a" || sibsB[1].Name != "c" {
		t.Errorf("root siblings of b: %v", sibsB)
	}
}

func TestSubdirectoriesIgnoredForHierarchy(t *testing.T) {
	tree := Build([]string{"sub/nested.md", "nested.md"})
	// Both files have basename "nested" — they collide on the same node.
	// Last write wins; either is acceptable for v0.1 (subdirs are not a
	// supported convention).
	n := tree.Get("nested")
	if n == nil {
		t.Fatal("expected node 'nested'")
	}
	if n.File != "nested.md" && n.File != "sub/nested.md" {
		t.Errorf("File=%q, want one of {nested.md, sub/nested.md}", n.File)
	}
}

func TestEmptyVault(t *testing.T) {
	tree := Build(nil)
	if tree.Len() != 0 {
		t.Errorf("Len=%d, want 0", tree.Len())
	}
	if len(tree.Roots()) != 0 {
		t.Errorf("Roots: %v", tree.Roots())
	}
}

func TestNamesSorted(t *testing.T) {
	tree := Build([]string{"zeta.md", "alpha.md", "alpha.beta.md"})
	got := tree.Names()
	want := []string{"alpha", "alpha.beta", "zeta"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Names=%v, want %v", got, want)
	}
}
