#!/usr/bin/env python3
"""Decide whether a finished run may be committed to results/ automatically.

usage: results-gate.py RUN_JSON EVENT_NAME

Exit status:
  0  commit the run
  3  skip the commit (not an error; the reason is printed)
  4  do not commit and fail the job (the run is unusable; the reason is printed)

Rules, in order:
  1. A run with no real model provider is a pipeline check, not a result.
  2. A real model that never completed a call (no input or cache-read tokens)
     means a bad key, a wrong model id or an outage; committing it would put a
     meaningless row in the results.
  3. A degraded run (a configured provider was dropped for lack of a key) is
     committed only when someone dispatched the workflow by hand, so that a
     push or a schedule never replaces the latest result with a lesser one.
"""
import json
import sys

COMMIT, SKIP, FAIL = 0, 3, 4


def decide(run, event):
    models = run.get("models", [])
    real = [m for m in models if m["ref"]["provider"] != "fake"]
    if not real:
        return SKIP, "fake-only run: not committed"
    idle = [m["ref"]["model"] for m in real
            if m["usage"].get("input_tokens", 0) + m["usage"].get("cache_read_tokens", 0) == 0]
    if idle:
        return FAIL, "no completed model call for: " + ", ".join(sorted(idle)) + " (bad key, wrong model id or outage); not committed"
    if run.get("mode") == "degraded" and event != "workflow_dispatch":
        return SKIP, "degraded run from a %s event: committed only when dispatched by hand" % event
    return COMMIT, "ok"


def main(argv):
    if len(argv) != 3:
        print(__doc__, file=sys.stderr)
        return 2
    with open(argv[1]) as fh:
        run = json.load(fh)
    code, why = decide(run, argv[2])
    print(why)
    return code


if __name__ == "__main__":
    sys.exit(main(sys.argv))
