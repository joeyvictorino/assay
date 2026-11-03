package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/replaydiff"
)

func TestReplayDiff_TextOutput(t *testing.T) {
	dir := t.TempDir()
	baseFindings := []model.Finding{
		{ID: "f1", DedupKey: "k1", Lab: "sqli", Model: model.ModelRef{Model: "m1"}, Severity: model.SevHigh, Summary: "SQL injection"},
	}
	curFindings := []model.Finding{
		{ID: "f2", DedupKey: "k1", Lab: "sqli", Model: model.ModelRef{Model: "m1"}, Severity: model.SevHigh, Summary: "SQL injection"},
		{ID: "f3", DedupKey: "k2", Lab: "xss", Model: model.ModelRef{Model: "m1"}, Severity: model.SevMedium, Summary: "XSS reflected"},
	}

	baseDir := setupCompareRun(t, dir, "run-base", baseFindings)
	curDir := setupCompareRun(t, dir, "run-cur", curFindings)

	var stdout, stderr bytes.Buffer
	code := runReplayDiff([]string{"--base", baseDir, "--current", curDir}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "New Findings") {
		t.Errorf("missing New Findings section:\n%s", out)
	}
	if !strings.Contains(out, "XSS reflected") {
		t.Errorf("missing new finding:\n%s", out)
	}
}

func TestReplayDiff_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	baseFindings := []model.Finding{
		{ID: "f1", DedupKey: "k1", Lab: "sqli", Model: model.ModelRef{Model: "m1"}, Severity: model.SevHigh},
		{ID: "f2", DedupKey: "k2", Lab: "xss", Model: model.ModelRef{Model: "m1"}, Severity: model.SevLow},
	}
	curFindings := []model.Finding{
		{ID: "f3", DedupKey: "k1", Lab: "sqli", Model: model.ModelRef{Model: "m1"}, Severity: model.SevCritical},
	}

	baseDir := setupCompareRun(t, dir, "run-b", baseFindings)
	curDir := setupCompareRun(t, dir, "run-c", curFindings)

	var stdout, stderr bytes.Buffer
	code := runReplayDiff([]string{"--base", baseDir, "--current", curDir, "--format", "json"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}

	var d replaydiff.DiffResult
	if err := json.Unmarshal(stdout.Bytes(), &d); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if d.BaseRunID != "run-b" || d.CurRunID != "run-c" {
		t.Errorf("run IDs: got %s/%s", d.BaseRunID, d.CurRunID)
	}
	if d.Summary.ResolvedCount != 1 {
		t.Errorf("resolved = %d, want 1", d.Summary.ResolvedCount)
	}
	if d.Summary.ChangedCount != 1 {
		t.Errorf("changed = %d, want 1", d.Summary.ChangedCount)
	}
}

func TestReplayDiff_MissingArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runReplayDiff(nil, &stdout, &stderr)
	if code != ExitError {
		t.Fatalf("expected exit %d, got %d", ExitError, code)
	}
}

func TestReplayDiff_BadFormat(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runReplayDiff([]string{"--base", "/tmp", "--current", "/tmp", "--format", "xml"}, &stdout, &stderr)
	if code != ExitError {
		t.Fatalf("expected exit %d, got %d", ExitError, code)
	}
}
