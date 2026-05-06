# Standards for Muninn VS Code Extension Rewrite

## Inventory

`docs/standards/` did not exist in the prior Muninn repo. There is no pre-existing standards catalog to inherit. The shape-spec workflow's "surface relevant standards" step is a no-op for this rewrite.

## Standards to define as part of this work

These will be authored under `docs/standards/` during implementation. They are not blockers for Phase A, but should land before public release (Phase E).

- **`docs/standards/json-rpc-protocol.md`** — versioning rules, error code reservations, breaking-change policy for the custom RPC channel. Drives compatibility between sidecar versions and extension versions.
- **`docs/standards/schema-format.md`** — YAML schema file format reference: pattern grammar, field types, vocabulary syntax, template variable list. Authoritative reference for users authoring `.muninn/schemas/*.yml`.
- **`docs/standards/release.md`** — semver discipline, changelog format ([Keep a Changelog](https://keepachangelog.com/)), platform-targeted vsix release procedure, marketplace publish checklist.
- **`docs/standards/contributing.md`** — Conventional Commits, PR conventions, test requirements (unit + integration), how to bump the protocol version.
- **`docs/standards/testing.md`** — fixture vault layout under `sidecar/testdata/vaults/`, naming conventions for integration tests, race-detector requirements (always on in CI).

## Style references

- **Keep a Changelog** (https://keepachangelog.com/) — for `CHANGELOG.md`.
- **Contributor Covenant** (https://www.contributor-covenant.org/) — for `CODE_OF_CONDUCT.md` if we add one.
- **Conventional Commits** (https://www.conventionalcommits.org/) — commit message style.

(Adam values referencing established public standards as a transparency signal — link them rather than reinvent.)
