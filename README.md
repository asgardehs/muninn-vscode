# Muninn

A hierarchical notes VS Code extension. Dot-path note names, fuzzy lookup, schema-driven creation, wikilinks with
backlinks.

> Heavy lifting in a Go sidecar; thin TypeScript glue for VS Code. No database, no telemetry, plain markdown on disk.

## For users

The full feature tour lives in [extension/README.md](extension/README.md). Once v0.1.0 ships you can install Muninn from
the [Open VSX Registry](https://open-vsx.org/extension/pharomwinters/muninn) (VSCodium, Cursor, Theia, Gitpod,
code-server) or sideload a `.vsix` from the [GitHub Releases](https://github.com/asgardehs/muninn-vscode/releases) page
on stock VS Code.

## For contributors

Setup, conventions, and how to land a change: [CONTRIBUTING.md](CONTRIBUTING.md).

Release cadence and the tag-driven publishing pipeline: [docs/standards/release.md](docs/standards/release.md).

Design specs: [docs/specs/](docs/specs/).

## Repository layout

- [`sidecar/`](sidecar/) — Go binary. Vault I/O, wikilink index, hierarchy tree, schema engine, LSP server, custom
  JSON-RPC server.
- [`extension/`](extension/) — VS Code extension (TypeScript). Spawns the sidecar, hosts UI commands and tree views.
- [`scripts/`](scripts/) — build + packaging.
- [`Makefile`](Makefile) — task runner; canonical place to look up commands.
- [`CHANGELOG.md`](CHANGELOG.md) — release notes per [Keep a Changelog](https://keepachangelog.com/).

## License

[GPL-3.0](LICENSE).

_Part of the [Asgard EHS](https://asgardehs.github.io/) family._
