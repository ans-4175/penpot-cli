#!/bin/sh
# Install penpot-cli from the latest GitHub Release (macOS / Linux).
# No sudo: installs to ~/.local/bin unless INSTALL_DIR is set.
#   curl -fsSL https://raw.githubusercontent.com/ans-4175/penpot-cli/main/install.sh | sh
# Env: INSTALL_DIR=<dir>  PENPOT_CLI_VERSION=v2.6.2 (default: latest)
set -eu

REPO="ans-4175/penpot-cli"
BIN="penpot-cli"
DIR="${INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${PENPOT_CLI_VERSION:-latest}"

die() { echo "install.sh: $*" >&2; exit 1; }

case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) die "unsupported OS '$(uname -s)'. Windows: download the .zip from https://github.com/$REPO/releases" ;;
esac
case "$(uname -m)" in
  arm64|aarch64) arch=arm64 ;;
  x86_64|amd64) arch=amd64 ;;
  *) die "unsupported CPU '$(uname -m)'" ;;
esac
# Shell running under Rosetta on Apple Silicon reports x86_64.
if [ "$os" = darwin ] && [ "$arch" = amd64 ] && [ "$(sysctl -n sysctl.proc_translated 2>/dev/null || echo 0)" = 1 ]; then
  arch=arm64
fi

name="$BIN-$os-$arch.tar.gz"
if [ "$VERSION" = latest ]; then
  base="https://github.com/$REPO/releases/latest/download"
else
  base="https://github.com/$REPO/releases/download/$VERSION"
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "Downloading $name ($VERSION)..."
curl -fsSL -o "$tmp/$name" "$base/$name" || die "download failed: $base/$name"
curl -fsSL -o "$tmp/SHA256SUMS.txt" "$base/SHA256SUMS.txt" || die "cannot fetch SHA256SUMS.txt"

want="$(grep " $name\$" "$tmp/SHA256SUMS.txt" | awk '{print $1}')"
[ -n "$want" ] || die "$name not listed in SHA256SUMS.txt"
if command -v sha256sum >/dev/null 2>&1; then
  got="$(sha256sum "$tmp/$name" | awk '{print $1}')"
else
  got="$(shasum -a 256 "$tmp/$name" | awk '{print $1}')"
fi
[ "$want" = "$got" ] || die "checksum mismatch (expected $want, got $got)"

tar -xzf "$tmp/$name" -C "$tmp"
mkdir -p "$DIR"
mv "$tmp/$BIN" "$DIR/$BIN"
chmod 755 "$DIR/$BIN"
# Only matters for browser downloads, harmless here.
[ "$os" = darwin ] && xattr -d com.apple.quarantine "$DIR/$BIN" 2>/dev/null || true

echo "Installed: $DIR/$BIN"
"$DIR/$BIN" version
case ":$PATH:" in
  *":$DIR:"*) ;;
  *) echo "NOTE: $DIR is not on your PATH. Add: export PATH=\"$DIR:\$PATH\"" ;;
esac
