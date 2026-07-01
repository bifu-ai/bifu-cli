#!/bin/sh
# bifu-cli installer.
#   curl -fsSL https://cli.bifu.dev/install.sh | bash
#
# Downloads the latest release binary for your OS/arch from GitHub and installs
# it to BIFU_INSTALL_DIR (default: /usr/local/bin, falling back to ~/.local/bin).
# Override version with BIFU_VERSION=v1.2.3.
set -eu

# Public repo that hosts the release binaries (source repo is private).
REPO="decodeex/bifu-cli-releases"
BINARY="bifu-cli"

err() { echo "error: $*" >&2; exit 1; }
info() { echo "$*" >&2; }

# ── Detect OS / arch ─────────────────────────────────────────────────────────
os=$(uname -s)
case "$os" in
  Darwin) os="darwin" ;;
  Linux)  os="linux" ;;
  *) err "unsupported OS: $os (use Homebrew, npm, or download from https://github.com/$REPO/releases)" ;;
esac

arch=$(uname -m)
case "$arch" in
  x86_64 | amd64) arch="amd64" ;;
  arm64 | aarch64) arch="arm64" ;;
  *) err "unsupported architecture: $arch" ;;
esac

# ── Resolve version ──────────────────────────────────────────────────────────
version="${BIFU_VERSION:-}"
if [ -z "$version" ]; then
  version=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | grep '"tag_name":' | head -1 | sed -E 's/.*"([^"]+)".*/\1/')
  [ -n "$version" ] || err "could not determine latest version; set BIFU_VERSION"
fi

# ── Download + extract ───────────────────────────────────────────────────────
asset="${BINARY}_${os}_${arch}.tar.gz"
url="https://github.com/$REPO/releases/download/$version/$asset"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

info "Downloading $BINARY $version ($os/$arch)..."
curl -fsSL "$url" -o "$tmp/$asset" || err "download failed: $url"

# ── Verify checksum ──────────────────────────────────────────────────────────
# GoReleaser publishes checksums.txt alongside the binaries. Verify the
# downloaded archive against it so a tampered/corrupted download is rejected
# rather than installed (BIFU-CLI-202606-008).
sums_url="https://github.com/$REPO/releases/download/$version/checksums.txt"
if curl -fsSL "$sums_url" -o "$tmp/checksums.txt"; then
  expected=$(grep " $asset\$" "$tmp/checksums.txt" | awk '{print $1}' | head -1)
  [ -n "$expected" ] || err "no checksum entry for $asset in checksums.txt"
  if command -v sha256sum >/dev/null 2>&1; then
    actual=$(sha256sum "$tmp/$asset" | awk '{print $1}')
  elif command -v shasum >/dev/null 2>&1; then
    actual=$(shasum -a 256 "$tmp/$asset" | awk '{print $1}')
  else
    err "no sha256 tool (sha256sum/shasum) available to verify the download"
  fi
  [ "$expected" = "$actual" ] || err "checksum mismatch for $asset (expected $expected, got $actual)"
  info "✓ checksum verified"
else
  err "could not download checksums.txt for verification ($sums_url)"
fi

tar -xzf "$tmp/$asset" -C "$tmp" || err "extract failed"
[ -f "$tmp/$BINARY" ] || err "binary not found in archive"
chmod +x "$tmp/$BINARY"

# ── Install ──────────────────────────────────────────────────────────────────
dir="${BIFU_INSTALL_DIR:-/usr/local/bin}"
if [ ! -d "$dir" ] || [ ! -w "$dir" ]; then
  if command -v sudo >/dev/null 2>&1 && [ "$dir" = "/usr/local/bin" ]; then
    info "Installing to $dir (sudo)..."
    sudo mv "$tmp/$BINARY" "$dir/$BINARY" || err "install failed"
  else
    dir="$HOME/.local/bin"
    mkdir -p "$dir"
    mv "$tmp/$BINARY" "$dir/$BINARY"
  fi
else
  mv "$tmp/$BINARY" "$dir/$BINARY"
fi

info ""
info "✓ Installed $BINARY $version to $dir/$BINARY"
case ":$PATH:" in
  *":$dir:"*) ;;
  *) info "  Add $dir to your PATH:  export PATH=\"$dir:\$PATH\"" ;;
esac
info "  Get started:  $BINARY config init --env dev"
