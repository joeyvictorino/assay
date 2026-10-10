#!/usr/bin/env python3
"""Recompute a committed run's overlap matrix from its per-model findings.

    go build -o assay ./cmd/assay
    python3 scripts/check_overlap.py results/<run-id> [--assay ./assay]

Runs `assay overlap` over results/<run>/findings.<model>.json and compares
every matrix field (models, keys, membership, pairwise, unique_per_model,
union, intersection) with results/<run>/overlap.json. Exit 0 when they
match, 1 otherwise. Input digests are not compared (the run writes none),
and neither is precision: the run scores each model per ground-truth lab,
while `assay overlap --ground-truth` scores all of a model's findings
against one lab's list. Precision is in run.json and report.md.
"""
import argparse
import json
import os
import subprocess
import sys
import tempfile

FIELDS = ["models", "keys", "membership", "pairwise", "unique_per_model", "union", "intersection"]


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("run")
    ap.add_argument("--assay", default="./assay")
    a = ap.parse_args()
    committed = json.load(open(os.path.join(a.run, "overlap.json")))["overlap"]
    inputs = [os.path.join(a.run, "findings.%s.json" % m) for m in committed["models"]]
    with tempfile.TemporaryDirectory() as d:
        out = os.path.join(d, "overlap.json")
        subprocess.run([a.assay, "overlap", "--inputs", ",".join(inputs), "--out", out], check=True, stdout=subprocess.DEVNULL)
        fresh = json.load(open(out))["overlap"]
    bad = [f for f in FIELDS if committed.get(f) != fresh.get(f)]
    for f in FIELDS:
        print("%-17s %s" % (f, "differs" if f in bad else "matches"))
    print("union %d, intersection %d across %d models" % (fresh["union"], fresh["intersection"], len(fresh["models"])))
    return 1 if bad else 0


if __name__ == "__main__":
    sys.exit(main())
