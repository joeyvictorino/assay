package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/regression"
)

func setupRegressionRun(t *testing.T, dir, runID string, reflections []model.ReflectionResult) string {
	t.Helper()
	rd := filepath.Join(dir, runID)
	if err := os.MkdirAll(rd, 0o755); err != nil {
		t.Fatal(err)
	}
	rr := model.RunReport{RunID: runID, Reflection: reflections}
	writeJSONFile(t, rd, runJSON, rr)
	return rd
}

func TestRegressionCLI_MissingArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runRegression(nil, &stdout, &stderr)
	if code != ExitError {
		t.Fatalf("expected ExitError, got %d", code)
	}
}

func TestRegressionCLI_NoRegression(t *testing.T) {
	dir := t.TempDir()
	baseDir := setupRegressionRun(t, dir, "base", []model.ReflectionResult{
		{Lab: "sqli", Model: "gpt-4", Final: 8},
	})
	curDir := setupRegressionRun(t, dir, "curr", []model.ReflectionResult{
		{Lab: "sqli", Model: "gpt-4", Final: 8},
	})

	var stdout, stderr bytes.Buffer
	code := runRegression([]string{"--base", baseDir, "--current", curDir}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "No significant") {
		t.Error("expected no-change message")
	}
}

func TestRegressionCLI_WithRegression(t *testing.T) {
	dir := t.TempDir()
	baseDir := setupRegressionRun(t, dir, "base", []model.ReflectionResult{
		{Lab: "sqli", Model: "gpt-4", Final: 8},
	})
	curDir := setupRegressionRun(t, dir, "curr", []model.ReflectionResult{
		{Lab: "sqli", Model: "gpt-4", Final: 2},
	})

	var stdout, stderr bytes.Buffer
	code := runRegression([]string{"--base", baseDir, "--current", curDir}, &stdout, &stderr)
	if code != ExitFail {
		t.Fatalf("exit %d, want ExitFail; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "critical") {
		t.Error("expected critical severity in output")
	}
	if !strings.Contains(out, "Critical regressions") {
		t.Error("expected critical regressions warning")
	}
}

func TestRegressionCLI_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	baseDir := setupRegressionRun(t, dir, "base", []model.ReflectionResult{
		{Lab: "sqli", Model: "gpt-4", Final: 8},
	})
	curDir := setupRegressionRun(t, dir, "curr", []model.ReflectionResult{
		{Lab: "sqli", Model: "gpt-4", Final: 5},
	})

	var stdout, stderr bytes.Buffer
	code := runRegression([]string{"--base", baseDir, "--current", curDir, "--format", "json"}, &stdout, &stderr)
	if code == ExitError {
		t.Fatalf("exit ExitError; stderr: %s", stderr.String())
	}
	var report regression.RegressionReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if report.BaseRun != "base" || report.CurrentRun != "curr" {
		t.Errorf("run IDs = %q/%q, want base/curr", report.BaseRun, report.CurrentRun)
	}
}

func TestRegressionCLI_Threshold(t *testing.T) {
	dir := t.TempDir()
	baseDir := setupRegressionRun(t, dir, "base", []model.ReflectionResult{
		{Lab: "sqli", Model: "gpt-4", Final: 8},
	})
	curDir := setupRegressionRun(t, dir, "curr", []model.ReflectionResult{
		{Lab: "sqli", Model: "gpt-4", Final: 7},
	})

	var stdout, stderr bytes.Buffer
	code := runRegression([]string{"--base", baseDir, "--current", curDir, "--threshold", "0.15"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d, want ExitPass (below threshold); stderr: %s", code, stderr.String())
	}
}

func TestRegressionCLI_FileOutput(t *testing.T) {
	dir := t.TempDir()
	baseDir := setupRegressionRun(t, dir, "base", []model.ReflectionResult{
		{Lab: "sqli", Model: "gpt-4", Final: 8},
	})
	curDir := setupRegressionRun(t, dir, "curr", []model.ReflectionResult{
		{Lab: "sqli", Model: "gpt-4", Final: 8},
	})
	outFile := filepath.Join(dir, "out.txt")

	var stdout, stderr bytes.Buffer
	code := runRegression([]string{"--base", baseDir, "--current", curDir, "--out", outFile}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Regression Report") {
		t.Error("output file missing report header")
	}
}
