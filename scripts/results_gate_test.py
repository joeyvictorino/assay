import importlib.util
import json
import os
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("results_gate", os.path.join(HERE, "ci", "results-gate.py"))
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)


def model(provider, name, tokens):
    return {"ref": {"provider": provider, "model": name}, "usage": {"input_tokens": tokens, "cache_read_tokens": 0}}


class ResultsGate(unittest.TestCase):
    def test_fake_only_is_skipped(self):
        code, _ = gate.decide({"mode": "full", "models": [model("fake", "fake-model", 10)]}, "workflow_dispatch")
        self.assertEqual(code, gate.SKIP)

    def test_idle_real_model_fails(self):
        run = {"mode": "full", "models": [model("anthropic", "claude-opus-5-5", 100), model("openai", "x", 0)]}
        code, why = gate.decide(run, "workflow_dispatch")
        self.assertEqual(code, gate.FAIL)
        self.assertIn("x", why)

    def test_degraded_only_from_a_manual_dispatch(self):
        run = {"mode": "degraded", "models": [model("anthropic", "claude-opus-5-5", 100)]}
        self.assertEqual(gate.decide(run, "push")[0], gate.SKIP)
        self.assertEqual(gate.decide(run, "schedule")[0], gate.SKIP)
        self.assertEqual(gate.decide(run, "workflow_dispatch")[0], gate.COMMIT)

    def test_full_run_commits_on_any_event(self):
        run = {"mode": "full", "models": [model("anthropic", "claude-opus-5-5", 100)]}
        self.assertEqual(gate.decide(run, "schedule")[0], gate.COMMIT)

    def test_every_committed_run_passes_the_gate_when_dispatched(self):
        root = os.path.join(HERE, "..", "results")
        for name in sorted(os.listdir(root)):
            path = os.path.join(root, name, "run.json")
            if not os.path.isfile(path):
                continue
            with open(path) as fh:
                run = json.load(fh)
            code, why = gate.decide(run, "workflow_dispatch")
            self.assertEqual(code, gate.COMMIT, "%s: %s" % (name, why))


if __name__ == "__main__":
    unittest.main()
