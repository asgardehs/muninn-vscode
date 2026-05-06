# Muninn VS Code Extension Rewrite — Shaping Notes

## Scope

Rewrite Muninn as a pure VS Code extension modeled on Dendron (which is unmaintained), minus Publishing and graph view. The previous Muninn (CLI + LSP + HTTP daemon + mdbase) was archived to `~/media/projects/asgard/.archive/muninn.tar.gz` and the GitHub repo removed; the new repo is `asgardehs/muninn-vscode`.

Architecture: Go sidecar handles all heavy lifting (vault I/O, wikilink index, hierarchy tree, schema engine, LSP server, custom JSON-RPC server). VS Code extension is a thin TypeScript host that spawns the sidecar and provides UI surface (commands, tree views, settings).

## Decisions

- **Audience:** Adam + EHS coworkers primary; schema engine kept generic enough for non-EHS adoption. EHS-flavored schema pack ships in the box but is opt-in.
- **Repo layout:** Monorepo with `/sidecar`, `/extension`, `/shared`, `/docs`, `/scripts`. Same local path as before; new GitHub remote.
- **Distribution:** Platform-targeted `.vsix` bundles via `vsce --target`. Sidecar binary embedded per platform (`extension/bin/`). Five targets: linux-x64, linux-arm64, darwin-x64, darwin-arm64, win32-x64. CGO disabled for static binaries.
- **Transport:** Single multiplexed stdio pipe. LSP frames + custom JSON-RPC frames distinguished by an extra `Channel:` header. Frames without a Channel default to LSP, so vscode-languageclient interoperates unchanged.
- **LSP scope:** Completions (wikilinks, headings, frontmatter values), hover, definition, references, diagnostics, code actions, document/workspace symbols.
- **RPC scope:** Lookup palette, hierarchy queries, schema introspection, lifecycle (`rpc/ping`, `rpc/shutdown`), filesystem change notifications, future refactor (v0.2).
- **Heimdall:** OFF by default. Workspace folder = vault. Heimdall integration available behind `muninn.useHeimdall: true` for Adam's own use.
- **Marketplace publisher:** `asgardehs`.
- **License:** GPL-3.0 (carried forward from prior Muninn).
- **v0.1 MVP:** Hierarchical notes + lookup palette, wikilinks/backlinks, schema engine (generic + EHS examples).
- **Deferred to v0.2:** Refactor-rename (rename note → update all wikilinks across vault).
- **Forever out of scope (in this product):** Publishing, graph view, mdbase typed-frontmatter+SQL system. (mdbase remains reserved for a hypothetical future *desktop* Muninn.)

## Context

- **Audience:** Adam (Asgard EHS developer) + coworkers; secondary: anyone who wants a Dendron-style PKM in VS Code.
- **Visuals/References:** None provided. Dendron's UX (lookup palette, schema-driven creation, hierarchy tree) is the implicit conceptual reference.
- **Code references:** See `references.md`. Prior Muninn at the archived tarball is the cherry-pick source.
- **Product alignment:** This rewrite supersedes the existing `docs/design.md` and `docs/mdbase-adoption.md` from the prior Muninn (now archived). Those are not migrated forward.

## Standards Applied

`docs/standards/` did not exist in the prior repo. See `standards.md` — standards will be defined as part of this work, not adopted from a prior catalog.

## Provenance

- Plan written and approved 2026-05-05.
- Plan file (working copy): `/home/adam/.claude/plans/serialized-leaping-lerdorf.md`.
- Plan file (in repo): `./plan.md` (this folder).
- Phase 1 exploration (portability map) and Phase 2 architecture (Plan agent) drove the design; Phase 3 verified the LSP transport-detachability claim by inspecting the prior `internal/lsp/server.go`.
