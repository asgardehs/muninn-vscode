# Muninn — VS Code Extension Rewrite (v0.1)

## Context

The existing Muninn at `/home/adam/media/projects/asgard/muninn/` is being nuked and rebuilt as a pure VS Code extension. Today's Muninn shipped quickly without a clear product shape — CLI-first, then HTTP daemon, then mid-flight mdbase adoption. The user (Adam) recognizes this rewrite as "the realistic version that should have been built from day one."

**Product model:** A Dendron clone (Dendron is unmaintained), minus Publishing and graph view — Dendron's two biggest maintenance sinks. Hierarchical dot-path notes, fuzzy lookup palette, schema-driven creation, wikilinks with backlinks.

**Audience:** Adam + his EHS coworkers. Schema engine is generic-core with EHS-flavored examples shipped in the box; non-EHS users can adopt it.

**Architecture:** Go sidecar (heavy lifting) + thin TypeScript VS Code extension (host). Custom JSON-RPC for Dendron-style operations, LSP for what LSP is good at.

A tarball backup of the current `muninn/` is already archived. The local path stays; the GitHub remote moves to `https://github.com/asgardehs/muninn-vscode`.

## Locked decisions

| Decision | Value |
|---|---|
| Repo layout | Monorepo (`/sidecar` Go, `/extension` TS, `/shared` assets) |
| Repo path | Nuke in place at `/home/adam/media/projects/asgard/muninn/` |
| Remote | New GitHub repo `asgardehs/muninn-vscode` |
| Distribution | Platform-targeted `.vsix` bundles; sidecar binary embedded |
| Sidecar transport | Single multiplexed stdio pipe; LSP frames + custom RPC frames distinguished by an extra `Channel:` header |
| LSP scope | Completions, hover, definition, references, diagnostics, code actions, document/workspace symbols |
| RPC scope | Lookup palette, hierarchy queries, schema introspection, lifecycle, fs change notifications, future refactor (v0.2) |
| Heimdall | Off by default (`muninn.useHeimdall: false`); workspace folder is the vault |
| Marketplace publisher | `asgardehs` |
| v0.1 MVP | Hierarchical notes + lookup palette, wikilinks/backlinks, schema engine (generic + EHS examples) |
| Deferred to v0.2 | Refactor-rename with cross-vault backlink updates |
| Out of scope (forever) | Publishing, graph view, mdbase typed-frontmatter+SQL system |

## Repo layout

```
muninn/
  sidecar/                       # Go module: github.com/asgardehs/muninn-sidecar
    cmd/muninn-sidecar/main.go
    internal/
      vault/      markdown/      wikilink/      lsp/
      hierarchy/  schema/        rpc/           transport/   env/
    testdata/vaults/

  extension/                     # VS Code extension
    package.json   tsconfig.json
    src/
      extension.ts
      sidecar/{process,framing,client,lspClient}.ts
      commands/{lookup,createFromHierarchy,createDailyNote}.ts
      views/hierarchyTreeProvider.ts
      config.ts   log.ts
    bin/                         # populated at package time per platform
    test/suite/

  shared/
    schemas/
      generic/{daily,meeting,reference,decision,til}.yml
      ehs/{incident,jha,inspection,training,audit}.yml
    templates/{generic,ehs}/

  docs/{architecture,rpc-protocol,schema-format,contributing,release}.md
  scripts/{build-sidecar,package-vsix,dev}.sh
  .github/workflows/{ci,release}.yml
  .vscode/{launch,tasks}.json
```

## Sidecar architecture

**Process model.** TS extension spawns one `muninn-sidecar` per workspace folder (single-root only in v0.1). Binary resolution: `MUNINN_SIDECAR_PATH` env (dev override) → bundled `extension/bin/`. Crash recovery: exponential backoff, max 3 retries within 60s, then surface notification with "Show Logs" action. Heartbeat every 30s via `rpc/ping`.

**Transport.** One stdio pipe, LSP-style framing extended with one header:

```
Content-Length: 123\r\n
Channel: lsp\r\n          (or "rpc"; absent = lsp, for vscode-languageclient compat)
\r\n
{...json payload...}
```

