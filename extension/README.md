# Muninn

**Hierarchical markdown notes for VS Code — fuzzy lookup, schema-driven creation, wikilinks with backlinks.**

Muninn turns a folder of `.md` files into a Dendron-style personal knowledge base inside VS Code. Files are named with dot-paths (`projects.alpha.kickoff.md`), navigated through a fuzzy lookup palette, linked together with `[[wikilinks]]`, and validated against schemas you write or install.

> Plain markdown on disk. No database. No telemetry. Your notes stay yours.

## Quick start

1. Open a folder in VS Code — that folder *is* your vault.
2. Press <kbd>Ctrl</kbd>+<kbd>L</kbd> (<kbd>⌘</kbd>+<kbd>L</kbd> on macOS) to open the lookup palette.
3. Type a name like `projects.alpha.kickoff` and hit Enter — Muninn creates the note with sensible frontmatter and opens it.
4. Inside any note, type `[[` to autocomplete a wikilink to another note. Broken links surface as Problems-panel diagnostics.
5. Open the **Muninn** view in the activity bar to browse the hierarchy tree.

## Features

### Hierarchical notes

Filenames become the hierarchy. `projects.md` is the parent of `projects.alpha.md` is the parent of `projects.alpha.kickoff.md`. Missing intermediates show up in the tree as italicized stubs you can click to create.

### Lookup palette

`Muninn: Lookup` (<kbd>Ctrl</kbd>+<kbd>L</kbd>) is a fuzzy quick-pick over every note name. The current input is offered as a "Create new note" entry whenever it doesn't exactly match an existing note — the same command navigates and creates.

`Muninn: Lookup Under Current Note` prefills with the active note's parent path, so adding a sibling is two keystrokes.

### Wikilinks (LSP-powered)

- `[[completion]]` for note names and heading fragments.
- Go-to-definition on a wikilink jumps to the target.
- Find-all-references shows backlinks across the vault.
- Hover previews the linked note.
- Broken-link diagnostics in the Problems panel; updates within ~100ms when files change on disk.

### Schema engine

Schemas declare which frontmatter fields a note should have based on its dot-path. They live in `<vault>/.muninn/schemas/*.yml` (one schema per file).

```yaml
# .muninn/schemas/incident.yml
id: incident
label: EHS Incident Report
pattern: "ehs.incidents.**"
priority: 100

frontmatter:
  - key: title
    type: string
    required: true
  - key: incident_date
    type: date
    required: true
  - key: severity
    type: enum
    required: true
    vocabulary: [near-miss, first-aid, recordable, lost-time, fatality]

template:
  body: |
    ## What happened

    ## Root cause

    ## Corrective actions
    - [ ]
  frontmatterDefaults:
    incident_date: "{{today}}"
    severity: near-miss
```

When you create `ehs.incidents.2026-05-06-something`, Muninn applies this schema's template, fills the defaults, and validates required fields. Missing fields appear as Problems-panel diagnostics; enum values get autocomplete inside frontmatter.

### Bundled schema packs

Run **Muninn: Install Schema Pack** to copy a curated pack into your vault:

- `generic` — daily, meeting, reference, decision, til
- `ehs` — incident, JHA, inspection, training, audit (Environmental Health & Safety)

Custom schemas you write live alongside installed packs and override them on `id` collision. The bundled `generic` pack is also loaded automatically when you have no `.muninn/schemas/` directory yet, so a fresh vault has sensible defaults from minute one.

Drop a YAML into `.muninn/schemas/` from any text editor — Muninn picks it up live without a reload.

## Commands

| Command | Default keybinding | Purpose |
|---|---|---|
| `Muninn: Lookup` | <kbd>Ctrl</kbd>+<kbd>L</kbd> / <kbd>⌘</kbd>+<kbd>L</kbd> | Fuzzy lookup or create. |
| `Muninn: Lookup Under Current Note` | — | Lookup prefilled with the active note's parent path. |
| `Muninn: Refresh Vault Index` | — | Re-scan after external changes. |
| `Muninn: Install Schema Pack` | — | Copy a bundled schema pack into the vault. |
| `Muninn: Show Sidecar Logs` | — | Open the output channel. |
| `Muninn: Ping Sidecar` | — | Round-trip health check. |

## Settings

- `muninn.diagnostics.unresolvedLinks` — Show diagnostics for unresolved `[[wikilinks]]` and missing heading fragments. *(default: true)*
- `muninn.sidecarPath` — Override the path to the bundled `muninn-sidecar` binary. Empty = use the binary shipped with the extension. *(advanced / development)*
- `muninn.logLevel` — Log verbosity for the Muninn output channel: `error`, `warn`, `info`, or `debug`. *(default: info)*

## Architecture

Muninn is two pieces wired through one stdio pipe:

- A **Go sidecar** (`muninn-sidecar`) handles vault I/O, the wikilink index, the hierarchy tree, the schema engine, and the LSP server. It's a static binary, no native dependencies.
- A **TypeScript extension host** (this package) spawns the sidecar, hosts the lookup palette and tree view, and bridges VS Code's `vscode-languageclient` to the sidecar's LSP channel. It's deliberately thin.

The two protocols (LSP for editor-shaped requests, custom JSON-RPC for everything else) share one stdio pipe via a `Channel:` header on top of LSP framing. No databases, no embeddings, no network calls.

## Related projects

Muninn is part of the Asgard EHS ecosystem. The architecture is inspired by [Dendron](https://github.com/dendronhq/dendron) (currently unmaintained) and [Foam](https://foambubble.github.io/foam/), with a deliberately narrower scope than either.

## License

[GPL-3.0](LICENSE).

## Installing

- **VSCodium / Cursor / Theia / Gitpod / code-server**: install directly
  from the [Open VSX Registry](https://open-vsx.org/extension/asgardehs/muninn).
- **Stock VS Code**: download the matching `muninn-<platform>-<version>.vsix`
  from the [GitHub Releases page](https://github.com/asgardehs/muninn-vscode/releases)
  and run `code --install-extension <path>`.

We do not currently publish to the Microsoft VS Code Marketplace.

## Source

[github.com/asgardehs/muninn-vscode](https://github.com/asgardehs/muninn-vscode)
