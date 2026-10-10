#!/usr/bin/env bash
# start-ensemble.sh WORK_DIR LOCK [LOCK...]
# Starts one llama-server per lock file, on consecutive ports from
# LLAMA_BASE_PORT (default 8081), by running start-llama.sh for each lock
# with its own work directory. Every asset is sha256-verified by
# start-llama.sh; any failure stops the script (fail closed). The llama.cpp
# release archive is downloaded once and copied into each work directory,
# where start-llama.sh verifies it again.
#
# Models are served side by side; the harness calls them one at a time, so
# the four runner cores are not shared between concurrent generations.
# LLAMA_CTX defaults to 16384 here (prompts in committed runs stay under
# 5k tokens) to keep three KV caches well inside the runner's memory.
set -euo pipefail
[ "$#" -ge 2 ] || { echo "usage: start-ensemble.sh WORK_DIR LOCK [LOCK...]" >&2; exit 2; }
WORK="$1"; shift
HERE="$(cd "$(dirname "$0")" && pwd)"
port="${LLAMA_BASE_PORT:-8081}"
export LLAMA_CTX="${LLAMA_CTX:-16384}"
first_release=""
for lock in "$@"; do
  [ -f "$lock" ] || { echo "start-ensemble: no lock file $lock" >&2; exit 2; }
  name="$(basename "$lock" .lock)"
  dir="$WORK/$name"
  asset="$(sed -n 's/^llama_cpp_asset:[[:space:]]*\([^[:space:]#]*\).*/\1/p' "$lock" | head -n1)"
  mkdir -p "$dir/release"
  if [ -n "$first_release" ] && [ ! -s "$dir/release/$asset" ]; then
    cp "$first_release" "$dir/release/$asset"
  fi
  echo "start-ensemble: $name on port $port"
  LLAMA_PORT="$port" bash "$HERE/start-llama.sh" "$lock" "$dir"
  first_release="$dir/release/$asset"
  port=$((port + 1))
done
echo "start-ensemble: $# servers ready"