The Go side reads frames and dispatches to `lsp.Server` or `rpc.Server` based on `Channel`. `vscode-languageclient` emits standard LSP frames untouched. The hand-rolled TS RPC client always sets `Channel: rpc`.

**Startup sequence.** Parse flags → resolve vault path (env → Heimdall if enabled → workspace) → scan `.md`, build `wikilink.Index` and `hierarchy.Tree` → load schemas (`<vault>/.muninn/schemas/*.yml`, fallback to embedded generic) → emit `sidecar/ready` notification with capabilities + version. Extension waits for ready before enabling commands.

## JSON-RPC v0.1 surface

Detailed request/response shapes go in `docs/rpc-protocol.md` (authored as part of Task 1). Method list:

| Method | Purpose |
|---|---|
| `vault/lookup` | Fuzzy lookup over hierarchical names; returns matches with stubs + scores |
| `vault/createFromHierarchy` | Create note at dot-path; optional `schemaId` applies template + frontmatter |
| `vault/listChildren`, `vault/listSiblings`, `vault/listRoots` | Tree-view support |
| `vault/getNote` | Frontmatter + headings + backlinks for a hierarchy name |
| `vault/refresh` | Force re-scan |
| `schema/list`, `schema/get`, `schema/applicableTo`, `schema/validate` | Schema engine queries |
| `rpc/ping`, `rpc/shutdown` | Lifecycle |
| Notifications: `sidecar/ready`, `vault/changed`, `sidecar/log` | Server-initiated |

Errors follow JSON-RPC 2.0: `-32602` invalid params, `-32000` vault, `-32001` schema violation, `-32002` not found.

## Schema engine

**File location.** `<vault>/.muninn/schemas/*.yml`, one schema per file (git-friendly). Embedded defaults at `shared/schemas/generic/` (auto-loaded). EHS pack at `shared/schemas/ehs/` shipped but **opt-in** via `muninn.installSchemaPack` command.

**Schema model** (simplified Dendron):
- `id`, `label`, `description`, `pattern` (glob over dot-path: `*` single segment, `**` recursive), `priority` (tie-break)
- `frontmatter`: list of `{key, type, required, vocabulary?}`. Types: `string`, `string-array`, `date`, `enum`, `number`, `boolean`, `note-ref`.
- `children`: optional pattern→schema constraints
- `template`: `body` + `frontmatterDefaults`, with `{{today}}`, `{{now}}`, `{{user}}`, `{{name}}` variables only — no Turing-complete templating.

**Validation timing.** Debounced (500ms) on change → LSP `publishDiagnostics`; full check on save (note-ref existence, child constraints); on-demand via `schema/validate`. YAML parse errors are surfaced separately and suppress schema diagnostics until YAML parses cleanly.

## VS Code extension surface

**Commands:** `muninn.lookup` (Ctrl+L), `muninn.lookupHere`, `muninn.createFromHierarchy`, `muninn.createDailyNote`, `muninn.gotoNote`, `muninn.installSchemaPack`, `muninn.validateVault`, `muninn.refreshVault`, `muninn.showSidecarLogs`, `muninn.restartSidecar`, `muninn.openVaultPath`.

