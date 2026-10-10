#!/usr/bin/env bash
# 60-second demo of assay. Offline: no API key, no model, no outbound network.
#
#   ./scripts/demo.sh
#
# It builds the harness and the synthetic-ops lab (a deliberately vulnerable
# service with 15 planted weaknesses), starts the lab on 127.0.0.1:9000, runs
# the harness against it with the scripted "fake" provider, prints a short
# summary and checks the audit log, including after a deliberate edit. Needs
# Go 1.26 or newer and curl. The first build downloads Go modules if they are
# not cached; everything after that is local. Nothing is left running or on
# disk when it ends (the temporary directory and the audit log are removed).
#
# Port 9000 is fixed because scope/ci.yaml, the signed scope document, only
# authorizes 127.0.0.1 ports 3000, 8080 and 9000.
set -euo pipefail

cd "$(dirname "$0")/.."
START=$(date +%s)
WORK=$(mktemp -d "${TMPDIR:-/tmp}/assay-demo.XXXXXX")
RUN_ID="demo-$$"
LAB_PID=""

cleanup() {
  [ -n "$LAB_PID" ] && kill "$LAB_PID" 2>/dev/null || true
  rm -rf "$WORK" ".audit/$RUN_ID.log"
  rmdir .audit 2>/dev/null || true
}
trap cleanup EXIT

if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then BOLD=$'\033[1m'; DIM=$'\033[2m'; OFF=$'\033[0m'; else BOLD=""; DIM=""; OFF=""; fi
step() { printf '\n%s%s%s\n' "$BOLD" "$*" "$OFF"; }
note() { printf '%s%s%s\n' "$DIM" "$*" "$OFF"; }
die() { echo "demo: $*" >&2; exit 1; }

command -v go >/dev/null || die "Go is required (https://go.dev/dl/, version 1.26 or newer)"
command -v curl >/dev/null || die "curl is required"
if curl -s -o /dev/null --max-time 2 http://127.0.0.1:9000/; then
  die "something already answers on 127.0.0.1:9000; stop it first (the signed scope only allows ports 3000, 8080 and 9000)"
fi

BIN="$WORK/bin"
ASSAY="$BIN/assay"
mkdir -p "$BIN"

step "1/6  Build the harness and the lab"
go build -o "$ASSAY" ./cmd/assay
(cd labs/synthetic-ops && go build -o "$BIN/synthetic-ops" ./cmd/synthetic-ops)
go build -o "$BIN/summary" ./scripts/demo/summary
"$ASSAY" version

step "2/6  Tools the model may call are signed; a policy decides who may call which"
note "\$ assay tools verify: every manifest must carry a valid Ed25519 signature from a trusted key"
"$ASSAY" tools verify --manifests internal/tools/manifests --trust trust/keys
note "\$ assay policy eval --agent recon --tool http_post: a read-only agent asks to write (deny wins)"
set +e
decision=$("$ASSAY" policy eval --policy policy/default.yaml --manifests internal/tools/manifests --trust trust/keys --agent recon --tool http_post)
code=$?
set -e
echo "$decision" | sed -n -e 's/^ *"effect": "\(.*\)",$/  effect: \1/p' -e 's/^ *"reason": "\(.*\)",$/  reason: \1/p'
[ "$code" -eq 1 ] || die "expected the policy to deny (exit 1), got exit $code"

step "3/6  A signed scope document gates every outbound request"
note "\$ assay gate check: the lab is in scope, an outside host is not"
"$ASSAY" gate check --scope scope/ci.yaml --trust trust/keys --target http://127.0.0.1:9000/api/users/1 \
  | sed -n 's/^ *"reason": "\(.*\)",\{0,1\}$/  http:\/\/127.0.0.1:9000\/api\/users\/1 -> \1/p'
set +e
outside=$("$ASSAY" gate check --scope scope/ci.yaml --trust trust/keys --target https://example.com/)
code=$?
set -e
echo "$outside" | sed -n 's/^ *"reason": "\(.*\)",\{0,1\}$/  https:\/\/example.com\/ -> \1/p'
[ "$code" -eq 2 ] || die "expected the gate to deny example.com (exit 2), got exit $code"

step "4/6  Start the lab on 127.0.0.1:9000 and run the harness with the scripted fake provider"
"$BIN/synthetic-ops" -addr 127.0.0.1:9000 -public labs/synthetic-ops/public >"$WORK/lab.log" 2>&1 &
LAB_PID=$!
disown "$LAB_PID"
sh scripts/ci/wait-for.sh http://127.0.0.1:9000/ 30 >/dev/null || { cat "$WORK/lab.log" >&2; die "the lab did not start"; }
kill -0 "$LAB_PID" 2>/dev/null || { cat "$WORK/lab.log" >&2; die "the lab exited; is port 9000 free?"; }
"$ASSAY" run --config runs/pr.yaml --mode full --out "$WORK/out" --lab synthetic-ops \
  --ephemeral-audit-key --run-id "$RUN_ID" >"$WORK/run.out" 2>"$WORK/run.err" || { cat "$WORK/run.err" >&2; die "assay run failed"; }
sed -n 1p "$WORK/run.out"

step "5/6  What happened"
note "The two findings are scripted, so this shows the pipeline (gate, policy, audit, checkers), not model skill."
"$BIN/summary" "$WORK/out/$RUN_ID"

step "6/6  The audit log is a hash chain; editing one byte breaks it"
"$ASSAY" audit verify --log ".audit/$RUN_ID.log" | sed -n 's/^ *"records": \(.*\),$/  \1 records, hash chain intact/p'
# Change the first character of one record's ciphertext to a different valid one.
awk 'NR == 14 { c = substr($0, index($0, "\"ct\":\"") + 6, 1); sub(/"ct":"./, "\"ct\":\"" (c == "A" ? "B" : "A")) } { print }' \
  ".audit/$RUN_ID.log" >"$WORK/tampered.log"
cmp -s ".audit/$RUN_ID.log" "$WORK/tampered.log" && die "the edit changed nothing"
note "\$ assay audit verify, after changing one character in one record"
set +e
"$ASSAY" audit verify --log "$WORK/tampered.log" 2>&1 | sed 's/^/  /'
code=${PIPESTATUS[0]}
set -e
[ "$code" -ne 0 ] || die "the edited log verified; the chain check is broken"

printf '\n%sdone in %ss. The lab is stopped and the temporary files are removed.%s\n' "$BOLD" "$(( $(date +%s) - START ))" "$OFF"
