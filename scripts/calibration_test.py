import json
import os
import subprocess
import sys
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
SCRIPT = os.path.join(HERE, "calibration.py")


def write(path, obj):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w") as fh:
        json.dump(obj, fh)


class CalibrationTable(unittest.TestCase):
    def test_rows_join_reflection_findings_and_recall(self):
        with tempfile.TemporaryDirectory() as tmp:
            run = os.path.join(tmp, "r1")
            write(os.path.join(run, "run.json"), {
                "ci_run_url": "https://example.test/o/r/actions/runs/1/attempts/1",
                "reflection": [
                    {"lab": "lab-a", "model": "m.x", "scored": True, "scores": [3, 8], "revisions": 1, "final_score": 8, "passed": True},
                    {"lab": "lab-b", "model": "m.x", "scored": False, "note": "assessment failed: timeout"},
                ],
                "precision": {"m.x@lab-a": {"recall": 0.25}},
            })
            write(os.path.join(run, "findings.m.x.json"), [
                {"lab": "lab-a", "state": "validated"}, {"lab": "lab-a", "state": "validated"},
                {"lab": "lab-a", "state": "theorized"}, {"lab": "lab-b", "state": "refuted"},
            ])
            # a run without reflection is ignored
            write(os.path.join(tmp, "r0", "run.json"), {"reflection": []})
            out = subprocess.run([sys.executable, SCRIPT, tmp, "--json"], capture_output=True, text=True, check=True).stdout
            rows = json.loads(out)
            self.assertEqual(len(rows), 2)
            a, b = rows
            self.assertEqual((a["lab"], a["scores"], a["revisions"], a["final"], a["passed"]), ("lab-a", [3, 8], 1, 8, True))
            self.assertEqual((a["validated"], a["theorized"], a["refuted"], a["recall"]), (2, 1, 0, 0.25))
            self.assertEqual((b["lab"], b["scores"], b["refuted"], b["recall"], b["note"]), ("lab-b", [], 1, None, "assessment failed: timeout"))
            self.assertEqual(a["ci_run"], "runs/1/attempts/1")

    def test_markdown_and_empty_message(self):
        with tempfile.TemporaryDirectory() as tmp:
            r = subprocess.run([sys.executable, SCRIPT, tmp], capture_output=True, text=True, check=True)
            self.assertIn("No run under", r.stdout)


if __name__ == "__main__":
    unittest.main()