**Views (activity bar):** `muninn.hierarchyView` (lazy-loaded tree, stubs italicized), `muninn.backlinksView` (active note's backlinks), `muninn.schemaView` (loaded schemas, click to open YAML).

**Settings:** `muninn.vaultPath`, `muninn.useHeimdall` (default `false`), `muninn.sidecarPath`, `muninn.lookupShowsStubs`, `muninn.diagnostics.{unresolvedLinks,schemaViolations}`, `muninn.schemaPacks.autoload` (default `["generic"]`), `muninn.dailyNote.pattern`, `muninn.logLevel`.

**Activation:** `onLanguage:markdown`, `workspaceContains:**/.muninn/`, `workspaceContains:**/*.md`, command activations.

**Language contributions:** Wikilink syntax highlighting via grammar injection into `text.html.markdown` — no new language registration.

## Cherry-pick from old repo

| Source | Destination | Action |
|---|---|---|
| `internal/vault/{vault,list,search,tags}.go` + tests | `sidecar/internal/vault/` | Port; strip `mdbase` import, drop `cfg`/`types` fields, drop `LoadConfig`/`LoadTypes`/`Config`/`Types`/`HasMdbase` methods |
| `internal/wikilink/*` | `sidecar/internal/wikilink/` | Port as-is; rewrite import paths |
| `internal/markdown/{frontmatter,parser,heading}.go` + tests | `sidecar/internal/markdown/` | Port as-is |
| `internal/markdown/schema.go` | — | **Delete.** Salvage `FieldDef`/`Requirement`/`FieldType` types into new `internal/schema/types.go` as the YAML-parsed model |
| `internal/lsp/server.go` | `sidecar/internal/lsp/server.go` | Port; **drop** `stdioRWC` (lines 22-29) and `Run()` (lines 73-79), replace with `ServeOn(stream)` accepting external stream from the mux. Replace `os.Exit(0)` in `exit` handler (line 91) with shutdown signal to App orchestrator. Strip `mdbase` import. From `ExecuteCommandProvider.Commands`: keep `muninn/createNote`, drop `muninn/dailyNote` (moves to extension), drop `muninn/graphLinks` (out of scope). |
| `internal/lsp/{completion,diagnostics}.go` | same | Port; replace `mdbase`/`HasMdbase`/`completeFrontmatterValue` paths with `s.schemas.EnumValuesFor(noteName, fieldName)` |
| `internal/lsp/{definition,references,hover,symbols,tokens,commands,codeactions}.go` + tests | same | Port as-is, import-path rewrite, mdbase strip where present |
| `internal/lsp/codelens.go` | same | Port if non-mdbase; otherwise drop |
| `internal/lsp/rename.go` | same | Port but **don't** advertise `RenameProvider` in capabilities until v0.2 |
| `cmd/muninn/env.go` | `sidecar/internal/env/` | Port `vaultPath()`, `heimdallVaultPath()`, `notesPath()`, `defaultVaultDir`. Add `WorkspaceVault(path)` for the v0.1 default. |
| `internal/mdbase/` (entire) | — | Drop |
| `cmd/muninn/cmd_*.go` (all but env.go), `main.go` | — | Drop |
| `mcpb/`, `vscode/`, `markdownpkm_idea/` | — | Drop |
| `go.mod` | `sidecar/go.mod` | Module name `github.com/asgardehs/muninn-sidecar`. Keep: `go.lsp.dev/{jsonrpc2,protocol,uri}`, `gopkg.in/yaml.v3`, `github.com/asgardehs/heimdall`. Drop: `cobra`, `modernc.org/sqlite`. Add: `github.com/sahilm/fuzzy`, `github.com/fsnotify/fsnotify`. CGO disabled. |

## Implementation phases

### Task 1 — Save spec documentation
Create `docs/specs/2026-05-05-{HHMM}-muninn-vscode-rewrite/` (in the new repo, after the nuke) with:
- `plan.md` — this plan
- `shape.md` — scope, decisions, audience, conversation context
- `references.md` — pointers to old `internal/lsp/server.go`, `internal/vault/vault.go`, `cmd/muninn/env.go`, `internal/wikilink/index.go` (cherry-pick anchors)
- `standards.md` — note that `docs/standards/` did not exist in the old repo; standards will be defined as part of this work
- (no `visuals/` — Dendron's UX is the implicit reference)

### Phase A — Skeleton (extension talks to sidecar)
1. Nuke `/home/adam/media/projects/asgard/muninn/`. Initialize fresh repo, point `origin` at `https://github.com/asgardehs/muninn-vscode` (create GitHub repo first).
2. Monorepo init: `sidecar/go.mod`, `extension/package.json`, `tsconfig.json`, `eslint`, `golangci-lint`, minimal `.github/workflows/ci.yml` (lint + test).
3. `sidecar/internal/transport/`: Content-Length + `Channel:` framing reader/writer with round-trip tests.
4. `sidecar/internal/rpc/`: dispatcher + method registry; implement `rpc/ping`, `sidecar/ready`.
5. `sidecar/cmd/muninn-sidecar/main.go`: parse `--workspace`, log to stderr, exit on stdin EOF.
6. `extension/src/sidecar/{process,framing,client}.ts`: spawn binary, wire stdio, JSON-RPC client. `extension.ts` calls `rpc/ping` on activation.
7. `.vscode/launch.json`: F5 launches dev host with `MUNINN_SIDECAR_PATH` set.

### Phase B — Wikilinks via LSP
8. Cherry-pick `vault/`, `wikilink/`, `markdown/` (mdbase-free) per the table above. Tests pass.
9. Cherry-pick `lsp/server.go` reshaped to `ServeOn(stream)`. Strip mdbase paths in `completion.go`/`diagnostics.go` (gate with `s.schemas` once that exists; for now stub enum completions to empty).
10. Wire `vscode-languageclient` against the sidecar's `lsp` channel (frames without `Channel:` header). Verify wikilink completion + broken-link diagnostics in dev host.
11. Add `references`, `definition`, `hover`, `symbols`, `codeAction` (Create-note quick-fix routing through shared internal create function).
12. `fsnotify` watcher → `vault/changed` notification + LSP-side index refresh.

### Phase C — Hierarchy + lookup
13. `sidecar/internal/hierarchy/`: dot-path tree from `vault.ListNotes()`; `Children/Siblings/Roots/IsStub`. Test against fixture.
14. Implement `vault/lookup` with `sahilm/fuzzy`. Stubs included when `includeStubs=true`.
15. Implement `vault/listChildren`, `listSiblings`, `listRoots`, `getNote`, `createFromHierarchy` (no schema yet).
16. `extension/src/commands/lookup.ts`: `vscode.window.createQuickPick`, debounced `vault/lookup`, `onDidAccept` opens-or-creates. Bind to Ctrl+L.
17. `lookupHere` variant (parent-prefilled).
18. `views/hierarchyTreeProvider.ts`: lazy-loaded tree from `vault/listChildren`; stubs italicized.

### Phase D — Schema engine
19. `sidecar/internal/schema/`: types, YAML parser (yaml.v3), pattern matcher (glob over dot-path), priority-ordered `ApplicableTo`. Tests.
20. `embed.FS` of `shared/schemas/generic/`. Loader: vault `.muninn/schemas/` → embedded fallback.
21. Template renderer (`{{today}}`, `{{now}}`, `{{user}}`, `{{name}}`). Reject unknown variables.
22. Validator: required, type, enum, note-ref existence (via wikilink index).
23. Wire `schema/list`, `schema/get`, `schema/applicableTo`, `schema/validate` RPC methods.
24. Wire `vault/createFromHierarchy` to apply schema template + frontmatter defaults when `schemaId` set.
25. Wire LSP frontmatter-value completion against `schema.EnumValuesFor()`.
26. Wire LSP schema-violation diagnostics (debounced on change, full on save). Distinguish YAML parse errors.
27. Author EHS schema pack (`shared/schemas/ehs/{incident,jha,inspection,training,audit}.yml`). Field semantics in collaboration with Adam.
28. `muninn.installSchemaPack` command: quick-pick over bundled packs, copy YAML into vault.
29. `muninn.createFromHierarchy` command: schema picker filtered by `schema/applicableTo`.

### Phase E — Polish + first release
30. Settings JSON schema, defaults polish, walkthroughs (`contributes.walkthroughs`).
31. Output channel formatter; `muninn.showSidecarLogs`, `muninn.restartSidecar`.
32. Backlinks tree view (vault-wide query via RPC).
33. Heartbeat + crash-recovery (max 3 retries, exponential backoff, error notification).
34. README, CHANGELOG, LICENSE, screenshots.
35. `scripts/build-sidecar.sh` cross-compile matrix (linux-x64, linux-arm64, darwin-x64, darwin-arm64, win32-x64; CGO disabled).
36. `scripts/package-vsix.sh` per platform; `vsce package --target <vscode-target>`.
37. `.github/workflows/release.yml`: on `v*.*.*` tag, lint+test → 5-platform build matrix → `vsce publish` per platform → GitHub release with all 5 .vsix as assets.
38. Marketplace publish under `asgardehs`. Tag `v0.1.0`.

## Open risks (revisit during implementation)

1. **Multi-root workspaces.** v0.1 single-root only; document the limitation. Defer to v0.3.
2. **Vault path config change mid-session.** `onDidChangeConfiguration` triggers sidecar respawn; surface "Reload required" notification.
3. **Concurrent writes to shared in-memory state.** Single `sync.RWMutex` on the App owning vault+wikilink-index+schemas. Revisit if write-heavy ops starve LSP responsiveness.
4. **Capability negotiation.** Bundled binary makes mismatch unlikely; keep capability list as forward-compat insurance, not gating.
5. **YAML-mid-edit diagnostic flicker.** Suppress schema diagnostics when YAML doesn't parse; surface YAML errors separately at higher severity.
6. **Heimdall as implicit-state risk.** Default off (locked decision). Workspace = vault is the path of least surprise.
7. **Schema priority ambiguity.** Ties resolved by file name; surface chosen schema in `schemaView` so users see which won.
8. **Lookup performance on large vaults.** Build hierarchy + fuzzy index during startup before `sidecar/ready`; incrementally update on `vault/changed`.

## Critical files

Old codebase (cherry-pick anchors):
- /home/adam/media/projects/asgard/muninn/internal/vault/vault.go — strip mdbase, port
- /home/adam/media/projects/asgard/muninn/internal/lsp/server.go — transport reshape (stdioRWC + Run + os.Exit)
- /home/adam/media/projects/asgard/muninn/internal/lsp/completion.go — schema-engine swap
- /home/adam/media/projects/asgard/muninn/internal/wikilink/index.go — concurrent state, locking design grounds here
- /home/adam/media/projects/asgard/muninn/cmd/muninn/env.go — 3-tier vault resolver

New codebase (created during execution):
- sidecar/cmd/muninn-sidecar/main.go
- sidecar/internal/transport/framing.go
- sidecar/internal/rpc/dispatcher.go
- sidecar/internal/lsp/server.go (ported)
- sidecar/internal/hierarchy/tree.go
- sidecar/internal/schema/{types,parser,matcher,validator,template}.go
- sidecar/internal/env/vault_path.go
- extension/src/extension.ts
- extension/src/sidecar/{process,framing,client,lspClient}.ts
- extension/src/commands/lookup.ts
- shared/schemas/generic/*.yml + shared/schemas/ehs/*.yml
- .github/workflows/release.yml

## Verification

**Phase A done when:**
- `go test ./sidecar/...` passes; `npm test` in `extension/` passes.
- F5 launches Extension Development Host with locally built sidecar.
- `Output → Muninn` shows the `rpc/ping` round-trip on activation.

**Phase B done when:**
- Open a sample vault in dev host: typing `[[` triggers wikilink completions from the vault's notes.
- Broken `[[nonexistent]]` shows a Problems-panel diagnostic.
- "Go to Definition" on a wikilink jumps to the target.
- Editing a note updates the Problems panel within ~500ms.

**Phase C done when:**
- Ctrl+L opens a quick-pick; typing `foo.bar` shows matches and stubs ranked by fuzzy score.
- Selecting a stub creates the file (and any parent stubs).
- Hierarchy tree view in the activity bar shows the dot-path tree, lazy-expanding.

**Phase D done when:**
- A vault with `.muninn/schemas/incident.yml` shows the schema in `schemaView`.
- `Cmd+P → muninn.createFromHierarchy → ehs.incidents.test` offers `incident` as the default schema.
- Creating from that schema emits a file with required frontmatter + template body.
- Editing the file to violate a required field surfaces a Problems-panel diagnostic.
- `muninn.installSchemaPack → ehs` copies all five EHS YAMLs into `<vault>/.muninn/schemas/`.

**Phase E done when:**
- `git tag v0.1.0 && git push --tags` produces 5 `.vsix` artifacts in the GitHub release.
- `code --install-extension muninn-linux-x64-0.1.0.vsix` works on a clean machine.
- The Marketplace listing under `asgardehs` shows the platform-specific install metadata.
- A first-time user with no `.muninn/` in their workspace can: open a folder, type Ctrl+L, create `daily.2026-05-05`, see it in the hierarchy tree, hover a `[[wikilink]]`, and get a broken-link diagnostic — all without manual setup.
