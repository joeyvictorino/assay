package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/checkermatrix"
	"github.com/joeyvictorino/assay/internal/model"
)

func TestCheckerMatrix_TextOutput(t *testing.T) {
	dir := t.TempDir()
	findings := []model.Finding{
		{Lab: "sqli", Model: model.ModelRef{Model: "gpt-4"}, Severity: model.SevHigh},
		{Lab: "sqli", Model: model.ModelRef{Model: "claude-3"}, Severity: model.SevMedium},
		{Lab: "xss", Model: model.ModelRef{Model: "gpt-4"}, Severity: model.SevLow},
	}
	setupCompareRun(t, dir, "run-1", findings)
	runDir := dir + "/run-1"

	var stdout, stderr bytes.Buffer
	code := runCheckerMatrix([]string{"--path", runDir}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "sqli") || !strings.Contains(out, "xss") {
		t.Errorf("missing checker names:\n%s", out)
	}
	if !strings.Contains(out, "gpt-4") {
		t.Errorf("missing model name:\n%s", out)
	}
}

func TestCheckerMatrix_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	findings := []model.Finding{
		{Lab: "sqli", Model: model.ModelRef{Model: "gpt-4"}, Severity: model.SevHigh},
		{Lab: "xss", Model: model.ModelRef{Model: "gpt-4"}, Severity: model.SevLow},
	}
	setupCompareRun(t, dir, "run-1", findings)
	runDir := dir + "/run-1"

	var stdout, stderr bytes.Buffer
	code := runCheckerMatrix([]string{"--path", runDir, "--format", "json"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}

	var m checkermatrix.MatrixResult
	if err := json.Unmarshal(stdout.Bytes(), &m); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if len(m.Checkers) != 2 {
		t.Errorf("checkers = %d, want 2", len(m.Checkers))
	}
	if len(m.Models) != 1 {
		t.Errorf("models = %d, want 1", len(m.Models))
	}
}

func TestCheckerMatrix_MissingPath(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCheckerMatrix(nil, &stdout, &stderr)
	if code != ExitError {
		t.Fatalf("expected exit %d, got %d", ExitError, code)
	}
}

func TestCheckerMatrix_NoFindings(t *testing.T) {
	dir := t.TempDir()
	setupCompareRun(t, dir, "empty-run", nil)
	runDir := dir + "/empty-run"

	var stdout, stderr bytes.Buffer
	code := runCheckerMatrix([]string{"--path", runDir}, &stdout, &stderr)
	if code != ExitError {
		t.Fatalf("expected exit %d for missing findings, got %d", ExitError, code)
	}
}
