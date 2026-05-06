# Changelog

All notable changes to the Muninn VS Code extension are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

(no unreleased changes)

## [0.2.0] - 2026-05-06

### Added

- **`type: reference` schema field.** Frontmatter fields can now declare
  `type: reference` with an optional `target:` glob (e.g. `target: 'people.**'`).
  The schema engine validates that the field value resolves to a note matching the
  target pattern and surfaces a `reference-target-mismatch` diagnostic when it
  does not. The older `type: note-ref` spelling still works; `reference` is now
  the canonical name and `note-ref` is legacy.
- **`Muninn: Rename Note` command.** Renames the active note's file, updates all
  wikilinks that target it across the vault, and rewrites any `reference`
  frontmatter fields pointing to it. Rename is atomic on the LSP side; the
  extension applies the workspace edit in a single transaction.

### Internal

- `refactor` package: logic for rewriting wikilinks and reference-field values
  during a rename operation.
- `refindex` package: index of all reference-typed field values keyed by target
  note path, used to find fields that need rewriting on rename without a full
  vault scan.

### Migration notes

**EHS training schema — `instructor` field.** The bundled `ehs/training` schema
changed `instructor` from `type: string, required: true` to
`type: reference, target: 'people.**', required: false`. Existing training notes
with `instructor: "Bob Smith"` (a plain name string) will now show a
`reference-target-mismatch` diagnostic. To clear it, migrate the value to a
dot-path that matches the `people.**` pattern (e.g. `instructor: people.bob-smith`)
and create the corresponding note under your vault's `people/` hierarchy. The
field is no longer required, so removing it is also valid if you do not use the
`people.*` hierarchy.

All other schema changes in this release are additive and backwards-compatible.

## [0.1.1] - 2026-05-06

### Fixed

- Extension activation failed with `Cannot find module
  'vscode-languageclient/node'` because the runtime dependency was
  excluded from the packaged `.vsix`. The extension now bundles its
  TypeScript sources and runtime dependencies into a single
  `out/extension.js` via esbuild, so no `node_modules/` is required at
  install time.

## [0.1.0] - 2026-05-06

First public release. Establishes the v0.1 feature surface described in
[`docs/specs/2026-05-05-2110-muninn-vscode-rewrite/`](docs/specs/2026-05-05-2110-muninn-vscode-rewrite).

### Added

- **Hierarchical notes.** Files named with dot-paths (`foo.bar.baz.md`)
  participate in a tree where dot-separated segments are parent/child links.
  Missing intermediates synthesize as stubs.
- **Lookup palette.** `Muninn: Lookup` (default <kbd>Ctrl</kbd>+<kbd>L</kbd> / <kbd>⌘</kbd>+<kbd>L</kbd>) fuzzy-matches over the
  hierarchy and creates notes inline. `Muninn: Lookup Under Current Note`
  prefills with the active note's parent dot-path.
- **Hierarchy tree view** in the activity bar, lazy-loaded over the RPC
  channel; refreshes within ~100ms of filesystem changes.
- **Wikilinks via LSP.** `[[target]]` completion, heading-fragment
  completion (`[[target#`), go-to-definition, find-all-references,
  hover preview, broken-link and broken-fragment diagnostics, code-lens
  reference counts.
- **Tag completion.** `#` outside wikilinks completes against tags
  collected from frontmatter across the vault.
- **Schema engine.** YAML schemas in `<vault>/.muninn/schemas/*.yml`
  declare frontmatter fields, vocabularies, child constraints, and
  templates. Pattern matching uses glob over dot-paths (`*` for one
  segment, `**` for one-or-more). Template rendering supports
  `{{today}}`, `{{now}}`, `{{user}}`, `{{name}}` variables.
- **Schema-driven create.** When `vault/createFromHierarchy` finds an
  applicable schema for the new note's name, the schema's template
  body and frontmatter defaults are applied; caller-supplied
  frontmatter overrides defaults.
- **Schema diagnostics.** Required-field, type-mismatch, invalid-enum,
  and unresolved-note-ref violations surface as Problems-panel
  diagnostics with `muninn-schema` source and `<schemaId>/<code>`
  diagnostic codes. Malformed YAML short-circuits the schema pass to
  prevent flood-of-cascading-violations during incremental edits.
- **Frontmatter enum completion.** Inside `<key>:` lines in
  frontmatter, completion suggests the union of every applicable
  schema's vocabulary for that key.
- **Bundled schema packs.** `generic` (daily, meeting, reference,
  decision, til) and `ehs` (incident, JHA, inspection, training,
  audit). Install via `Muninn: Install Schema Pack`. The `generic`
  pack auto-loads as a fallback when a vault has no
  `.muninn/schemas/` directory yet.
- **Live schema reload.** Drops into `.muninn/schemas/` are picked up
  by the fsnotify watcher within ~100ms — no reload needed.
- **Schema layering.** User-authored schemas in `.muninn/schemas/`
  layer on top of the bundled `generic` pack instead of replacing it,
  so dropping a custom schema doesn't silently delete the defaults
  the user was relying on.
- **Filesystem watcher.** Vault edits update the wikilink index,
  refresh diagnostics on open editors, and emit `vault/changed` /
  `schema/changed` RPC notifications.
- **Multiplexed transport.** A single stdio pipe carries both LSP and
  custom JSON-RPC frames, distinguished by an extra `Channel:` header
  on top of LSP-style Content-Length framing.

### Notes

- VS Code 1.95+ required.
- Distributed via the [Open VSX Registry](https://open-vsx.org/) under
  the `pharomwinters` namespace (the maintainer's Eclipse Foundation
  identity). VSCodium, Cursor, Theia, Gitpod, and code-server install
  directly. Stock VS Code users sideload the matching `.vsix` from the
  GitHub Release.
- Platform-targeted `.vsix` for linux-x64, linux-arm64, darwin-x64,
  darwin-arm64, and win32-x64. CGO is disabled in builds — the
  sidecar binary is fully static.
- v0.1 ships single-root workspace support; multi-root is on the
  roadmap.
- Reserved out of scope for this extension (was prior Muninn): publish
  pipeline, graph view, mdbase typed-frontmatter+SQL system.
