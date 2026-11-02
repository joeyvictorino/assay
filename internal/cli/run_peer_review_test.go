package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/provider/fake"
)

// Peer review (ADR 0018): another model of the run scores the report. The
// tests run two scripted providers, one per model, and switch the script on
// the provider name.

const twoFakeModels = `name: two-fakes
providers:
  fake: { type: fake }
  fake2: { type: fake }
models:
  - { provider: fake, model: fake-model }
  - { provider: fake2, model: fake-model-2 }
routes:
  recon: [{ provider: fake, model: fake-model }]
  probe: [{ provider: fake, model: fake-model }]
  validate: [{ provider: fake, model: fake-model }]
  remediate: [{ provider: fake, model: fake-model }]
  score: [{ provider: fake2, model: fake-model-2 }]
budgets:
  per_run: { max_usd: 15 }
  per_model: { max_usd: 6 }
  per_task: { max_usd: 1.5, max_latency_ms: 120000 }
cost_per_mtok:
  fake-model: { input_usd: 1, output_usd: 5, cache_read_usd: 0.10 }
  fake-model-2: { input_usd: 1, output_usd: 5, cache_read_usd: 0.10 }
max_failovers: 1
agent: { max_turns: 8, max_tokens: 2048 }
`

// withConfig points the fixture at a run config written from text.
func (f *pipelineFixture) withConfig(t *testing.T, text string) {
	t.Helper()
	path := filepath.Join(f.root, "run.yaml")
	must(t, os.WriteFile(path, []byte(text), 0o644))
	f.opts.config = path
}

// runWithScripts runs the pipeline with a script per provider name.
func (f *pipelineFixture) runWithScripts(t *testing.T, scripts map[string][]fake.Step) model.RunReport {
	t.Helper()
	fakeProviderFn = func(name, _ string) *fake.Provider {
		steps, ok := scripts[name]
		if !ok {
			t.Fatalf("no script for provider %q", name)
		}
		return fake.New(steps, fake.Options{Name: name})
	}
	if code, stderr := f.run(t); code != ExitPass {
		t.Fatalf("exit %d\n%s", code, stderr)
	}
	var rr model.RunReport
	readJSON(t, filepath.Join(f.opts.out, f.opts.runID, "run.json"), &rr)
	return rr
}

func reflectionForModel(t *testing.T, rr model.RunReport, lab, m string) model.ReflectionResult {
	t.Helper()
	for _, r := range rr.Reflection {
		if r.Lab == lab && r.Model == m {
			return r
		}
	}
	t.Fatalf("no reflection record for %s on %s in %+v", m, lab, rr.Reflection)
	return model.ReflectionResult{}
}

func TestPeerReviewerScoresAnotherModelsReport(t *testing.T) {
	f := newPipelineFixture(t, "NOT-A-SECRET-LAB-MARKER")
	f.withConfig(t, twoFakeModels+"reflection: { reviewer: peer }\n")
	// fake-model reports a finding; its peer fake-model-2 scores it 8. When
	// fake-model-2 is itself assessed, its script offers score_task to the
	// probe agent (denied by policy) and fake-model, as its reviewer, tries
	// to report a finding (denied too): that record must say no score was
	// reported, not invent one.
	rr := f.runWithScripts(t, map[string][]fake.Step{
		"fake":  {reportStep("a1", "security-headers", "/"), fake.Text("done")},
		"fake2": {scoreStep("s1", 8), fake.Text("scored")},
	})
	if len(rr.Reflection) != 4 {
		t.Fatalf("want one record per lab and model, got %+v", rr.Reflection)
	}
	r := reflectionForModel(t, rr, "synthetic-ops", "fake-model")
	if r.ReviewerKind != model.ReviewerPeer || r.Reviewer != "fake-model-2" || !r.Scored || len(r.Scores) != 1 || r.Scores[0] != 8 || !r.Passed || r.Note != "" {
		t.Errorf("fake-model: %+v", r)
	}
	r = reflectionForModel(t, rr, "synthetic-ops", "fake-model-2")
	if r.ReviewerKind != model.ReviewerPeer || r.Reviewer != "fake-model" || r.Scored || !strings.Contains(r.Note, "did not report a score") {
		t.Errorf("fake-model-2: %+v", r)
	}
	// The reviewer's scoring turn ran under the score agent and is in the
	// audit log as a model call by the peer; no critique text is stored.
	export, err := os.ReadFile(filepath.Join(f.opts.out, f.opts.runID, "audit.export.jsonl"))
	must(t, err)
	if !strings.Contains(string(export), `"model":"fake-model-2"`) {
		t.Error("the audit log has no model call by the peer reviewer")
	}
	if strings.Contains(string(export), "Missing the authenticated surface") {
		t.Error("critique text reached the audit export")
	}
	report, err := os.ReadFile(filepath.Join(f.opts.out, f.opts.runID, "report.md"))
	must(t, err)
	if !strings.Contains(string(report), "| synthetic-ops | fake-model | - | peer: fake-model-2 | 8 |") {
		t.Errorf("report.md lacks the peer reviewer row:\n%s", report)
	}
}

