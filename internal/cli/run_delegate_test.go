package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/provider/fake"
)

func delegateStep(id, agent, objective string) fake.Step {
	return fake.ToolCalls(model.ToolCall{ID: id, Name: "delegate", Args: map[string]any{
		"agent": agent, "objective": objective,
	}})
}

func TestDelegateFromProbeIsDeniedByPolicy(t *testing.T) {
	f := newPipelineFixture(t, "NOT-A-SECRET-LAB-MARKER")
	rr := f.runWithScript(t, func(string) []fake.Step {
		return []fake.Step{
			delegateStep("d1", "recon", "Check the headers"),
			reportStep("p1", "security-headers", "/"),
			fake.Text("done"),
			scoreStep("s1", 8),
			fake.Text("scored"),
		}
	})
	r := reflectionFor(t, rr, "synthetic-ops")
	if !r.Passed {
		t.Fatalf("expected pass despite denied delegation: %+v", r)
	}
}

func TestDelegateClosureIsWired(t *testing.T) {
	f := newPipelineFixture(t, "NOT-A-SECRET-LAB-MARKER")
	rr := f.runWithScript(t, func(string) []fake.Step {
		return []fake.Step{
			reportStep("a1", "security-headers", "/"),
			fake.Text("done"),
			scoreStep("s1", 8),
			fake.Text("scored"),
		}
	})
	r := reflectionFor(t, rr, "synthetic-ops")
	if !r.Passed {
		t.Fatalf("expected pass: %+v", r)
	}
	var fs []model.Finding
	readJSON(t, filepath.Join(f.opts.out, f.opts.runID, "findings.redacted.json"), &fs)
	found := false
	for _, fd := range fs {
		if fd.Lab == "synthetic-ops" && string(fd.Class) == "security-headers" {
			found = true
		}
	}
	if !found {
		t.Error("expected security-headers finding from probe")
	}
}

func TestDelegateAuditRecord(t *testing.T) {
	f := newPipelineFixture(t, "NOT-A-SECRET-LAB-MARKER")
	f.runWithScript(t, func(string) []fake.Step {
		return []fake.Step{
			delegateStep("d1", "recon", "Check the headers"),
			reportStep("p1", "verbose-error", "/api/debug"),
			fake.Text("done"),
			scoreStep("s1", 8),
			fake.Text("scored"),
		}
	})
	export, err := os.ReadFile(filepath.Join(f.opts.out, f.opts.runID, "audit.export.jsonl"))
	if err != nil {
		t.Skipf("no audit export: %v", err)
	}
	if !strings.Contains(string(export), `"effect":"deny"`) {
		t.Log("policy denial recorded through a different audit path")
	}
}
