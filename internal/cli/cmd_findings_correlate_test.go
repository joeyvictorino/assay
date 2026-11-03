package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestFindingsCorrelateBasic(t *testing.T) {
	dir := t.TempDir()
	findings := []map[string]any{
		{"severity": "high", "checker": "c1", "class": "injection", "message": "sqli"},
		{"severity": "high", "checker": "c2", "class": "injection", "message": "xss"},
		{"severity": "low", "checker": "c1", "class": "info", "message": "leak"},
	}
	data, _ := json.Marshal(findings)
	path := dir + "/f.json"
	os.WriteFile(path, data, 0644)

	var stdout, stderr bytes.Buffer
	code := runFindingsCorrelate([]string{"--input", path, "--format", "json"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}

	var result correlateResult
	json.Unmarshal(stdout.Bytes(), &result)
	if result.MultiChecker != 1 {
		t.Fatalf("expected 1 multi-checker correlation, got %d", result.MultiChecker)
	}
	if result.Correlations[0].Value != "injection" {
		t.Fatalf("expected injection, got %s", result.Correlations[0].Value)
	}
}

func TestFindingsCorrelateText(t *testing.T) {
	dir := t.TempDir()
	findings := []map[string]any{
		{"checker": "c1", "class": "auth"},
		{"checker": "c2", "class": "auth"},
	}
	data, _ := json.Marshal(findings)
	path := dir + "/f.json"
	os.WriteFile(path, data, 0644)

	var stdout, stderr bytes.Buffer
	code := runFindingsCorrelate([]string{"--input", path, "--key", "class"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout.String(), "Correlation") {
		t.Fatal("expected text header")
	}
}

func TestFindingsCorrelateNoCorrelations(t *testing.T) {
	dir := t.TempDir()
	findings := []map[string]any{
		{"checker": "c1", "class": "a"},
		{"checker": "c1", "class": "b"},
	}
	data, _ := json.Marshal(findings)
	path := dir + "/f.json"
	os.WriteFile(path, data, 0644)

	var stdout, stderr bytes.Buffer
	code := runFindingsCorrelate([]string{"--input", path, "--format", "json"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d", code)
	}
	var result correlateResult
	json.Unmarshal(stdout.Bytes(), &result)
	if result.MultiChecker != 0 {
		t.Fatalf("expected 0 correlations, got %d", result.MultiChecker)
	}
}

func TestFindingsCorrelateMissingInput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runFindingsCorrelate(nil, &stdout, &stderr)
	if code != ExitError {
		t.Fatal("expected error exit")
	}
}

func TestTruncate(t *testing.T) {
	if truncate("short", 10) != "short" {
		t.Fatal("short string should not be truncated")
	}
	long := "this is a very long string indeed"
	got := truncate(long, 15)
	if len(got) != 15 || !strings.HasSuffix(got, "...") {
		t.Fatalf("truncate failed: %q", got)
	}
}
