#!/usr/bin/env bash
# Cross-compile the muninn-sidecar Go binary for one platform target.
#
# Usage: build-sidecar.sh <goos> <goarch>
# Output: sidecar/dist/sidecar/<goos>-<goarch>/muninn-sidecar[.exe]
#
# CGO is disabled so we produce a static binary on every platform — the
# heimdall transitive dependency on modernc.org/sqlite is pure Go, so this
# works without a C toolchain.
set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "usage: $0 <goos> <goarch>" >&2
  echo "  e.g. $0 linux amd64" >&2
  exit 2
fi

goos="$1"
goarch="$2"
ext=""
if [[ "$goos" == "windows" ]]; then
  ext=".exe"
fi

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
out_dir="$repo_root/sidecar/dist/sidecar/$goos-$goarch"
out_bin="$out_dir/muninn-sidecar$ext"

mkdir -p "$out_dir"

version="$(git -C "$repo_root" describe --tags --always --dirty 2>/dev/null || echo dev)"

cd "$repo_root/sidecar"
CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build \
  -trimpath \
  -ldflags "-s -w -X main.version=$version" \
  -o "$out_bin" \
  ./cmd/muninn-sidecar

size=$(stat -c %s "$out_bin" 2>/dev/null || stat -f %z "$out_bin")
printf 'built %s (%s bytes, version %s)\n' "$out_bin" "$size" "$version"
