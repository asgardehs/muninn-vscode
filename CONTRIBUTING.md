# Contributing to Muninn

Thanks for being interested. The contracts below keep the project moving;
the spec under `docs/specs/` is the source of truth for design decisions.

## Local setup

You'll need:

- Go 1.26+
- Node 22+
- VS Code 1.95+

```sh
git clone https://github.com/asgardehs/muninn-vscode.git
cd muninn-vscode
make build       # compiles sidecar + extension
make test        # go test -race + tsc --noEmit
```

To launch the dev host, open the repo in VS Code and press <kbd>F5</kbd>.
The launch task rebuilds the sidecar binary and recompiles the extension
TypeScript before starting the Extension Development Host. The dev host
opens the `sidecar/testdata/vaults/example` fixture vault automatically.

## Layout

- `sidecar/` — Go module. Vault I/O, wikilink index, hierarchy tree,
  schema engine, LSP server, RPC dispatcher, transport multiplexer.
- `extension/` — TypeScript VS Code extension. Spawns the sidecar,
  hosts UI commands and tree views, bridges `vscode-languageclient`.
- `docs/specs/` — design specs, one folder per substantive change.
- `docs/standards/` — repo conventions (release process, coming soon:
  json-rpc-protocol, schema-format, contributing).
- `scripts/` — build + packaging helpers.
- `Makefile` — task runner; the canonical place to look up commands.

## Conventional Commits

We follow [Conventional Commits](https://www.conventionalcommits.org/).
The most common types we use:

- `feat(<scope>)` — new user-facing capability.
- `fix(<scope>)` — bug fix.
- `chore(<scope>)` — build, deps, scaffolding.
- `docs(<scope>)` — README, specs, comments.
- `refactor(<scope>)` — internal restructuring with no behavior change.
- `test(<scope>)` — adding or fixing tests.

`<scope>` is usually a phase identifier (`phase-c`) for in-progress
work, or a package/component name (`vault`, `lsp`, `extension`).

## Branching

`main` is the trunk. Feature branches off `main`, PR back to `main`.
Force-pushing `main` is forbidden.

## Tests

Sidecar packages should hold above 80% line coverage on new code; run
`go test ./... -race -count=1` before PR. The race detector is required
because LSP and RPC handlers share state through the App struct.

For the extension TypeScript, `npm run typecheck` (which is
`tsc --noEmit`) gates PRs. Integration tests via
`@vscode/test-electron` are on the roadmap.

## Schemas and the JSON-RPC protocol

Both surface user-facing contracts:

- `<vault>/.muninn/schemas/*.yml` files in the wild expect the format
  documented in `docs/specs/.../shape.md`.
- The custom JSON-RPC channel (everything outside LSP) is documented
  in `docs/specs/.../plan.md` §3.

Breaking changes to either require:

1. A spec update (new `docs/specs/.../` folder).
2. A migration note in `CHANGELOG.md`.
3. A MAJOR version bump after we hit 1.0.

Pre-1.0 we permit breaking changes in MINOR releases; please call them
out clearly in the changelog.

## Filing issues

Bug reports should include:

- Muninn version (visible in `Muninn: Show Sidecar Logs` startup line).
- VS Code version + platform.
- A minimal vault that reproduces the issue (or a description of one).
- The relevant excerpt from the **Muninn** and **Muninn LSP** output
  channels — both at debug level if possible (`muninn.logLevel` →
  `debug`).
