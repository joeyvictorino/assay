#!/usr/bin/env sh
# Fails if any tracked or working-tree file names an organization, product or
# requisition that must never appear in this repository. Run before every push.
#
# The pattern itself is not stored in the repository, because listing the
# names here would publish them. It is read from, in order:
#   1. the NAME_AUDIT_PATTERN environment variable (a repository secret in CI)
#   2. ${XDG_CONFIG_HOME:-$HOME/.config}/name-audit/pattern (one extended regex)
# With neither, the audit fails (exit 2), unless NAME_AUDIT_ALLOW_MISSING=1,
# which CI sets only for pull requests from forks, where secrets are absent.
set -eu
cd "$(dirname "$0")/.."
PATTERN="${NAME_AUDIT_PATTERN:-}"
CONF="${XDG_CONFIG_HOME:-$HOME/.config}/name-audit/pattern"
if [ -z "$PATTERN" ] && [ -r "$CONF" ]; then
  PATTERN=$(head -n 1 "$CONF")
fi
if [ -z "$PATTERN" ]; then
  if [ "${NAME_AUDIT_ALLOW_MISSING:-0}" = "1" ]; then
    echo "name-audit: SKIPPED (no pattern configured; allowed for this run)"
    exit 0
  fi
  echo "name-audit: ERROR (set NAME_AUDIT_PATTERN or create $CONF)" >&2
  exit 2
fi
if git grep -q -I -i -E "$PATTERN" -- . ':!scripts/name-audit.sh' ; then
  # Print only file:line, never the matched text, so CI logs do not echo it.
  git grep -n -I -i -E "$PATTERN" -- . ':!scripts/name-audit.sh' | cut -d: -f1,2 >&2
  echo "name-audit: FAIL (matches at the locations above)" >&2
  exit 1
fi
echo "name-audit: OK"