func TestPeerReviewFallsBackToSelfWithOneModel(t *testing.T) {
	f := newPipelineFixture(t, "NOT-A-SECRET-LAB-MARKER")
	repo := repoRoot(t)
	pr, err := os.ReadFile(filepath.Join(repo, "runs", "pr.yaml"))
	must(t, err)
	f.withConfig(t, string(pr)+"reflection: { reviewer: peer }\n")
	rr := f.runWithScript(t, func(base string) []fake.Step { return fakeSteps(base) })
	r := reflectionFor(t, rr, "synthetic-ops")
	if r.ReviewerKind != model.ReviewerSelf || r.Reviewer != "fake-model" || !r.Scored || r.Scores[0] != 8 {
		t.Fatalf("%+v", r)
	}
	if r.Note != NoteNoPeer {
		t.Errorf("note = %q, want %q", r.Note, NoteNoPeer)
	}
}

func TestSelfReviewIsTheDefaultAndNamed(t *testing.T) {
	f := newPipelineFixture(t, "NOT-A-SECRET-LAB-MARKER")
	rr := f.runWithScript(t, func(base string) []fake.Step { return fakeSteps(base) })
	for _, r := range rr.Reflection {
		if r.ReviewerKind != model.ReviewerSelf || r.Reviewer != "fake-model" || r.Note != "" {
			t.Errorf("%s: %+v", r.Lab, r)
		}
		// Beside the score, what the checkers said: the two scripted
		// findings each end in exactly one state.
		if r.Validated+r.Refuted+r.Theorized != 2 {
			t.Errorf("%s: checker counts %d/%d/%d do not cover the 2 findings", r.Lab, r.Validated, r.Refuted, r.Theorized)
		}
	}
}

// ADR 0019: a revision that reports nothing new ends reflection; the same
// report is not scored twice.
func TestRevisionWithNothingNewStopsReflection(t *testing.T) {
	f := newPipelineFixture(t, "NOT-A-SECRET-LAB-MARKER")
	rr := f.runWithScript(t, func(string) []fake.Step {
		return []fake.Step{
			reportStep("a1", "security-headers", "/"), fake.Text("done"), // probe
			scoreStep("s1", 4), fake.Text("scored"), //                      score: low
			reportStep("b1", "security-headers", "/"), fake.Text("done"), // revision: the same finding again
			scoreStep("s2", 9), fake.Text("scored"), //                      must never be consumed
		}
	})
	r := reflectionFor(t, rr, "synthetic-ops")
	if len(r.Scores) != 1 || r.Scores[0] != 4 || r.Revisions != 1 || r.Passed || r.Final != 4 {
		t.Fatalf("%+v", r)
	}
	if r.Note != NoteNoProgress {
		t.Errorf("note = %q, want %q", r.Note, NoteNoProgress)
	}
	export, err := os.ReadFile(filepath.Join(f.opts.out, f.opts.runID, "audit.export.jsonl"))
	must(t, err)
	if !strings.Contains(string(export), `"no_progress":true`) {
		t.Error("the audit log lacks the no-progress record")
	}
}
