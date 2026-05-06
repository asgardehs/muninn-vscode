# References for Muninn VS Code Extension Rewrite

## Cherry-pick anchors (from archived prior Muninn)

The previous Muninn codebase lives in `~/media/projects/asgard/.archive/muninn.tar.gz`. Extract for reference; do not import as a Go module.

### `internal/vault/`

- **Location (in archive):** `muninn/internal/vault/{vault,list,search,tags}.go` + tests
- **Relevance:** Direct port. Provides `Vault` with `ListNotes()`, `ReadNote()`, `CreateNote()`, `Search()`, `ListFiltered()`, `CollectTags()`. Clean stdlib dependencies.
- **Action:** Port to `sidecar/internal/vault/`. Strip `mdbase` import, drop `cfg`/`types` fields, drop `LoadConfig`/`LoadTypes`/`Config`/`Types`/`HasMdbase` methods. The mdbase coupling was gated by a `.muninn/` config check on init; remove that path entirely.

### `internal/wikilink/`

- **Location:** `muninn/internal/wikilink/{extract,index}.go` + tests
- **Relevance:** Pure regex `Extract()` + threadsafe in-memory `Index`. Self-contained; no edits needed beyond import-path rewrite.
- **Action:** Port as-is.

### `internal/markdown/`

- **Location:** `muninn/internal/markdown/{frontmatter,parser,heading}.go` + tests
- **Relevance:** Frontmatter parsing (yaml.v3), heading extraction. Light mdbase coupling in `schema.go` only.
- **Action:** Port `frontmatter.go`, `parser.go`, `heading.go` as-is. Delete `schema.go` (mdbase-coupled hardcoded schema); salvage `FieldDef`/`Requirement`/`FieldType` types into the new `internal/schema/types.go` as the YAML-parsed model.

### `internal/lsp/server.go` — transport reshape

- **Location:** `muninn/internal/lsp/server.go`, lines 22-29 (`stdioRWC`) and lines 73-79 (`Run()`).
- **Relevance:** Architecturally critical. `Run()` instantiates `jsonrpc2.NewStream(stdioRWC{})` and binds the server to the process's stdio. The `handle()` dispatcher (lines 82-134) and all method handlers are transport-agnostic — they take `jsonrpc2.Replier` and `jsonrpc2.Request`.
- **Verified 2026-05-05:** Detachable. Replace `Run()` with `ServeOn(stream jsonrpc2.Stream)` that accepts an externally provided stream from the multiplexer. Drop `stdioRWC`. Replace `os.Exit(0)` in the `exit` handler (line 91) with a graceful shutdown signal back to the App orchestrator so the multiplexer can flush both channels.
- **Note:** `ExecuteCommandProvider.Commands` (line 179) lists `"muninn/createNote"`, `"muninn/dailyNote"`, `"muninn/graphLinks"`. Keep the first; drop `dailyNote` (moves to extension); drop `graphLinks` (graph view is out of scope).

### `internal/lsp/{completion,diagnostics}.go` — schema engine swap

- **Location:** `muninn/internal/lsp/completion.go`, `muninn/internal/lsp/diagnostics.go`
- **Relevance:** Core LSP behavior. mdbase coupling at `s.vault.HasMdbase()` checks and `completeFrontmatterValue` paths.
- **Action:** Port; replace mdbase paths with `s.schemas.EnumValuesFor(noteName, fieldName)` once the schema engine exists. Phase B can stub these to empty pending the schema engine in Phase D.

### `internal/wikilink/index.go` — concurrent state

- **Location:** `muninn/internal/wikilink/index.go`
- **Relevance:** Threadsafe `Index` with internal `sync.RWMutex`. Will be shared between LSP handlers and RPC handlers in the new sidecar.
- **Locking design:** A single `sync.RWMutex` on the App struct that owns vault+index+schemas; all LSP and RPC handlers acquire it for composite operations (read note → extract links → write index). The index's own mutex stays for atomic single-key access.

### `cmd/muninn/env.go` — vault path resolver

- **Location:** `muninn/cmd/muninn/env.go`
- **Relevance:** Three-tier vault path resolution: `MUNINN_VAULT_PATH` env var → Heimdall config (`muninn.vault_path`) → platform default `~/.local/share/muninn`. Heimdall integration via `heimdallVaultPath()`.
- **Action:** Port functions `vaultPath()`, `heimdallVaultPath()`, `notesPath()`, and the `defaultVaultDir` constant to `sidecar/internal/env/`. Add `WorkspaceVault(workspacePath string)` that returns the workspace folder as the v0.1 default. Heimdall lookup is gated behind the `muninn.useHeimdall` setting (off by default).

## Prior Muninn product docs (archived, NOT migrated forward)

- `muninn/docs/design.md` — described the CLI + LSP + mdbase architecture. Superseded.
- `muninn/docs/mdbase-adoption.md` — Phase 1/2 design for typed frontmatter + SQL queries. Reserved for hypothetical future *desktop* Muninn; not part of this rewrite.
- `muninn/docs/chores.md` — old maintenance notes. Not relevant.

## External references

- **Dendron** (https://github.com/dendronhq/dendron, unmaintained) — UX conceptual reference. Hierarchical lookup palette, schema-driven creation, tree view.
- **vscode-languageclient** — The TS extension uses this for the LSP channel. It speaks plain LSP framing (no `Channel:` header), which is why the sidecar treats absent-channel as LSP.
- **go.lsp.dev/jsonrpc2 + protocol + uri** — LSP server-side primitives. Already present in the archived go.mod.
- **sahilm/fuzzy** — fuzzy-match library for the lookup palette (added in Phase C).
- **fsnotify/fsnotify** — vault watcher (added in Phase B).
