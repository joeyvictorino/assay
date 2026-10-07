#!/usr/bin/env sh
# Fails if any tracked or working-tree file names an organization, product or
# requisition that must never appear in this repository. Run before every push.
set -eu
cd "$(dirname "$0")/.."
PATTERN='qompute|qomputeai|\bqore\b|palo alto|paloaltonetworks|unit 42|unit42|requisition'
if git grep -n -I -i -E "$PATTERN" -- . ':!scripts/name-audit.sh' ; then
  echo "name-audit: FAIL (matches above)" >&2
  exit 1
fi
echo "name-audit: OK"
