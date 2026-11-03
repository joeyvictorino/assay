package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindingsStatsMissingInput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runFindingsStats([]string{}, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected ExitError, got %d", code)
	}
}

func TestFindingsStatsText(t *testing.T) {
	dir := t.TempDir()
	inputFile := filepath.Join(dir, "findings.json")
	findings := []map[string]any{
		{"id": "f1", "severity": "critical", "checker": "semgrep", "class": "sqli"},
		{"id": "f2", "severity": "low", "checker": "bandit", "class": "xss"},
		{"id": "f3", "severity": "critical", "checker": "semgrep", "class": "sqli"},
	}
	b, _ := json.MarshalIndent(findings, "", "  ")
	os.WriteFile(inputFile, b, 0o644)

	var stdout, stderr bytes.Buffer
	code := runFindingsStats([]string{
		"--input", inputFile,
	}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "3 total") {
		t.Errorf("expected '3 total' in output, got: %s", out)
	}
	if !strings.Contains(out, "critical") {
		t.Errorf("expected 'critical' in output, got: %s", out)
	}
	if !strings.Contains(out, "semgrep") {
		t.Errorf("expected 'semgrep' in output, got: %s", out)
	}
}

func TestFindingsStatsJSON(t *testing.T) {
	dir := t.TempDir()
	inputFile := filepath.Join(dir, "findings.json")
	findings := []map[string]any{
		{"id": "f1", "severity": "high", "checker": "a", "class": "sqli", "state": "open"},
		{"id": "f2", "severity": "low", "checker": "a", "class": "xss", "state": "resolved"},
	}
	b, _ := json.MarshalIndent(findings, "", "  ")
	os.WriteFile(inputFile, b, 0o644)

	var stdout, stderr bytes.Buffer
	code := runFindingsStats([]string{
		"--input", inputFile,
		"--format", "json",
	}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}

	var result findingsStatsResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("expected total 2, got %d", result.Total)
	}
	if result.BySeverity["high"] != 1 {
		t.Errorf("expected 1 high, got %d", result.BySeverity["high"])
	}
	if result.ByState["open"] != 1 {
		t.Errorf("expected 1 open, got %d", result.ByState["open"])
	}
}

func TestFindingsStatsEmpty(t *testing.T) {
	dir := t.TempDir()
	inputFile := filepath.Join(dir, "findings.json")
	os.WriteFile(inputFile, []byte("[]"), 0o644)

	var stdout, stderr bytes.Buffer
	code := runFindingsStats([]string{
		"--input", inputFile,
	}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "0 total") {
		t.Errorf("expected '0 total', got: %s", stdout.String())
	}
}
