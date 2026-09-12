#!/usr/bin/env bash
# The spec is authored in the edge repo, where the API lives. This copy exists
# so the drift test can run without that checkout, and so CI has something to
# check against.
#
# Converted to JSON on the way in: Go reads JSON from the standard library and
# a YAML dependency for one test is a dependency for the whole binary.
set -euo pipefail

SRC="${1:-../linkwise-edge-function/docs/api/openapi.yaml}"

if [ ! -f "$SRC" ]; then
  echo "No spec at $SRC. Pass the path explicitly: scripts/sync-openapi.sh <path>" >&2
  exit 1
fi

npx --yes js-yaml "$SRC" > spec/openapi.json
echo "Synced spec/openapi.json from $SRC"
