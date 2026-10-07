#!/usr/bin/env sh
# selftest.sh [OUT_PDF]
# Generates a synthetic results directory, renders it with build_report.py
# in --sample mode and checks the PDF: exactly four pages, titled SAMPLE,
# every number labelled synthetic. Writes docs/report/sample-report.pdf by
# default. Needs only python3 and `pip install -r scripts/report/requirements.txt`.
set -eu
cd "$(dirname "$0")/../.."
OUT="${1:-docs/report/sample-report.pdf}"
PY="${PYTHON:-python3}"
WORK="$(mktemp -d 2>/dev/null || mktemp -d -t assay-report)"
trap 'rm -rf "$WORK"' EXIT

"$PY" -c 'import reportlab' 2>/dev/null || {
  echo "selftest: reportlab missing; run: $PY -m pip install -r scripts/report/requirements.txt" >&2
  exit 2
}

RUN_DIR="$("$PY" scripts/report/synthetic_results.py "$WORK/results")"
for f in run.json findings.redacted.json overlap.json costs.json ci.json reconcile.json; do
  [ -f "$RUN_DIR/$f" ] || { echo "selftest: generator did not write $f" >&2; exit 1; }
done
[ "$(cat "$WORK/results/LATEST")" = "$(basename "$RUN_DIR")" ] || { echo "selftest: LATEST mismatch" >&2; exit 1; }

"$PY" scripts/report/build_report.py "$RUN_DIR" --sample --out "$OUT"

"$PY" - "$OUT" <<'PYCHECK'
import base64, re, sys, zlib
data = open(sys.argv[1], "rb").read()
pages = len(re.findall(rb"/Type\s*/Page[^s]", data))
if pages != 4:
    sys.exit(f"selftest: expected 4 pages, found {pages}")
# Decode content streams (reportlab writes ASCII85 + Flate) so text can be inspected.
text = b""
for m in re.finditer(rb"stream\r?\n(.*?)endstream", data, re.S):
    chunk = m.group(1).strip()
    if chunk.endswith(b"~>"):
        try:
            chunk = base64.a85decode(chunk, adobe=True)
        except ValueError:
            pass
    try:
        chunk = zlib.decompress(chunk)
    except zlib.error:
        pass
    text += chunk
for needle in (b"SAMPLE", b"synthetic", b"Scope statement", b"Overlap", b"Runtime"):
    if needle not in text and needle not in data:
        sys.exit(f"selftest: {needle!r} not found in PDF")
print(f"selftest: OK ({pages} pages, {len(data)} bytes)")
PYCHECK
