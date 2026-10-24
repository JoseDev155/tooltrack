#!/usr/bin/env sh
# Builds static tooltrack binaries into dist/ without needing make.
# Usage: scripts/build.sh [target ...]
# Targets: linux-amd64 windows-amd64 darwin-amd64 darwin-arm64 (default: all)
set -eu

cd "$(dirname "$0")/.."

APP_NAME=tooltrack
DIST_DIR=dist
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"

if [ "$#" -eq 0 ]; then
  set -- linux-amd64 windows-amd64 darwin-amd64 darwin-arm64
fi

mkdir -p "$DIST_DIR"

for target in "$@"; do
  case "$target" in
    linux-amd64 | windows-amd64 | darwin-amd64 | darwin-arm64) ;;
    *)
      echo "unknown target: $target" >&2
      echo "valid targets: linux-amd64 windows-amd64 darwin-amd64 darwin-arm64" >&2
      exit 2
      ;;
  esac
  goos="${target%-*}"
  goarch="${target#*-}"
  out="$DIST_DIR/$APP_NAME-$target"
  if [ "$goos" = "windows" ]; then
    out="$out.exe"
  fi
  echo "building $out ($VERSION)"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -ldflags="-s -w -X 'main.Version=$VERSION'" -o "$out" ./cmd/cli
done
