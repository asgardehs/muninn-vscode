// Package hierarchy models the dot-path note hierarchy that drives Dendron-style
// lookup, tree views, and createFromHierarchy.
//
// Conventions (v0.1):
//   - A note's "name" is its filename basename without the .md extension.
//   - Dot-separated segments in the name form the hierarchy path
//     (e.g., "foo.bar.baz" is a child of "foo.bar", which is a child of "foo").
//   - Subdirectories are NOT part of the hierarchy. Notes are expected to
//     live at the vault root with dot-path filenames (Dendron convention).
//   - A "stub" is a hierarchy node that has children but no .md file of its
//     own (e.g., "foo.bar" is a stub when only "foo.bar.baz.md" exists).
package hierarchy

import (
	"path/filepath"
	"sort"
	"strings"
)

// Node is one position in the hierarchy. File is empty for stubs.
type Node struct {
	Name     string
	Parent   string
	File     string
	IsStub   bool
	Children []string
}

// Tree is a built dot-path index.
type Tree struct {
	nodes map[string]*Node
}

// Build constructs a Tree from a set of vault-relative .md paths. Subdirectory
// prefixes are stripped — only the basename is considered for the hierarchy.
func Build(relPaths []string) *Tree {
	t := &Tree{nodes: make(map[string]*Node)}

	for _, rel := range relPaths {
		base := strings.TrimSuffix(filepath.Base(rel), ".md")
		if base == "" {
			continue
		}
		t.ensureNode(base, rel)

		// Walk up the dot-path, ensuring each ancestor exists at least as a stub.
		current := base
		for {
			parent := parentName(current)
			if parent == "" {
				break
			}
			t.ensureNode(parent, "")
			current = parent
		}
	}

	// Link children. Sort children lexicographically for deterministic output.
	for name, n := range t.nodes {
		if n.Parent == "" {
			continue
		}
		p, ok := t.nodes[n.Parent]
		if !ok {
			continue
		}
		p.Children = append(p.Children, name)
	}
	for _, n := range t.nodes {
		sort.Strings(n.Children)
	}
	return t
}

// ensureNode creates or upgrades a node. If file is non-empty, the node is
// marked as a real file (not a stub) even if it was previously a stub.
func (t *Tree) ensureNode(name, file string) {
	if existing, ok := t.nodes[name]; ok {
		if file != "" {
			existing.File = file
			existing.IsStub = false
		}
		return
	}
	t.nodes[name] = &Node{
		Name:   name,
		Parent: parentName(name),
		File:   file,
		IsStub: file == "",
	}
}

// parentName returns the parent dot-path of name, or "" if name is a root.
func parentName(name string) string {
	idx := strings.LastIndex(name, ".")
	if idx < 0 {
		return ""
	}
	return name[:idx]
}

// Get returns the node for a name, or nil if not present.
func (t *Tree) Get(name string) *Node { return t.nodes[name] }

// Has reports whether the tree knows about the given name.
func (t *Tree) Has(name string) bool { _, ok := t.nodes[name]; return ok }

// Names returns every node name, sorted.
func (t *Tree) Names() []string {
	out := make([]string, 0, len(t.nodes))
	for name := range t.nodes {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Roots returns every node with no parent, sorted by name.
func (t *Tree) Roots() []*Node {
	var out []*Node
	for _, n := range t.nodes {
		if n.Parent == "" {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Children returns the immediate children of name, sorted.
func (t *Tree) Children(name string) []*Node {
	n, ok := t.nodes[name]
	if !ok {
		return nil
	}
	out := make([]*Node, 0, len(n.Children))
	for _, c := range n.Children {
		out = append(out, t.nodes[c])
	}
	return out
}

// Siblings returns the other children of name's parent (excluding name itself).
// If name is a root, returns the other roots.
func (t *Tree) Siblings(name string) []*Node {
	n, ok := t.nodes[name]
	if !ok {
		return nil
	}
	var pool []*Node
	if n.Parent == "" {
		pool = t.Roots()
	} else {
		pool = t.Children(n.Parent)
	}
	out := make([]*Node, 0, len(pool))
	for _, p := range pool {
		if p.Name != name {
			out = append(out, p)
		}
	}
	return out
}

// Len reports the total number of nodes (including stubs).
func (t *Tree) Len() int { return len(t.nodes) }
