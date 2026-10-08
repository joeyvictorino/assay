#!/usr/bin/env python3
"""Self-score against checked results, from committed runs.

For every run under results/ that carries self-reflection records, print one
row per (lab, model): the model's own scores in order, how many revisions
followed, and what the deterministic checkers then said about that model's
findings on that lab (validated, theorized, refuted), plus recall against
ground truth where the lab has complete ground truth.

This is a table, not a calibration test. Three runs of one small model is an
anecdote; the script exists so that a real study over many runs and models
can be read the same way, and so that every number quoted elsewhere in this
repository can be regenerated:

    python3 scripts/calibration.py [results_dir]   # default: results
    python3 scripts/calibration.py --json

Standard library only.
"""
import json
import os
import re
import sys


def safe_name(model):
    """Mirror of the pipeline's file-name rule for per-model findings."""
    return re.sub(r"[^A-Za-z0-9.\-]", "_", model)


def load(path):
    with open(path, encoding="utf-8") as fh:
        return json.load(fh)


def rows_for_run(run_dir):
    run = load(os.path.join(run_dir, "run.json"))
    reflection = run.get("reflection") or []
    if not reflection:
        return []
    precision = run.get("precision") or {}
    cache = {}
    out = []
    for r in reflection:
        model, lab = r["model"], r["lab"]
        if model not in cache:
            p = os.path.join(run_dir, "findings.%s.json" % safe_name(model))
            cache[model] = load(p) if os.path.exists(p) else []
        states = {"validated": 0, "theorized": 0, "refuted": 0}
        for f in cache[model]:
            if f.get("lab") == lab and f.get("state") in states:
                states[f["state"]] += 1
        prf = precision.get("%s@%s" % (model, lab))
        out.append({
            "run": os.path.basename(run_dir),
            "ci_run": (run.get("ci_run_url") or "").split("/actions/")[-1],
            "lab": lab,
            "model": model,
            "scores": r.get("scores") or [],
            "revisions": r.get("revisions", 0),
            "final": r.get("final_score"),
            "passed": bool(r.get("passed")),
            "note": r.get("note", ""),
            "validated": states["validated"],
            "theorized": states["theorized"],
            "refuted": states["refuted"],
            "recall": None if not prf else round(prf.get("recall", 0), 3),
        })
    return out


def collect(results_dir):
    rows = []
    for d in sorted(os.listdir(results_dir)):
        full = os.path.join(results_dir, d)
        if os.path.isfile(os.path.join(full, "run.json")):
            rows.extend(rows_for_run(full))
    return rows


def markdown(rows):
    lines = ["| Run | Lab | Model | Self-scores | Revisions | Passed | Validated | Theorized | Refuted | Recall (ground truth) | Note |",
             "| --- | --- | --- | --- | ---: | --- | ---: | ---: | ---: | ---: | --- |"]
    for r in rows:
        scores = ", ".join(str(s) for s in r["scores"]) or "-"
        recall = "-" if r["recall"] is None else "%.3f" % r["recall"]
        lines.append("| %s | %s | %s | %s | %d | %s | %d | %d | %d | %s | %s |" % (
            r["run"], r["lab"], r["model"], scores, r["revisions"], "yes" if r["passed"] else "no",
            r["validated"], r["theorized"], r["refuted"], recall, r["note"] or ""))
    return "\n".join(lines)


def main(argv):
    as_json = "--json" in argv
    args = [a for a in argv if not a.startswith("--")]
    results_dir = args[0] if args else "results"
    rows = collect(results_dir)
    if as_json:
        print(json.dumps(rows, indent=2, sort_keys=True))
    elif rows:
        print(markdown(rows))
    else:
        print("No run under %s carries self-reflection records." % results_dir)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
