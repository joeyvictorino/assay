package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindingsDedupBasic(t *testing.T) {
	findings := []map[string]any{
		{"checker": "c1", "class": "xss", "severity": "high", "message": "found XSS"},
		{"checker": "c1", "class": "xss", "severity": "high", "message": "found XSS"},
		{"checker": "c2", "class": "sqli", "severity": "critical", "message": "SQL injection"},
	}
	data, _ := json.Marshal(findings)
	f := filepath.Join(t.TempDir(), "findings.json")
	os.WriteFile(f, data, 0644)

	var out, errBuf bytes.Buffer
	code := runFindingsDedup([]string{"--input", f, "--format", "text"}, &out, &errBuf)
	if code != ExitPass {
		t.Fatalf("exit %d: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "Unique:     2") {
		t.Fatalf("expected 2 unique, got: %s", out.String())
	}
	if !strings.Contains(out.String(), "Duplicates: 1") {
		t.Fatalf("expected 1 duplicate, got: %s", out.String())
	}
}

func TestFindingsDedupJSON(t *testing.T) {
	findings := []map[string]any{
		{"checker": "c1", "message": "a"},
		{"checker": "c1", "message": "a"},
	}
	data, _ := json.Marshal(findings)
	f := filepath.Join(t.TempDir(), "findings.json")
	os.WriteFile(f, data, 0644)

	var out, errBuf bytes.Buffer
	code := runFindingsDedup([]string{"--input", f, "--format", "json"}, &out, &errBuf)
	if code != ExitPass {
		t.Fatalf("exit %d", code)
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if int(result["unique"].(float64)) != 1 {
		t.Fatalf("expected 1 unique: %v", result)
	}
}

func TestFindingsDedupCustomKeys(t *testing.T) {
	findings := []map[string]any{
		{"checker": "c1", "class": "xss", "severity": "high", "message": "msg1"},
		{"checker": "c1", "class": "xss", "severity": "medium", "message": "msg2"},
	}
	data, _ := json.Marshal(findings)
	f := filepath.Join(t.TempDir(), "findings.json")
	os.WriteFile(f, data, 0644)

	var out, errBuf bytes.Buffer
	code := runFindingsDedup([]string{"--input", f, "--keys", "checker,class"}, &out, &errBuf)
	if code != ExitPass {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out.String(), "Unique:     1") {
		t.Fatalf("with checker,class keys should be 1 unique: %s", out.String())
	}
}

func TestFindingsDedupNoDups(t *testing.T) {
	findings := []map[string]any{
		{"checker": "c1", "message": "a"},
		{"checker": "c2", "message": "b"},
	}
	data, _ := json.Marshal(findings)
	f := filepath.Join(t.TempDir(), "findings.json")
	os.WriteFile(f, data, 0644)

	var out, errBuf bytes.Buffer
	code := runFindingsDedup([]string{"--input", f}, &out, &errBuf)
	if code != ExitPass {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out.String(), "Duplicates: 0") {
		t.Fatalf("expected 0 duplicates: %s", out.String())
	}
}

func TestFindingsDedupOutput(t *testing.T) {
	findings := []map[string]any{
		{"checker": "c1", "message": "a"},
		{"checker": "c1", "message": "a"},
	}
	data, _ := json.Marshal(findings)
	dir := t.TempDir()
	f := filepath.Join(dir, "findings.json")
	os.WriteFile(f, data, 0644)

	outFile := filepath.Join(dir, "deduped.json")
	var out, errBuf bytes.Buffer
	code := runFindingsDedup([]string{"--input", f, "--output", outFile}, &out, &errBuf)
	if code != ExitPass {
		t.Fatalf("exit %d", code)
	}

	outData, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	var deduped []map[string]any
	json.Unmarshal(outData, &deduped)
	if len(deduped) != 1 {
		t.Fatalf("output file should have 1 finding, got %d", len(deduped))
	}
}

func TestFindingsDedupMissingInput(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := runFindingsDedup([]string{}, &out, &errBuf)
	if code != ExitError {
		t.Fatalf("expected error for missing input")
	}
}
