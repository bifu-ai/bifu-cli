#!/usr/bin/env bash
# Build self-contained Claude Desktop extensions (.mcpb) — one per platform,
# each bundling the matching bifu-cli binary under server/ so Claude Desktop
# runs it without relying on the system PATH.
#
# Usage: scripts/build-mcpb.sh [VERSION] [OUTDIR] [PLATFORMS]
#   VERSION    default: git describe
#   OUTDIR     default: dist/mcpb
#   PLATFORMS  default: the 6 release targets (space-separated os/arch)
set -euo pipefail

cd "$(dirname "$0")/.."
VERSION="${1:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
OUTDIR="${2:-dist/mcpb}"
PLATFORMS="${3:-darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64}"
SRC="plugins/claude-desktop"

mkdir -p "$OUTDIR"
for p in $PLATFORMS; do
  os="${p%/*}"; arch="${p#*/}"
  bin="bifu-cli"; [ "$os" = "windows" ] && bin="bifu-cli.exe"
  stage="$(mktemp -d)"
  mkdir -p "$stage/server"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
    go build -ldflags "-s -w -X bifu-cli/cmd.version=${VERSION#v}" -o "$stage/server/$bin" .
  # Point the manifest at the platform's binary name (Windows is .exe).
  sed "s#server/bifu-cli#server/$bin#g" "$SRC/manifest.json" > "$stage/manifest.json"
  npx -y @anthropic-ai/mcpb@latest pack "$stage" "$OUTDIR/bifu_${os}_${arch}.mcpb" >/dev/null
  rm -rf "$stage"
  echo "built $OUTDIR/bifu_${os}_${arch}.mcpb"
done
echo "Done — $(ls -1 "$OUTDIR"/*.mcpb | wc -l | tr -d ' ') .mcpb bundle(s) in $OUTDIR"
