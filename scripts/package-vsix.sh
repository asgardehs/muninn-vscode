#!/usr/bin/env bash
# Build the sidecar for one platform target and package the extension as a
# platform-specific .vsix that bundles that binary.
#
# Usage: package-vsix.sh <vscode-target>
#   vscode-target ∈ {linux-x64, linux-arm64, darwin-x64, darwin-arm64, win32-x64}
# Output: dist/vsix/muninn-<vscode-target>-<version>.vsix
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <vscode-target>" >&2
  echo "  one of: linux-x64 linux-arm64 darwin-x64 darwin-arm64 win32-x64" >&2
  exit 2
fi

target="$1"

case "$target" in
  linux-x64)    goos=linux;   goarch=amd64 ;;
  linux-arm64)  goos=linux;   goarch=arm64 ;;
  darwin-x64)   goos=darwin;  goarch=amd64 ;;
  darwin-arm64) goos=darwin;  goarch=arm64 ;;
  win32-x64)    goos=windows; goarch=amd64 ;;
  *) echo "unknown target: $target" >&2; exit 2 ;;
esac

repo_root="$(cd "$(dirname "$0")/.." && pwd)"

# 1. Build the sidecar for the target platform.
"$repo_root/scripts/build-sidecar.sh" "$goos" "$goarch"

# 2. Stage the binary into extension/bin/. Empty the directory first so a
#    previous run for a different target can't leak into this .vsix.
ext=""
if [[ "$goos" == "windows" ]]; then
  ext=".exe"
fi
src_bin="$repo_root/sidecar/dist/sidecar/$goos-$goarch/muninn-sidecar$ext"
dest_dir="$repo_root/extension/bin"

rm -rf "$dest_dir"
mkdir -p "$dest_dir"
cp "$src_bin" "$dest_dir/muninn-sidecar$ext"
chmod +x "$dest_dir/muninn-sidecar$ext"

# 3. Compile the TypeScript and package.
out_dir="$repo_root/dist/vsix"
mkdir -p "$out_dir"

cd "$repo_root/extension"
npm run compile

version="$(node -p "require('./package.json').version")"
out_vsix="$out_dir/muninn-$target-$version.vsix"

# npx --yes uses the local devDependency without prompting if it isn't
# installed yet (CI or fresh clone).
npx --yes @vscode/vsce package \
  --target "$target" \
  --out "$out_vsix"

printf 'packaged %s\n' "$out_vsix"
