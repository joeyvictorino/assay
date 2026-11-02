package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/promptanalyze"
)

func TestPromptAnalyze_TextOutput(t *testing.T) {
	dir := t.TempDir()
	findings := []model.Finding{
		{PromptHash: "p1", Lab: "sqli", Model: model.ModelRef{Model: "gpt-4"}, Severity: model.SevHigh, Summary: "SQL injection in login"},
		{PromptHash: "p1", Lab: "xss", Model: model.ModelRef{Model: "gpt-4"}, Severity: model.SevMedium, Summary: "XSS in search"},
		{PromptHash: "p2", Lab: "sqli", Model: model.ModelRef{Model: "claude-3"}, Severity: model.SevLow, Summary: "Minor issue"},
	}
	setupCompareRun(t, dir, "run-1", findings)
	runDir := dir + "/run-1"

	var stdout, stderr bytes.Buffer
	code := runPromptAnalyze([]string{"--path", runDir}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Prompt Effectiveness") {
		t.Errorf("missing header:\n%s", out)
	}
	if !strings.Contains(out, "p1") {
		t.Errorf("missing prompt hash p1:\n%s", out)
	}
}

func TestPromptAnalyze_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	findings := []model.Finding{
		{PromptHash: "p1", Lab: "sqli", Model: model.ModelRef{Model: "gpt-4"}, Severity: model.SevHigh, Summary: "Finding 1"},
		{PromptHash: "p2", Lab: "xss", Model: model.ModelRef{Model: "gpt-4"}, Severity: model.SevMedium, Summary: "Finding 2"},
	}
	setupCompareRun(t, dir, "run-1", findings)
	runDir := dir + "/run-1"

	var stdout, stderr bytes.Buffer
	code := runPromptAnalyze([]string{"--path", runDir, "--format", "json"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}

	var pa promptanalyze.PromptAnalysis
	if err := json.Unmarshal(stdout.Bytes(), &pa); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if pa.TotalPrompts != 2 {
		t.Errorf("total prompts = %d, want 2", pa.TotalPrompts)
	}
}

func TestPromptAnalyze_MissingPath(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runPromptAnalyze(nil, &stdout, &stderr)
	if code != ExitError {
		t.Fatalf("expected exit %d, got %d", ExitError, code)
	}
}

func TestPromptAnalyze_SortByScore(t *testing.T) {
	dir := t.TempDir()
	findings := []model.Finding{
		{PromptHash: "low-findings-high-score", Lab: "sqli", Model: model.ModelRef{Model: "m1"}, Severity: model.SevCritical, Summary: "Critical"},
		{PromptHash: "high-findings-low-score", Lab: "sqli", Model: model.ModelRef{Model: "m1"}, Severity: model.SevLow, Summary: "Low 1"},
		{PromptHash: "high-findings-low-score", Lab: "xss", Model: model.ModelRef{Model: "m1"}, Severity: model.SevLow, Summary: "Low 2"},
	}
	setupCompareRun(t, dir, "run-1", findings)
	runDir := dir + "/run-1"

	var stdout, stderr bytes.Buffer
	code := runPromptAnalyze([]string{"--path", runDir, "--format", "json", "--sort", "score"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}

	var pa promptanalyze.PromptAnalysis
	if err := json.Unmarshal(stdout.Bytes(), &pa); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if pa.Prompts[0].PromptHash != "low-findings-high-score" {
		t.Errorf("expected high-score prompt first when sorting by score, got %q", pa.Prompts[0].PromptHash)
	}
}

func TestPromptAnalyze_NoFindings(t *testing.T) {
	dir := t.TempDir()
	setupCompareRun(t, dir, "run-1", nil)
	runDir := dir + "/run-1"

	var stdout, stderr bytes.Buffer
	code := runPromptAnalyze([]string{"--path", runDir}, &stdout, &stderr)
	if code != ExitError {
		t.Fatalf("expected exit %d for missing findings, got %d", ExitError, code)
	}
}
