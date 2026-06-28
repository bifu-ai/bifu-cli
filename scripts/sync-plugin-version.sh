#!/usr/bin/env bash
# Set the "version" field in every plugin/extension manifest to <version>, so
# the Claude Code / Codex plugins and the Claude Desktop .mcpb all track the
# release tag. Run before cutting a release: scripts/sync-plugin-version.sh 1.1.12
# (a leading "v" is stripped). Only the value of a `"version": "..."` key is
# touched — `manifest_version` is left alone.
set -euo pipefail
cd "$(dirname "$0")/.."

V="${1:?usage: sync-plugin-version.sh <version>}"
V="${V#v}"

files=(
  plugins/bifu/.claude-plugin/plugin.json
  plugins/bifu/.codex-plugin/plugin.json
  plugins/claude-desktop/manifest.json
  .claude-plugin/marketplace.json
)
for f in "${files[@]}"; do
  VERSION="$V" perl -i -pe 's/("version"\s*:\s*)"[^"]*"/$1 . "\"" . $ENV{VERSION} . "\""/e' "$f"
  echo "set version $V in $f"
done
