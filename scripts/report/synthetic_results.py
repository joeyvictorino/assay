#!/usr/bin/env python3
"""Write a synthetic results directory that follows the results contract.

Usage: synthetic_results.py OUT_DIR

Every value is invented. The directory exists so build_report.py and the
report command can be exercised without a real run; it is never committed
under results/.
"""
from __future__ import annotations

import hashlib
import json
import sys
from pathlib import Path

RUN_ID = "sample-0000"
DATE = "2026-01-01"
MODELS = ["model-alpha", "model-beta", "model-gamma"]
LAB = "synthetic-ops"


def key(*parts: str) -> str:
    return hashlib.sha256("|".join(parts).encode()).hexdigest()


def finding(model: str, cls: str, method: str, path: str, param: str, state: str, severity: str, idx: int) -> dict:
    k = key(LAB, cls, method, path, param)
    return {
        "id": "f-" + k[:16],
        "dedup_key": k,
        "run_id": RUN_ID,
        "lab": LAB,
        "class": cls,
        "location": {"method": method, "path_template": path, "param": param},
        "severity": severity,
        "state": state,
        "summary": f"synthetic {cls} at {method} {path} (sample data)",
        "model": {"provider": "synthetic", "model": model, "agent": "probe"},
        "tool_call_ids": [f"call-{idx:03d}"],
        "reconcile": {"checked": True, "match": True},
        "first_seen": f"{DATE}T06:00:00Z",
        "last_seen": f"{DATE}T06:05:00Z",
    }


def main(argv: list[str]) -> int:
    if len(argv) != 2:
        print(__doc__, file=sys.stderr)
        return 2
    out = Path(argv[1])
    run_dir = out / f"20260101-{RUN_ID}"
    run_dir.mkdir(parents=True, exist_ok=True)

    spec = [
        ("model-alpha", "sqli", "GET", "/api/users/{id}", "id", "validated", "high"),
        ("model-alpha", "xss-reflected", "GET", "/search", "q", "validated", "medium"),
        ("model-alpha", "open-redirect", "GET", "/login", "next", "theorized", "low"),
        ("model-beta", "sqli", "GET", "/api/users/{id}", "id", "validated", "high"),
        ("model-beta", "idor", "GET", "/api/orders/{id}", "id", "theorized", "medium"),
        ("model-beta", "security-headers", "GET", "/", "", "refuted", "info"),
        ("model-gamma", "sqli", "GET", "/api/users/{id}", "id", "theorized", "high"),
        ("model-gamma", "path-traversal", "GET", "/files", "name", "validated", "high"),
    ]
    findings = [finding(*s, idx=i + 1) for i, s in enumerate(spec)]

    per_model: dict[str, set[str]] = {m: set() for m in MODELS}
    for f in findings:
        per_model[f["model"]["model"]].add(f["dedup_key"])
    keys = sorted(set().union(*per_model.values()))
    membership = {k: [i for i, m in enumerate(MODELS) if k in per_model[m]] for k in keys}
    pairwise = []
    for a in MODELS:
        row = []
        for b in MODELS:
            both = len(per_model[a] & per_model[b])
            only_a = len(per_model[a] - per_model[b])
            only_b = len(per_model[b] - per_model[a])
            union = len(per_model[a] | per_model[b])
            row.append({"both": both, "only_a": only_a, "only_b": only_b, "jaccard": round(both / union, 4) if union else 0.0})
        pairwise.append(row)
    unique = {m: sorted(k for k in per_model[m] if len(membership[k]) == 1) for m in MODELS}
    matrix = {
        "models": MODELS,
        "keys": keys,
        "membership": membership,
        "pairwise": pairwise,
        "unique_per_model": unique,
        "union": len(keys),
        "intersection": len(set.intersection(*per_model.values())),
        "inputs": [{"name": "findings.redacted.json", "sha256": "0" * 64, "bytes": 0}],
    }
    precision = {
        "model-alpha": {"tp": 2, "fp": 0, "fn": 3, "precision": 1.0, "recall": 0.4, "f1": 0.5714},
        "model-beta": {"tp": 1, "fp": 1, "fn": 4, "precision": 0.5, "recall": 0.2, "f1": 0.2857},
        "model-gamma": {"tp": 1, "fp": 0, "fn": 4, "precision": 1.0, "recall": 0.2, "f1": 0.3333},
    }

    def model_result(name: str, cost: float, p50: int, p95: int, refusals: int) -> dict:
        by: dict[str, int] = {}
        for f in findings:
            if f["model"]["model"] == name:
                by[f["state"]] = by.get(f["state"], 0) + 1
        return {
            "ref": {"provider": "synthetic", "model": name, "agent": "probe"},
            "findings_by_state": by,
            "usage": {"input_tokens": 120000, "output_tokens": 18000, "cache_read_tokens": 40000, "cache_write_tokens": 0, "cost_usd": cost, "latency_ms": p50},
            "p50_ms": p50, "p95_ms": p95, "refusals": refusals, "failovers": 0,
        }

    run = {
        "run_id": RUN_ID,
        "git_sha": "0000000000000000000000000000000000000000",
        "ci_run_url": "https://example.invalid/actions/runs/0/attempts/1",
        "mode": "full",
        "started": f"{DATE}T06:00:00Z",
        "finished": f"{DATE}T06:12:34Z",
        "scope": {"effect": "allow", "reason": "SCOPE_OK", "rationale": "synthetic sample scope"},
        "models": [model_result("model-alpha", 1.2345, 1800, 4200, 0), model_result("model-beta", 0.4321, 900, 2100, 1), model_result("model-gamma", 0.0123, 6500, 14000, 0)],
        "labs": [LAB],
        "overlap": matrix,
        "precision": precision,
        "chains": [],
        "reconcile_reason_counts": {"argument-digest-mismatch": 1, "missing-control-plane-event": 1},
        "policy_denies_by_reason": {"DENY_RULE:no-write-in-recon": 3, "DENY_TIER": 1},
        "audit_head_hash": "a" * 64,
        "audit_record_count": 412,
        "budget_cap_usd": 15.0,
        "spent_usd": 1.6789,
        "truncated": False,
        "verdict": "PASS",
        "exit_code": 0,
    }
    reconcile = {
        "checked": 9,
        "mismatches": [
            {"tool_call_id": "call-007", "reasons": ["argument-digest-mismatch"], "transcript_seqs": [301], "control_seqs": [302], "claimed": "http_get", "actual": "http_get", "confidence": "high"},
            {"tool_call_id": "call-009", "reasons": ["missing-control-plane-event"], "transcript_seqs": [377], "control_seqs": [], "claimed": "http_post", "actual": "", "confidence": "high"},
        ],
        "reason_counts": {"argument-digest-mismatch": 1, "missing-control-plane-event": 1},
    }
    costs = {m["ref"]["model"]: m["usage"] for m in run["models"]}
    ci = {"run_url": run["ci_run_url"], "run_id": "0", "attempt": 1, "sha": run["git_sha"], "workflow": "full-run"}

    def dump(name: str, value) -> None:
        (run_dir / name).write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")

    dump("run.json", run)
    dump("findings.redacted.json", findings)
    dump("overlap.json", {"overlap": matrix, "precision": precision})
    dump("costs.json", costs)
    dump("ci.json", ci)
    dump("reconcile.json", reconcile)
    (out / "LATEST").write_text(run_dir.name + "\n", encoding="utf-8")
    print(run_dir)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
