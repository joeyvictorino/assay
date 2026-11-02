package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindingsGroupMissingInput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runFindingsGroup([]string{}, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected ExitError, got %d", code)
	}
	if !strings.Contains(stderr.String(), "required") {
		t.Errorf("expected 'required' in stderr, got: %s", stderr.String())
	}
}

func TestFindingsGroupByClass(t *testing.T) {
	dir := t.TempDir()
	inputFile := filepath.Join(dir, "findings.json")
	findings := []map[string]any{
		{"id": "f1", "class": "sqli", "summary": "SQL injection"},
		{"id": "f2", "class": "xss", "summary": "Cross-site scripting"},
		{"id": "f3", "class": "sqli", "summary": "Another SQL injection"},
	}
	b, _ := json.MarshalIndent(findings, "", "  ")
	os.WriteFile(inputFile, b, 0o644)

	var stdout, stderr bytes.Buffer
	code := runFindingsGroup([]string{
		"--input", inputFile,
		"--by", "class",
	}, &stdout, &stderr)
	if code != ExitPass {
		t.Errorf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "sqli") {
		t.Errorf("expected 'sqli' in output, got: %s", out)
	}
	if !strings.Contains(out, "xss") {
		t.Errorf("expected 'xss' in output, got: %s", out)
	}
}

func TestFindingsGroupJSON(t *testing.T) {
	dir := t.TempDir()
	inputFile := filepath.Join(dir, "findings.json")
	findings := []map[string]any{
		{"id": "f1", "class": "sqli", "state": "open"},
		{"id": "f2", "class": "xss", "state": "open"},
		{"id": "f3", "class": "sqli", "state": "resolved"},
	}
	b, _ := json.MarshalIndent(findings, "", "  ")
	os.WriteFile(inputFile, b, 0o644)

	var stdout, stderr bytes.Buffer
	code := runFindingsGroup([]string{
		"--input", inputFile,
		"--by", "state",
		"--format", "json",
	}, &stdout, &stderr)
	if code != ExitPass {
		t.Errorf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}

	var result groupResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if result.Field != "state" {
		t.Errorf("expected field 'state', got %q", result.Field)
	}
	if result.Total != 3 {
		t.Errorf("expected total 3, got %d", result.Total)
	}
	if result.Groups["open"] != 2 {
		t.Errorf("expected 2 open, got %d", result.Groups["open"])
	}
	if result.Groups["resolved"] != 1 {
		t.Errorf("expected 1 resolved, got %d", result.Groups["resolved"])
	}
}

func TestFindingsGroupEmptyField(t *testing.T) {
	dir := t.TempDir()
	inputFile := filepath.Join(dir, "findings.json")
	findings := []map[string]any{
		{"id": "f1", "class": "sqli"},
		{"id": "f2"},
	}
	b, _ := json.MarshalIndent(findings, "", "  ")
	os.WriteFile(inputFile, b, 0o644)

	var stdout, stderr bytes.Buffer
	code := runFindingsGroup([]string{
		"--input", inputFile,
		"--by", "class",
	}, &stdout, &stderr)
	if code != ExitPass {
		t.Errorf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "(empty)") {
		t.Errorf("expected '(empty)' for missing field, got: %s", out)
	}
}
