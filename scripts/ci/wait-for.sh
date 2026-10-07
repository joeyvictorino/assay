#!/usr/bin/env sh
# wait-for.sh URL [TIMEOUT_SECONDS] [EXPECTED_STATUS]
# Polls URL with curl until it answers with the expected HTTP status
# (default: any 2xx/3xx) or the timeout (default 120s) elapses.
# Loopback lab targets only; this script is a CI helper, not a probe.
set -eu
URL="${1:?usage: wait-for.sh URL [TIMEOUT_SECONDS] [EXPECTED_STATUS]}"
TIMEOUT="${2:-120}"
EXPECT="${3:-}"
case "$URL" in
  http://127.0.0.1:*|http://localhost:*|http://127.0.0.1/*|http://localhost/*) ;;
  *) echo "wait-for: refusing non-loopback url $URL" >&2; exit 2 ;;
esac
start=$(date +%s)
while :; do
  code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 "$URL" || true)
  if [ -n "$EXPECT" ]; then
    [ "$code" = "$EXPECT" ] && { echo "wait-for: $URL -> $code"; exit 0; }
  else
    case "$code" in 2*|3*) echo "wait-for: $URL -> $code"; exit 0 ;; esac
  fi
  now=$(date +%s)
  if [ $((now - start)) -ge "$TIMEOUT" ]; then
    echo "wait-for: timeout after ${TIMEOUT}s waiting for $URL (last status: ${code:-none})" >&2
    exit 1
  fi
  sleep 2
done
