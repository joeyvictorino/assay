package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/continuous"
	"github.com/joeyvictorino/assay/internal/model"
)

func setupCompareRun(t *testing.T, dir, runID string, findings []model.Finding) string {
	t.Helper()
	rd := filepath.Join(dir, runID)
	if err := os.MkdirAll(rd, 0o755); err != nil {
		t.Fatal(err)
	}
	rr := model.RunReport{RunID: runID}
	writeJSONFile(t, rd, runJSON, rr)
	if findings != nil {
		writeJSONFile(t, rd, findingsJSON, findings)
	}
	return rd
}

func TestCompare_TextOutput(t *testing.T) {
	dir := t.TempDir()
	baseFindings := []model.Finding{
		{ID: "f1", DedupKey: "key-a", State: model.StateValidated, Severity: model.SevHigh},
		{ID: "f2", DedupKey: "key-b", State: model.StateValidated, Severity: model.SevMedium},
	}
	curFindings := []model.Finding{
		{ID: "f3", DedupKey: "key-a", State: model.StateValidated, Severity: model.SevHigh},
		{ID: "f4", DedupKey: "key-c", State: model.StateValidated, Severity: model.SevLow},
	}
	baseDir := setupCompareRun(t, dir, "run-base", baseFindings)
	curDir := setupCompareRun(t, dir, "run-cur", curFindings)

	var stdout, stderr bytes.Buffer
	code := runCompare([]string{"--baseline", baseDir, "--current", curDir}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "run-base") || !strings.Contains(out, "run-cur") {
		t.Errorf("output should reference both run IDs:\n%s", out)
	}
	if !strings.Contains(out, "key-c") {
		t.Error("new finding key-c not in output")
	}
	if !strings.Contains(out, "key-b") {
		t.Error("fixed finding key-b not in output")
	}
}

func TestCompare_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	baseDir := setupCompareRun(t, dir, "run-b", []model.Finding{
		{ID: "f1", DedupKey: "k1", State: model.StateValidated, Severity: model.SevHigh},
	})
	curDir := setupCompareRun(t, dir, "run-c", []model.Finding{
		{ID: "f2", DedupKey: "k1", State: model.StateValidated, Severity: model.SevHigh},
		{ID: "f3", DedupKey: "k2", State: model.StateTheorized, Severity: model.SevMedium},
	})

	var stdout, stderr bytes.Buffer
	code := runCompare([]string{"--baseline", baseDir, "--current", curDir, "--format", "json"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	var d continuous.Delta
	if err := json.Unmarshal(stdout.Bytes(), &d); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if d.BaselineRunID != "run-b" || d.CurrentRunID != "run-c" {
		t.Errorf("run IDs: got %s/%s", d.BaselineRunID, d.CurrentRunID)
	}
	if len(d.New) != 1 || d.New[0] != "k2" {
		t.Errorf("new = %v, want [k2]", d.New)
	}
}

func TestCompare_FailOn(t *testing.T) {
	dir := t.TempDir()
	baseDir := setupCompareRun(t, dir, "base", nil)
	curDir := setupCompareRun(t, dir, "cur", []model.Finding{
		{ID: "f1", DedupKey: "k1", State: model.StateValidated, Severity: model.SevHigh},
	})

	var stdout, stderr bytes.Buffer
	code := runCompare([]string{"--baseline", baseDir, "--current", curDir, "--fail-on", "high"}, &stdout, &stderr)
	if code != ExitBlocked {
		t.Fatalf("expected exit %d, got %d; stderr: %s", ExitBlocked, code, stderr.String())
	}
}

func TestCompare_FailOnBelowThreshold(t *testing.T) {
	dir := t.TempDir()
	baseDir := setupCompareRun(t, dir, "base", nil)
	curDir := setupCompareRun(t, dir, "cur", []model.Finding{
		{ID: "f1", DedupKey: "k1", State: model.StateValidated, Severity: model.SevLow},
	})

	var stdout, stderr bytes.Buffer
	code := runCompare([]string{"--baseline", baseDir, "--current", curDir, "--fail-on", "high"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected exit %d, got %d", ExitPass, code)
	}
}

func TestCompare_MissingArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCompare(nil, &stdout, &stderr)
	if code != ExitError {
		t.Fatalf("expected exit %d, got %d", ExitError, code)
	}
}

func TestCompare_WriteToFile(t *testing.T) {
	dir := t.TempDir()
	baseDir := setupCompareRun(t, dir, "base", nil)
	curDir := setupCompareRun(t, dir, "cur", nil)
	outFile := filepath.Join(dir, "output.md")

	var stdout, stderr bytes.Buffer
	code := runCompare([]string{"--baseline", baseDir, "--current", curDir, "--out", outFile}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Continuous validation drift") {
		t.Error("output file missing expected content")
	}
}
