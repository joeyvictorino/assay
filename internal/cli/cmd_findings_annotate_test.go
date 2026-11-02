package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindingsAnnotateMissingFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runFindingsAnnotate([]string{}, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected ExitError, got %d", code)
	}
}

func TestFindingsAnnotateValid(t *testing.T) {
	dir := t.TempDir()
	inputFile := filepath.Join(dir, "findings.json")
	findings := []map[string]any{
		{"id": "f1", "class": "sqli", "summary": "SQL injection risk"},
		{"id": "f2", "class": "xss", "summary": "XSS vulnerability"},
	}
	b, _ := json.MarshalIndent(findings, "", "  ")
	os.WriteFile(inputFile, b, 0o644)

	var stdout, stderr bytes.Buffer
	code := runFindingsAnnotate([]string{
		"add",
		"--input", inputFile,
		"--finding-id", "f1",
		"--note", "Confirmed by manual review",
		"--author", "alice",
	}, &stdout, &stderr)
	if code != ExitPass {
		t.Errorf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Added annotation") {
		t.Errorf("expected confirmation message, got: %s", stdout.String())
	}
}

func TestFindingsAnnotateUnknownFinding(t *testing.T) {
	dir := t.TempDir()
	inputFile := filepath.Join(dir, "findings.json")
	findings := []map[string]any{
		{"id": "f1", "class": "sqli", "summary": "test"},
	}
	b, _ := json.MarshalIndent(findings, "", "  ")
	os.WriteFile(inputFile, b, 0o644)

	var stdout, stderr bytes.Buffer
	code := runFindingsAnnotate([]string{
		"add",
		"--input", inputFile,
		"--finding-id", "nonexistent",
		"--note", "test",
	}, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected ExitError for unknown finding, got %d", code)
	}
	if !strings.Contains(stderr.String(), "not found") {
		t.Errorf("expected 'not found' in stderr, got: %s", stderr.String())
	}
}

func TestFindingsAnnotateSeparateOutput(t *testing.T) {
	dir := t.TempDir()
	inputFile := filepath.Join(dir, "findings.json")
	outputFile := filepath.Join(dir, "annotated.json")
	findings := []map[string]any{
		{"id": "f1", "class": "sqli", "summary": "test"},
	}
	b, _ := json.MarshalIndent(findings, "", "  ")
	os.WriteFile(inputFile, b, 0o644)

	var stdout, stderr bytes.Buffer
	code := runFindingsAnnotate([]string{
		"add",
		"--input", inputFile,
		"--output", outputFile,
		"--finding-id", "f1",
		"--note", "reviewed",
	}, &stdout, &stderr)
	if code != ExitPass {
		t.Errorf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}

	if _, err := os.Stat(outputFile); err != nil {
		t.Errorf("output file not created: %v", err)
	}
}

func TestFindingsAnnotateList(t *testing.T) {
	dir := t.TempDir()
	inputFile := filepath.Join(dir, "findings.json")
	findings := []map[string]any{
		{"id": "f1", "class": "sqli", "summary": "test"},
	}
	b, _ := json.MarshalIndent(findings, "", "  ")
	os.WriteFile(inputFile, b, 0o644)

	runFindingsAnnotate([]string{
		"add", "--input", inputFile, "--finding-id", "f1",
		"--note", "test note", "--author", "bob",
	}, &bytes.Buffer{}, &bytes.Buffer{})

	var stdout, stderr bytes.Buffer
	code := runFindingsAnnotate([]string{
		"list", "--input", inputFile,
	}, &stdout, &stderr)
	if code != ExitPass {
		t.Errorf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "test note") {
		t.Errorf("expected note text in list output, got: %s", stdout.String())
	}
}

func TestFindingsAnnotateSummary(t *testing.T) {
	dir := t.TempDir()
	inputFile := filepath.Join(dir, "findings.json")
	findings := []map[string]any{
		{"id": "f1", "class": "sqli", "summary": "test"},
	}
	b, _ := json.MarshalIndent(findings, "", "  ")
	os.WriteFile(inputFile, b, 0o644)

	runFindingsAnnotate([]string{
		"add", "--input", inputFile, "--finding-id", "f1",
		"--note", "reviewed", "--status", "confirmed",
	}, &bytes.Buffer{}, &bytes.Buffer{})

	var stdout, stderr bytes.Buffer
	code := runFindingsAnnotate([]string{
		"summary", "--input", inputFile,
	}, &stdout, &stderr)
	if code != ExitPass {
		t.Errorf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Annotations:") {
		t.Errorf("expected summary output, got: %s", stdout.String())
	}
}
