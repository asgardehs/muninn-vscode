// Package refindex maintains an in-memory inverted index from referenced
// note name -> (source file, field name, schema ID). It mirrors the
// wikilink index's shape but tracks schema-declared TypeReference fields
// instead of [[wikilinks]]. Refactor uses this to find frontmatter fields
// that need rewriting when a note is renamed.
package refindex

import "sync"

// ReferenceEdge is one frontmatter reference from a source file to a target.
type ReferenceEdge struct {
	SourceFile string // vault-relative path
	Field      string // schema field key
	Target     string // referenced note's hierarchy name
	SchemaID   string // schema that declared this field (informational)
}

// Index is a thread-safe inverted index of typed references.
type Index struct {
	mu        sync.RWMutex
	bySource  map[string][]ReferenceEdge // source file -> outgoing edges
	byTarget  map[string][]ReferenceEdge // target name -> incoming edges
}

// NewIndex returns an empty Index.
func NewIndex() *Index {
	return &Index{
		bySource: make(map[string][]ReferenceEdge),
		byTarget: make(map[string][]ReferenceEdge),
	}
}

// Update replaces all edges originating from sourceFile.
func (idx *Index) Update(sourceFile string, edges []ReferenceEdge) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.removeBySource(sourceFile)
	for i := range edges {
		edges[i].SourceFile = sourceFile
	}
	idx.bySource[sourceFile] = edges
	for _, e := range edges {
		idx.byTarget[e.Target] = append(idx.byTarget[e.Target], e)
	}
}

// Remove drops all edges originating from sourceFile.
func (idx *Index) Remove(sourceFile string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.removeBySource(sourceFile)
	delete(idx.bySource, sourceFile)
}

// RefsTo returns all edges pointing at the named target.
func (idx *Index) RefsTo(targetName string) []ReferenceEdge {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	out := make([]ReferenceEdge, len(idx.byTarget[targetName]))
	copy(out, idx.byTarget[targetName])
	return out
}

// removeBySource is a helper; caller must hold idx.mu.
func (idx *Index) removeBySource(sourceFile string) {
	for _, e := range idx.bySource[sourceFile] {
		sources := idx.byTarget[e.Target]
		filtered := sources[:0]
		for _, s := range sources {
			if s.SourceFile != sourceFile {
				filtered = append(filtered, s)
			}
		}
		if len(filtered) == 0 {
			delete(idx.byTarget, e.Target)
		} else {
			idx.byTarget[e.Target] = filtered
		}
	}
}
