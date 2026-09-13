#!/usr/bin/env sh
# curl -fsSL https://linkwise.app/install.sh | sh
#
# Resolves the latest release rather than hard-coding a version, so this file
# is written once and does not need touching per release.
#
# Environment:
#   LINKWISE_INSTALL_DIR    where to put the binary, overriding the default
#   LINKWISE_INSTALL_FORCE  install even over another channel's install
#   LINKWISE_INSTALL_BASE   another release host, for tests and mirrors
set -eu

REPO="LinkwiseApp/linkwise-cli"
BASE="${LINKWISE_INSTALL_BASE:-https://github.com/$REPO/releases}"

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

# resolve follows a chain of symlinks to the real file. readlink -f would be
# one line, but it is a GNU extension that macOS did not ship until Monterey
# and this script has to run on whatever is there.
resolve() {
  p=$1
  n=0
  while [ -L "$p" ] && [ "$n" -lt 20 ]; do
    link=$(readlink "$p")
    case "$link" in
      /*) p=$link ;;
      *) p=$(dirname "$p")/$link ;;
    esac
    n=$((n + 1))
  done
  printf '%s\n' "$p"
}

# Where the user's shell would find linkwise today. Empty on a first install.
existing=$(command -v linkwise 2>/dev/null || true)
existing_real=""
[ -n "$existing" ] && existing_real=$(resolve "$existing")

# Homebrew and npm each own the file they put on disk. Writing over a
# Caskroom symlink leaves brew certain it has installed something that is no
# longer there, and the next `brew upgrade` silently undoes this install. So
# an install managed by another channel is updated through that channel.
if [ -z "${LINKWISE_INSTALL_FORCE:-}" ] && [ -n "$existing_real" ]; then
  case "$existing_real" in
    */Caskroom/*|*/Cellar/*)
      echo "linkwise is already installed with Homebrew:" >&2
      echo "  $existing" >&2
      echo >&2
      echo "Update it with:" >&2
      echo "  brew upgrade --cask linkwiseapp/tap/linkwise" >&2
      echo >&2
      echo "To replace it with this installer anyway, set LINKWISE_INSTALL_FORCE=1." >&2
      exit 1
      ;;
    */node_modules/*)
      echo "linkwise is already installed with npm:" >&2
      echo "  $existing" >&2
      echo >&2
      echo "Update it with:" >&2
      echo "  npm install -g @linkwise/cli@latest" >&2
      echo >&2
      echo "To replace it with this installer anyway, set LINKWISE_INSTALL_FORCE=1." >&2
      exit 1
      ;;
  esac
fi

# The newest tag, read from the redirect the release page answers with.
#
# Not api.github.com: that endpoint is rate limited to 60 requests an hour per
# IP unauthenticated, so behind an office NAT or on a CI runner it starts
# answering 403 and the installer fails for reasons nobody can see. The
# website's redirect is unmetered, and the tag is the last path segment of
# where it lands.
tag=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "$BASE/latest" 2>/dev/null | sed 's#.*/##') || tag=""
case "$tag" in
  v*) ;;
  *) echo "Could not find the latest release at $BASE/latest" >&2; exit 1 ;;
esac

asset="linkwise_${os}_${arch}.tar.gz"
download="$BASE/download/$tag"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

curl -fsSL "$download/$asset" -o "$tmp/$asset" || {
  echo "Could not download $download/$asset" >&2
  exit 1
}
curl -fsSL "$download/checksums.txt" -o "$tmp/checksums.txt" || {
  echo "Could not download $download/checksums.txt" >&2
  exit 1
}

# Verified before anything is unpacked. A pipe-to-shell installer that skips
# this is a pipe-to-shell installer worth being afraid of.
expected=$(grep " $asset\$" "$tmp/checksums.txt" | awk '{print $1}')
if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$tmp/$asset" | awk '{print $1}')
else
  actual=$(shasum -a 256 "$tmp/$asset" | awk '{print $1}')
fi
[ -n "$expected" ] || { echo "No checksum published for $asset. Not installing." >&2; exit 1; }
[ "$expected" = "$actual" ] || { echo "Checksum mismatch. Not installing." >&2; exit 1; }

tar -xzf "$tmp/$asset" -C "$tmp"

# Where to put it. An install that is already there wins, because that is the
# file the user's shell resolves today: picking somewhere else by rule would
# leave two binaries on the PATH, the older one shadowing the new, and an
# update that appears to have done nothing.
if [ -n "${LINKWISE_INSTALL_DIR:-}" ]; then
  dest=$LINKWISE_INSTALL_DIR
  mkdir -p "$dest"
elif [ -n "$existing_real" ] && [ -w "$(dirname "$existing_real")" ]; then
  dest=$(dirname "$existing_real")
elif [ -w /usr/local/bin ]; then
  dest=/usr/local/bin
else
  dest="$HOME/.local/bin"
  mkdir -p "$dest"
fi

install -m 755 "$tmp/linkwise" "$dest/linkwise"
echo "Installed linkwise $tag to $dest"

case ":$PATH:" in
  *":$dest:"*) ;;
  *)
    echo >&2
    echo "$dest is not on your PATH. Add it to use linkwise:" >&2
    echo "  export PATH=\"$dest:\$PATH\"" >&2
    exit 0
    ;;
esac

# An older copy earlier on the PATH is the reason an update looks like it did
# nothing, and it is invisible unless somebody says so.
found=$(command -v linkwise 2>/dev/null || true)
if [ -n "$found" ] && [ "$(resolve "$found")" != "$dest/linkwise" ]; then
  echo >&2
  echo "Warning: another linkwise comes first on your PATH:" >&2
  echo "  $found" >&2
  echo "Remove it, or the version you just installed will not be the one that runs." >&2
fi
