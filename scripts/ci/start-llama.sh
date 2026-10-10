#!/usr/bin/env bash
# start-llama.sh [LOCK_FILE] [WORK_DIR]
# Downloads the pinned llama.cpp Linux x64 release and the pinned GGUF model
# listed in labs/models.lock, verifies sha256 digests, starts llama-server on
# 127.0.0.1:8081 (LLAMA_PORT) and waits for /health. LLAMA_CTX sets the
# context size (default 32768) and LLAMA_THREADS the thread count (default 4).
#
# Hash policy: every sha256 in the lock file must match the downloaded file.
# An EMPTY hash is accepted only when ASSAY_FIRST_RUN=1; the computed digest
# is then printed so it can be committed into labs/models.lock. Without that
# variable an empty hash fails the job, so hashes cannot stay unpinned.
set -euo pipefail
LOCK="${1:-labs/models.lock}"
WORK="${2:-${RUNNER_TEMP:-/tmp}/assay-llama}"
PORT="${LLAMA_PORT:-8081}"
CTX="${LLAMA_CTX:-32768}"
mkdir -p "$WORK/release" "$WORK/models"

lockval() { sed -n "s/^$1:[[:space:]]*\"\{0,1\}\([^\"#]*\)\"\{0,1\}.*/\1/p" "$LOCK" | head -n1 | tr -d '[:space:]'; }
RELEASE=$(lockval llama_cpp_release)
ASSET=$(lockval llama_cpp_asset)
ASSET_SHA=$(lockval llama_cpp_sha256)
MODEL_REPO=$(lockval model_repo)
MODEL_FILE=$(lockval model_file)
MODEL_SHA=$(lockval model_sha256)
FB_REPO=$(lockval fallback_model_repo)
FB_FILE=$(lockval fallback_model_file)
FB_SHA=$(lockval fallback_model_sha256)
[ -n "$RELEASE" ] && [ -n "$ASSET" ] && [ -n "$MODEL_REPO" ] && [ -n "$MODEL_FILE" ] || { echo "start-llama: lock file $LOCK is incomplete" >&2; exit 2; }

sha256() { if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | cut -d' ' -f1; else shasum -a 256 "$1" | cut -d' ' -f1; fi; }

# verify FILE EXPECTED KEY -> exits non-zero on mismatch or unpinned (unless first run)
verify() {
  local file="$1" want="$2" key="$3" got
  got=$(sha256 "$file")
  if [ -z "$want" ]; then
    echo "start-llama: $key is empty; computed sha256 for $(basename "$file"): $got"
    if [ "${ASSAY_FIRST_RUN:-0}" = "1" ]; then
      echo "start-llama: ASSAY_FIRST_RUN=1, accepting and printing the hash to pin:"
      echo "  $key: \"$got\""
      return 0
    fi
    echo "start-llama: refusing unpinned asset; set $key in $LOCK (or ASSAY_FIRST_RUN=1 to bootstrap)" >&2
    return 1
  fi
  if [ "$got" != "$want" ]; then
    echo "start-llama: sha256 mismatch for $(basename "$file"): got $got want $want" >&2
    return 1
  fi
  echo "start-llama: verified $(basename "$file")"
}

fetch() { curl -fL --retry 3 --retry-delay 5 --max-time 1800 -o "$2" "$1"; }

# 1. llama.cpp release asset.
ASSET_PATH="$WORK/release/$ASSET"
if [ ! -s "$ASSET_PATH" ]; then
  echo "start-llama: downloading llama.cpp $RELEASE/$ASSET"
  fetch "https://github.com/ggml-org/llama.cpp/releases/download/$RELEASE/$ASSET" "$ASSET_PATH"
fi
verify "$ASSET_PATH" "$ASSET_SHA" llama_cpp_sha256
EXTRACT="$WORK/release/extract"
rm -rf "$EXTRACT" && mkdir -p "$EXTRACT"
case "$ASSET" in
  *.tar.gz|*.tgz) tar -xzf "$ASSET_PATH" -C "$EXTRACT" ;;
  *.zip) unzip -q "$ASSET_PATH" -d "$EXTRACT" ;;
  *) echo "start-llama: unknown archive type $ASSET" >&2; exit 2 ;;
esac
SERVER=$(find "$EXTRACT" -type f -name llama-server | head -n1)
[ -n "$SERVER" ] || { echo "start-llama: llama-server not found in $ASSET" >&2; exit 1; }
chmod +x "$SERVER"
export LD_LIBRARY_PATH="$(dirname "$SERVER"):$(dirname "$SERVER")/../lib:${LD_LIBRARY_PATH:-}"

# 2. Model, with fallback to the smaller pinned model.
download_model() {
  local repo="$1" file="$2" sha="$3" key="$4"
  local path="$WORK/models/$file"
  if [ ! -s "$path" ]; then
    echo "start-llama: downloading $repo/$file"
    fetch "https://huggingface.co/$repo/resolve/main/$file" "$path" || return 1
  fi
  verify "$path" "$sha" "$key" || return 1
  MODEL_PATH="$path"
}
MODEL_PATH=""
if ! download_model "$MODEL_REPO" "$MODEL_FILE" "$MODEL_SHA" model_sha256; then
  echo "start-llama: primary model unavailable; trying fallback $FB_REPO/$FB_FILE" >&2
  [ -n "$FB_REPO" ] && [ -n "$FB_FILE" ] || exit 1
  download_model "$FB_REPO" "$FB_FILE" "$FB_SHA" fallback_model_sha256
fi

# 3. Start the server and wait for health.
LOG="$WORK/llama-server.log"
echo "start-llama: starting llama-server on 127.0.0.1:$PORT with $(basename "$MODEL_PATH")"
nohup "$SERVER" --host 127.0.0.1 --port "$PORT" -c "$CTX" -t "${LLAMA_THREADS:-4}" --jinja -m "$MODEL_PATH" >"$LOG" 2>&1 &
echo $! > "$WORK/llama-server.pid"
"$(dirname "$0")/wait-for.sh" "http://127.0.0.1:$PORT/health" "${LLAMA_WAIT_SECONDS:-300}" 200 || { tail -n 50 "$LOG" >&2; exit 1; }
echo "start-llama: ready (pid $(cat "$WORK/llama-server.pid"), log $LOG)"
