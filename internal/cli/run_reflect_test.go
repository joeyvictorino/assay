package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/provider/fake"
)

func scoreStep(id string, score int) fake.Step {
	return fake.ToolCalls(model.ToolCall{ID: id, Name: "score_task", Args: map[string]any{
		"score": score, "critique": "Missing the authenticated surface.",
	}})
}

func reportStep(id, class, path string) fake.Step {
	return fake.ToolCalls(model.ToolCall{ID: id, Name: "report_finding", Args: map[string]any{
		"class": class, "method": "GET", "path": path, "param": "", "severity": "low",
		"summary": "Observed on the root page.", "evidence_tool_call_ids": []any{},
	}})
}

func (f *pipelineFixture) runWithScript(t *testing.T, steps func(base string) []fake.Step) model.RunReport {
	t.Helper()
	fakeProviderFn = func(name, baseURL string) *fake.Provider {
		return fake.New(steps(baseURL), fake.Options{Name: name})
	}
	if code, stderr := f.run(t); code != ExitPass {
		t.Fatalf("exit %d\n%s", code, stderr)
	}
	var rr model.RunReport
	readJSON(t, filepath.Join(f.opts.out, f.opts.runID, "run.json"), &rr)
	return rr
}

func reflectionFor(t *testing.T, rr model.RunReport, lab string) model.ReflectionResult {
	t.Helper()
	for _, r := range rr.Reflection {
		if r.Lab == lab {
			return r
		}
	}
	t.Fatalf("no reflection record for lab %s in %+v", lab, rr.Reflection)
	return model.ReflectionResult{}
}

func TestPassingSelfScoreIsRecordedWithoutRevision(t *testing.T) {
	f := newPipelineFixture(t, "NOT-A-SECRET-LAB-MARKER")
	rr := f.runWithScript(t, func(base string) []fake.Step { return fakeSteps(base) })
	if len(rr.Reflection) != 2 {
		t.Fatalf("want one record per lab, got %+v", rr.Reflection)
	}
	for _, r := range rr.Reflection {
		if !r.Scored || len(r.Scores) != 1 || r.Scores[0] != 8 || r.Revisions != 0 || !r.Passed || r.Final != 8 {
			t.Errorf("%s: %+v", r.Lab, r)
		}
	}
	export, err := os.ReadFile(filepath.Join(f.opts.out, f.opts.runID, "audit.export.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(export), `"kind":"score"`) {
		t.Error("the audit log lacks a score record")
	}
	// the critique text is a model output and must not be persisted
	if strings.Contains(string(export), "Covered the visible surface") {
		t.Error("critique text reached the audit export")
	}
}

func TestLowScoreTriggersOneRevisionThenPasses(t *testing.T) {
	f := newPipelineFixture(t, "NOT-A-SECRET-LAB-MARKER")
	rr := f.runWithScript(t, func(string) []fake.Step {
		return []fake.Step{
			reportStep("a1", "security-headers", "/"), fake.Text("done"), // probe
			scoreStep("s1", 4), fake.Text("scored"), //                      score: low
			reportStep("b1", "verbose-error", "/api/boom"), fake.Text("done"), // revision
			scoreStep("s2", 9), fake.Text("scored"), //                      score: pass
		}
	})
	r := reflectionFor(t, rr, "synthetic-ops")
	if len(r.Scores) != 2 || r.Scores[0] != 4 || r.Scores[1] != 9 || r.Revisions != 1 || !r.Passed || r.Final != 9 {
		t.Fatalf("%+v", r)
	}
	var fs []model.Finding
	readJSON(t, filepath.Join(f.opts.out, f.opts.runID, "findings.redacted.json"), &fs)
	classes := map[string]bool{}
	for _, fd := range fs {
		if fd.Lab == "synthetic-ops" {
			classes[string(fd.Class)] = true
		}
	}
	if !classes["security-headers"] || !classes["verbose-error"] {
		t.Errorf("revision findings must be merged with the first pass: %v", classes)
	}
	// revision runs reuse tool call ids freely; per-task ids keep them clean
	var rec struct {
		Mismatches []any `json:"mismatches"`
	}
	readJSON(t, filepath.Join(f.opts.out, f.opts.runID, "reconcile.json"), &rec)
	if len(rec.Mismatches) != 0 {
		t.Errorf("reconciliation mismatches: %+v", rec.Mismatches)
	}
}

func TestRevisionsAreCappedAtTwo(t *testing.T) {
	f := newPipelineFixture(t, "NOT-A-SECRET-LAB-MARKER")
	rr := f.runWithScript(t, func(string) []fake.Step {
		return []fake.Step{
			fake.Text("done"),
			scoreStep("s1", 3), fake.Text("scored"), fake.Text("done"),
			scoreStep("s2", 3), fake.Text("scored"), fake.Text("done"),
			scoreStep("s3", 3), fake.Text("scored"),
		}
	})
	r := reflectionFor(t, rr, "synthetic-ops")
	if len(r.Scores) != 3 || r.Revisions != 2 || r.Passed || r.Final != 3 {
		t.Fatalf("%+v", r)
	}
}

func TestModelThatReportsNoScoreIsRecordedNotGuessed(t *testing.T) {
	f := newPipelineFixture(t, "NOT-A-SECRET-LAB-MARKER")
	rr := f.runWithScript(t, func(string) []fake.Step {
		return []fake.Step{fake.Text("done"), fake.Text("I think it is fine.")}
	})
	r := reflectionFor(t, rr, "synthetic-ops")
	if r.Scored || len(r.Scores) != 0 || r.Revisions != 0 || r.Passed {
		t.Fatalf("%+v", r)
	}
	if !strings.Contains(r.Note, "did not report a score") {
		t.Errorf("note: %q", r.Note)
	}
}
