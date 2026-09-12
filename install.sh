#!/usr/bin/env sh
# curl -fsSL https://linkwise.app/install.sh | sh
#
# Resolves the latest release rather than hard-coding a version, so this file
# is written once and does not need touching per release.
set -eu

REPO="LinkwiseApp/linkwise-cli"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$arch" in
  x86_64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) echo "Unsupported architecture: $arch" >&2; exit 1 ;;
esac

case "$os" in
  darwin|linux) ;;
  *) echo "Unsupported system: $os. On Windows use: npm install -g @linkwise/cli" >&2; exit 1 ;;
esac

tag=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
  | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1)
[ -n "$tag" ] || { echo "Could not find the latest release" >&2; exit 1; }

asset="linkwise_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/$tag"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

curl -fsSL "$base/$asset" -o "$tmp/$asset"
curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt"

# Verified before anything is unpacked. A pipe-to-shell installer that skips
# this is a pipe-to-shell installer worth being afraid of.
expected=$(grep " $asset\$" "$tmp/checksums.txt" | awk '{print $1}')
if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$tmp/$asset" | awk '{print $1}')
else
  actual=$(shasum -a 256 "$tmp/$asset" | awk '{print $1}')
fi
[ "$expected" = "$actual" ] || { echo "Checksum mismatch. Not installing." >&2; exit 1; }

tar -xzf "$tmp/$asset" -C "$tmp"

if [ -w /usr/local/bin ]; then
  dest=/usr/local/bin
else
  dest="$HOME/.local/bin"
  mkdir -p "$dest"
fi

install -m 755 "$tmp/linkwise" "$dest/linkwise"
echo "Installed linkwise $tag to $dest"

case ":$PATH:" in
  *":$dest:"*) ;;
  *) echo "Add $dest to your PATH to use it." ;;
esac
