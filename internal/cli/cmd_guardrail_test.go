package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/guardrail"
	"github.com/joeyvictorino/assay/internal/model"
)

func setupGuardrailRun(t *testing.T, dir string, reflections []model.ReflectionResult, findings []model.Finding) string {
	t.Helper()
	rd := filepath.Join(dir, "run1")
	if err := os.MkdirAll(rd, 0o755); err != nil {
		t.Fatal(err)
	}
	rr := model.RunReport{RunID: "run1", Reflection: reflections}
	writeJSONFile(t, rd, runJSON, rr)
	if findings != nil {
		writeJSONFile(t, rd, findingsJSON, findings)
	}
	return rd
}

func TestGuardrailCLI_MissingInput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runGuardrail(nil, &stdout, &stderr)
	if code != ExitError {
		t.Fatalf("expected ExitError, got %d", code)
	}
}

func TestGuardrailCLI_EmptyRun(t *testing.T) {
	dir := t.TempDir()
	rd := setupGuardrailRun(t, dir, nil, nil)

	var stdout, stderr bytes.Buffer
	code := runGuardrail([]string{"--input", rd}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Guardrail") {
		t.Error("missing report header")
	}
}

func TestGuardrailCLI_TextOutput(t *testing.T) {
	dir := t.TempDir()
	rd := setupGuardrailRun(t, dir, []model.ReflectionResult{
		{Lab: "prompt-injection-test", Passed: true, Final: 8},
		{Lab: "data-leakage-check", Passed: false, Final: 3, Revisions: 2},
	}, nil)

	var stdout, stderr bytes.Buffer
	code := runGuardrail([]string{"--input", rd}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "prompt_injection") {
		t.Error("missing prompt_injection category")
	}
	if !strings.Contains(out, "data_leakage") {
		t.Error("missing data_leakage category")
	}
}

func TestGuardrailCLI_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	rd := setupGuardrailRun(t, dir, []model.ReflectionResult{
		{Lab: "injection-a", Passed: true, Final: 9},
		{Lab: "injection-b", Passed: false, Final: 4},
	}, nil)

	var stdout, stderr bytes.Buffer
	code := runGuardrail([]string{"--input", rd, "--format", "json"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	var report guardrail.GuardrailReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(report.Categories) == 0 {
		t.Error("expected at least one category")
	}
	if report.OverallPass < 0 || report.OverallPass > 1 {
		t.Errorf("overall pass rate out of range: %f", report.OverallPass)
	}
}

func TestGuardrailCLI_WithFindings(t *testing.T) {
	dir := t.TempDir()
	findings := []model.Finding{
		{Lab: "info-disclosure-test", State: model.StateValidated, Severity: model.SevHigh},
	}
	rd := setupGuardrailRun(t, dir, nil, findings)

	var stdout, stderr bytes.Buffer
	code := runGuardrail([]string{"--input", rd}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "data_leakage") {
		t.Error("expected data_leakage from info-disclosure finding")
	}
}

func TestGuardrailCLI_FileOutput(t *testing.T) {
	dir := t.TempDir()
	rd := setupGuardrailRun(t, dir, []model.ReflectionResult{
		{Lab: "injection-a", Passed: true, Final: 9},
	}, nil)
	outFile := filepath.Join(dir, "out.txt")

	var stdout, stderr bytes.Buffer
	code := runGuardrail([]string{"--input", rd, "--out", outFile}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Guardrail") {
		t.Error("output file missing report header")
	}
}
