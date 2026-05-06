// Package refactor implements vault-wide note renames. A rename moves a
// file on disk and rewrites every [[wikilink]] (and, after Phase C, every
// schema-declared reference field) that points to it. Refactor lives in
// the RPC channel rather than LSP — see docs/specs/v0.2.0.md.
package refactor

// FileEdit is a planned edit to one file's contents.
type FileEdit struct {
	Path       string // vault-relative path
	OldContent string
	NewContent string
}

// Plan is the full set of changes a single rename will perform. Build it,
// inspect it, then Apply it. Plans are inert — building one does not touch
// the filesystem.
type Plan struct {
	OldName    string     // hierarchy name before rename, e.g. "people.john-smith"
	NewName    string     // hierarchy name after rename
	RenameFrom string     // vault-relative file path to move from
	RenameTo   string     // vault-relative file path to move to
	FileEdits  []FileEdit // per-file content rewrites (backlink sources)
}
