package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestModelCompareBasic(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	rrA := model.RunReport{RunID: "run-a", Models: []model.ModelResult{
		{Ref: model.ModelRef{Provider: "openai", Model: "gpt-4"}},
	}}
	rrB := model.RunReport{RunID: "run-b", Models: []model.ModelResult{
		{Ref: model.ModelRef{Provider: "anthropic", Model: "claude-3"}},
	}}
	writeJSONFile(t, dirA, runJSON, rrA)
	writeJSONFile(t, dirB, runJSON, rrB)

	fA := []model.Finding{
		{ID: "f1", DedupKey: "f1", Lab: "lab1", Severity: "high", Summary: "SQL injection"},
		{ID: "f2", DedupKey: "f2", Lab: "lab1", Severity: "medium", Summary: "XSS"},
	}
	fB := []model.Finding{
		{ID: "f1", DedupKey: "f1", Lab: "lab1", Severity: "high", Summary: "SQL injection"},
		{ID: "f3", DedupKey: "f3", Lab: "lab1", Severity: "critical", Summary: "RCE"},
	}
	writeJSONFile(t, dirA, findingsJSON, fA)
	writeJSONFile(t, dirB, findingsJSON, fB)

	var out, errb bytes.Buffer
	code := runModelCompare([]string{"--run-a", dirA, "--run-b", dirB}, &out, &errb)
	if code != ExitPass {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	text := out.String()
	if !strings.Contains(text, "openai/gpt-4") {
		t.Error("missing model A name in output")
	}
	if !strings.Contains(text, "anthropic/claude-3") {
		t.Error("missing model B name in output")
	}
}

func TestModelCompareJSON(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	writeJSONFile(t, dirA, runJSON, model.RunReport{RunID: "a"})
	writeJSONFile(t, dirB, runJSON, model.RunReport{RunID: "b"})

	var out, errb bytes.Buffer
	code := runModelCompare([]string{"--run-a", dirA, "--run-b", dirB, "--format", "json"}, &out, &errb)
	if code != ExitPass {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), `"run_a"`) {
		t.Error("missing run_a in JSON output")
	}
}

func TestModelCompareMissingFlags(t *testing.T) {
	var out, errb bytes.Buffer
	code := runModelCompare([]string{}, &out, &errb)
	if code != ExitError {
		t.Fatalf("expected error exit, got %d", code)
	}
}

func TestModelCompareBadFormat(t *testing.T) {
	var out, errb bytes.Buffer
	code := runModelCompare([]string{"--run-a", "x", "--run-b", "y", "--format", "xml"}, &out, &errb)
	if code != ExitError {
		t.Fatalf("expected error exit, got %d", code)
	}
}
