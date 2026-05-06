# Release Standard

Every Muninn release is driven by a single annotated git tag of the form
`vMAJOR.MINOR.PATCH` (semver). Pushing the tag fires
`.github/workflows/release.yml`, which cross-compiles the sidecar for five
VS Code platform targets, packages a per-platform `.vsix`, publishes each
to the marketplace under the `asgardehs` publisher, and attaches all five
`.vsix` files to a matching GitHub Release.

## Versioning

We follow [Semantic Versioning](https://semver.org/). The version in
`extension/package.json` is the source of truth; the git tag must match.

- `0.x.y` — Pre-1.0 development. Breaking RPC or schema changes are
  permitted in MINOR bumps but should be called out in `CHANGELOG.md`.
- `1.0.0` — First stable release. After this, RPC protocol changes are
  MAJOR bumps and require an upgrade path in `docs/standards/json-rpc-protocol.md`.

The CHANGELOG follows [Keep a Changelog](https://keepachangelog.com/) format.

## Required GitHub repo secrets

| Name        | Purpose                                                  |
|-------------|----------------------------------------------------------|
| `VSCE_PAT`  | Azure DevOps Personal Access Token with Marketplace (Manage) scope for the `asgardehs` publisher. Used by `vsce publish`. |

`GH_TOKEN` is supplied automatically as `${{ github.token }}` for the
GitHub Release step; no setup needed.

### Provisioning the VSCE_PAT

1. Sign in to https://dev.azure.com under the Microsoft account that owns
   the `asgardehs` publisher.
2. User Settings → Personal Access Tokens → New Token.
3. Organization: All accessible organizations. Scopes: custom defined →
   Marketplace → Manage.
4. Copy the token, then in GitHub: Settings → Secrets and variables →
   Actions → New repository secret → name `VSCE_PAT`.
5. Rotate annually.

## Local dry run

Before tagging, package every target locally:

```sh
make vsix-all
```

The five `.vsix` files in `dist/vsix/` should be byte-identical to what CI
will produce for the same commit (CGO disabled, `-trimpath`, version
embedded via `git describe`).

You can sanity-check a single package by installing it into your daily
VS Code:

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
   minute or two.

## Recovery

A failed publish leaves a partial GitHub Release behind. Before
re-tagging:

```sh
gh release delete vX.Y.Z --yes
git push --delete origin vX.Y.Z
git tag -d vX.Y.Z
```

Fix the underlying issue, then start again from step 4 above.
