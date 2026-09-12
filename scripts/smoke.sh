#!/usr/bin/env bash
# Runs the real surface against the live API. Needs LINKWISE_TOKEN set to a
# read-and-write key, and it saves and then deletes one link.
#
# Not part of `go test`: it needs a credential, it costs quota, and it would
# fail in CI for reasons that say nothing about the change under test.
set -euo pipefail

BIN=${BIN:-./linkwise}
: "${LINKWISE_TOKEN:?set LINKWISE_TOKEN to a read-and-write key}"

fail() { echo "FAIL: $*" >&2; exit 1; }
ok() { echo "  ok: $*"; }

# Runs a command that is expected to fail, and checks the exit code it failed
# with. Written as a function because `cmd || [ $? -eq N ]` reads the status of
# the test, not the command, once a pipeline gets involved.
expect_exit() {
  local want=$1; shift
  local got=0
  "$@" >/dev/null 2>&1 || got=$?
  [ "$got" -eq "$want" ] || fail "$* exited $got, want $want"
}

echo "auth"
$BIN auth status --json | jq -e '.account' >/dev/null || fail "auth status"
ok "auth status names an account"

echo "exit codes"
LINKWISE_TOKEN=lw_pat_definitely_wrong expect_exit 3 $BIN ls
ok "a bad key exits 3"

expect_exit 5 $BIN read 00000000-0000-0000-0000-000000000000
ok "a missing link exits 5"

expect_exit 2 $BIN search x --mode semantic
ok "an unaccepted mode exits 2"

echo "output detection"
$BIN ls --limit 2 | head -1 | jq -e '.id' >/dev/null || fail "a pipe should produce NDJSON"
ok "a pipe produces NDJSON"

echo "round trip"
# A full library exits 7, which is the account being full rather than anything
# wrong with the binary. Reported and skipped instead of failing the run.
saved=$($BIN save "https://example.com/linkwise-smoke-$RANDOM" --json 2>/dev/null) || save_status=$?
if [ "${save_status:-0}" -eq 7 ]; then
  echo "  skipped: this account is at its link limit, so save and rm were not exercised"
elif [ "${save_status:-0}" -ne 0 ]; then
  fail "save exited ${save_status}"
else
  id=$(printf '%s' "$saved" | jq -r .id)
  [ -n "$id" ] || fail "save returned no id"
  ok "saved $id"
  $BIN rm "$id" >/dev/null
  ok "deleted it again"
fi

echo "feeds export"
$BIN feeds export | head -1 | grep -q '^<?xml' || fail "export should start with an XML declaration"
ok "export is XML and nothing else"

echo "all passed"
