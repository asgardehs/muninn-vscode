# Release Standard

Every Muninn release is driven by a single annotated git tag of the form
`vMAJOR.MINOR.PATCH` (semver). Pushing the tag fires
`.github/workflows/release.yml`, which cross-compiles the sidecar for five
VS Code platform targets, packages a per-platform `.vsix`, publishes each
to the [Open VSX Registry](https://open-vsx.org/) under the `asgardehs`
namespace, and attaches all five `.vsix` files to a matching GitHub
Release.

We currently publish only to Open VSX (used by VSCodium, Cursor, Theia,
Gitpod, code-server, and the VS Code Marketplace's open-source mirror).
Microsoft's VS Code Marketplace publication via `vsce publish` requires
an Azure DevOps organization that we don't maintain; users on stock
VS Code can install our `.vsix` directly from the GitHub Release page or
opt into Open VSX as their extension gallery.

## Versioning

We follow [Semantic Versioning](https://semver.org/). The version in
`extension/package.json` is the source of truth; the git tag must match.

- `0.x.y` — Pre-1.0 development. Breaking RPC or schema changes are
  permitted in MINOR bumps but should be called out in `CHANGELOG.md`.
- `1.0.0` — First stable release. After this, RPC protocol changes are
  MAJOR bumps and require an upgrade path in
  `docs/standards/json-rpc-protocol.md`.

The CHANGELOG follows [Keep a Changelog](https://keepachangelog.com/) format.

## Required secrets

| Name      | Scope    | Purpose                                                                 |
|-----------|----------|-------------------------------------------------------------------------|
| `VSX_PAT` | Org-wide | Open VSX Personal Access Token for the `asgardehs` namespace. Used by `ovsx publish`. |

`GH_TOKEN` is supplied automatically as `${{ github.token }}` for the
GitHub Release step; no setup needed.

The workflow reads `VSX_PAT` from the secret store and exposes it to
the `ovsx` CLI as `OVSX_PAT` (the env var name `ovsx` looks for).
Org-level secrets under `asgardehs` are inherited by this repo, so you
don't need to set anything per-repo.

### Provisioning the VSX_PAT

1. Sign in to https://open-vsx.org with the Eclipse account that owns
   the `asgardehs` namespace.
2. Profile menu → Settings → Access Tokens → Generate new token.
3. Copy the token value. (Open VSX shows it only once.)
4. Add it as an org-level GitHub secret: org settings → Secrets and
   variables → Actions → New organization secret → name `VSX_PAT`,
   value the token. Repository access: select repos that need it
   (currently `muninn-vscode`).
5. Rotate annually.

## Local dry run

Before tagging, package every target locally:

```sh
make vsix-all
```

The five `.vsix` files in `dist/vsix/` should be byte-identical to what
CI will produce for the same commit (CGO disabled, `-trimpath`, version
embedded via `git describe`).

You can sanity-check a single package by installing it into your daily
VS Code (or VSCodium / Cursor):

```sh
code --install-extension dist/vsix/muninn-linux-x64-X.Y.Z.vsix
```

## Tagging the release

1. Land all merges intended for the release on `main`.
2. Bump `extension/package.json` `version` to the target.
3. Update `CHANGELOG.md` — move the `## Unreleased` section under a new
   `## [X.Y.Z] - YYYY-MM-DD` heading.
4. Commit with message `chore: release X.Y.Z` and push to `main`.
5. Tag and push: `git tag -a vX.Y.Z -m "vX.Y.Z" && git push origin vX.Y.Z`.
6. Watch the Actions tab. The matrix runs ~5 minutes; publish is another
   minute or two. The Open VSX listing typically appears within a minute
   of the publish job finishing.

## Recovery

A failed publish leaves a partial GitHub Release behind. Open VSX rejects
re-publishing the same version, so you must bump the patch number rather
than re-tagging the same number. Steps:

```sh
gh release delete vX.Y.Z --yes
git push --delete origin vX.Y.Z
git tag -d vX.Y.Z
```

Fix the underlying issue, bump to `vX.Y.(Z+1)` in `extension/package.json`
and `CHANGELOG.md`, commit, then start again from step 4 above.
