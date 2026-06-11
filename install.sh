#!/bin/sh
# bifu-cli installer.
#   curl -fsSL https://cli.bifu.dev/install.sh | bash
#
# Downloads the latest release binary for your OS/arch from GitHub and installs
# it to BIFU_INSTALL_DIR (default: /usr/local/bin, falling back to ~/.local/bin).
# Override version with BIFU_VERSION=v1.2.3.
set -eu

REPO="decodeex/bifu-cli"
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
