package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindingsFilterMissingInput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runFindingsFilter([]string{}, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected ExitError, got %d", code)
	}
}

func TestFindingsFilterBySeverity(t *testing.T) {
	dir := t.TempDir()
	inputFile := filepath.Join(dir, "findings.json")
	findings := []map[string]any{
		{"id": "f1", "severity": "critical", "class": "sqli"},
		{"id": "f2", "severity": "low", "class": "xss"},
		{"id": "f3", "severity": "high", "class": "sqli"},
	}
	b, _ := json.MarshalIndent(findings, "", "  ")
	os.WriteFile(inputFile, b, 0o644)

	var stdout, stderr bytes.Buffer
	code := runFindingsFilter([]string{
		"--input", inputFile,
		"--severity", "high",
	}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "2/3") {
		t.Errorf("expected 2/3 matched, got: %s", out)
	}
}

func TestFindingsFilterByChecker(t *testing.T) {
	dir := t.TempDir()
	inputFile := filepath.Join(dir, "findings.json")
	findings := []map[string]any{
		{"id": "f1", "checker": "semgrep", "severity": "high"},
		{"id": "f2", "checker": "bandit", "severity": "medium"},
		{"id": "f3", "checker": "semgrep", "severity": "low"},
	}
	b, _ := json.MarshalIndent(findings, "", "  ")
	os.WriteFile(inputFile, b, 0o644)

	var stdout, stderr bytes.Buffer
	code := runFindingsFilter([]string{
		"--input", inputFile,
		"--checker", "semgrep",
	}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "2/3") {
		t.Errorf("expected 2/3 matched, got: %s", stdout.String())
	}
}

func TestFindingsFilterInvert(t *testing.T) {
	dir := t.TempDir()
	inputFile := filepath.Join(dir, "findings.json")
	findings := []map[string]any{
		{"id": "f1", "severity": "critical"},
		{"id": "f2", "severity": "low"},
	}
	b, _ := json.MarshalIndent(findings, "", "  ")
	os.WriteFile(inputFile, b, 0o644)

	var stdout, stderr bytes.Buffer
	code := runFindingsFilter([]string{
		"--input", inputFile,
		"--severity", "critical",
		"--invert",
	}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "1/2") {
		t.Errorf("expected 1/2 matched, got: %s", stdout.String())
	}
}

func TestFindingsFilterJSON(t *testing.T) {
	dir := t.TempDir()
	inputFile := filepath.Join(dir, "findings.json")
	findings := []map[string]any{
		{"id": "f1", "severity": "high"},
	}
	b, _ := json.MarshalIndent(findings, "", "  ")
	os.WriteFile(inputFile, b, 0o644)

	var stdout, stderr bytes.Buffer
	code := runFindingsFilter([]string{
		"--input", inputFile,
		"--format", "json",
	}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}

	var result filterResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout.String())
	}
	if result.Total != 1 {
		t.Errorf("expected total 1, got %d", result.Total)
	}
}

func TestFindingsFilterByField(t *testing.T) {
	dir := t.TempDir()
	inputFile := filepath.Join(dir, "findings.json")
	findings := []map[string]any{
		{"id": "f1", "state": "open"},
		{"id": "f2", "state": "resolved"},
		{"id": "f3", "state": "open"},
	}
	b, _ := json.MarshalIndent(findings, "", "  ")
	os.WriteFile(inputFile, b, 0o644)

	var stdout, stderr bytes.Buffer
	code := runFindingsFilter([]string{
		"--input", inputFile,
		"--field", "state=open",
	}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "2/3") {
		t.Errorf("expected 2/3 matched, got: %s", stdout.String())
	}
}
