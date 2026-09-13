#!/usr/bin/env bash
# Exercises install.sh's guards, which are the part that can ruin somebody's
# machine and the part no Go test can reach.
#
# Nothing here touches the network: every case is decided before a download
# would start, and the one case that gets that far is pointed at a URL that
# does not resolve.
set -uo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
INSTALL="$HERE/../install.sh"

pass=0
fail=0

ok() { echo "  ok: $1"; pass=$((pass + 1)); }
bad() { echo "  FAIL: $1" >&2; fail=$((fail + 1)); }

# run <expected-exit> <description> -- runs install.sh in a sandbox whose PATH
# holds only what the case sets up. Output and status come back in OUT/STATUS.
run() {
  OUT=$(PATH="$SANDBOX/bin:/usr/bin:/bin" \
    LINKWISE_INSTALL_BASE="http://127.0.0.1:1/releases" \
    HOME="$SANDBOX/home" \
    sh "$INSTALL" 2>&1)
  STATUS=$?
}

setup() {
  SANDBOX=$(mktemp -d)
  mkdir -p "$SANDBOX/bin" "$SANDBOX/home"
}

teardown() { rm -rf "$SANDBOX"; }

# A Homebrew install is a symlink into the Caskroom. Overwriting it leaves
# brew certain it has installed a file that is no longer there.
echo "a homebrew install"
setup
mkdir -p "$SANDBOX/Caskroom/linkwise/0.1.1"
touch "$SANDBOX/Caskroom/linkwise/0.1.1/linkwise"
chmod +x "$SANDBOX/Caskroom/linkwise/0.1.1/linkwise"
ln -s "$SANDBOX/Caskroom/linkwise/0.1.1/linkwise" "$SANDBOX/bin/linkwise"
run
[ "$STATUS" -ne 0 ] && ok "refuses to overwrite it" || bad "overwrote a Homebrew install"
case "$OUT" in
  *"brew upgrade"*) ok "names the brew command instead" ;;
  *) bad "does not name the brew command: $OUT" ;;
esac
teardown

echo "an npm install"
setup
mkdir -p "$SANDBOX/lib/node_modules/@linkwise/cli/bin"
touch "$SANDBOX/lib/node_modules/@linkwise/cli/bin/linkwise"
chmod +x "$SANDBOX/lib/node_modules/@linkwise/cli/bin/linkwise"
ln -s "$SANDBOX/lib/node_modules/@linkwise/cli/bin/linkwise" "$SANDBOX/bin/linkwise"
run
[ "$STATUS" -ne 0 ] && ok "refuses to overwrite it" || bad "overwrote an npm install"
case "$OUT" in
  *"npm install -g @linkwise/cli@latest"*) ok "names the npm command instead" ;;
  *) bad "does not name the npm command: $OUT" ;;
esac
teardown

# Switching channels deliberately has to stay possible.
echo "the force override"
setup
mkdir -p "$SANDBOX/Caskroom/linkwise/0.1.1"
touch "$SANDBOX/Caskroom/linkwise/0.1.1/linkwise"
ln -s "$SANDBOX/Caskroom/linkwise/0.1.1/linkwise" "$SANDBOX/bin/linkwise"
OUT=$(PATH="$SANDBOX/bin:/usr/bin:/bin" \
  LINKWISE_INSTALL_BASE="http://127.0.0.1:1/releases" \
  LINKWISE_INSTALL_FORCE=1 \
  HOME="$SANDBOX/home" \
  sh "$INSTALL" 2>&1)
case "$OUT" in
  *"brew upgrade"*) bad "the override did not get past the guard: $OUT" ;;
  *) ok "gets past the guard" ;;
esac
teardown

# A plain existing binary is what this installer wrote last time, and
# replacing it is the whole job.
echo "an existing shell install"
setup
touch "$SANDBOX/bin/linkwise"
chmod +x "$SANDBOX/bin/linkwise"
run
case "$OUT" in
  *"brew upgrade"*|*"npm install"*) bad "sent a shell install to another channel: $OUT" ;;
  *) ok "proceeds to the download" ;;
esac
teardown

# An unreachable network must say so, rather than carrying on and unpacking
# whatever an empty file turns out to be.
echo "an unreachable release page"
setup
run
[ "$STATUS" -ne 0 ] && ok "fails rather than installing nothing" || bad "claimed success with no download"
case "$OUT" in
  *"tar"*|*"cannot"*|*"Could not"*|*"not find"*) ok "says the release lookup failed" ;;
  *) bad "unhelpful failure: $OUT" ;;
esac
teardown

echo
if [ "$fail" -gt 0 ]; then
  echo "$fail failed, $pass passed"
  exit 1
fi
echo "$pass passed"
