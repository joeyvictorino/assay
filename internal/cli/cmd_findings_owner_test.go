package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindingsOwnerMissingInput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runFindingsOwner(nil, &stdout, &stderr)
	if code != ExitError {
		t.Fatalf("expected ExitError, got %d", code)
	}
}

func TestFindingsOwnerEmpty(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "findings.json")
	os.WriteFile(f, []byte("[]"), 0644)

	var stdout, stderr bytes.Buffer
	code := runFindingsOwner([]string{"--input", f}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Total findings: 0") {
		t.Errorf("unexpected output: %s", stdout.String())
	}
}

func TestFindingsOwnerText(t *testing.T) {
	findings := []ownerFinding{
		{Checker: "xss_check", Model: "claude-3", Severity: "high"},
		{Checker: "xss_check", Model: "claude-3", Severity: "medium"},
		{Checker: "sqli_check", Model: "gpt-4", Severity: "critical"},
	}
	dir := t.TempDir()
	f := filepath.Join(dir, "findings.json")
	data, _ := json.Marshal(findings)
	os.WriteFile(f, data, 0644)

	var stdout, stderr bytes.Buffer
	code := runFindingsOwner([]string{"--input", f}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "xss_check") {
		t.Errorf("missing checker: %s", out)
	}
	if !strings.Contains(out, "claude-3") {
		t.Errorf("missing model: %s", out)
	}
}

func TestFindingsOwnerJSON(t *testing.T) {
	findings := []ownerFinding{
		{Checker: "c1", Model: "m1", Severity: "high"},
		{Checker: "c1", Model: "m2", Severity: "low"},
		{Checker: "c2", Model: "m1", Severity: "high"},
	}
	dir := t.TempDir()
	f := filepath.Join(dir, "findings.json")
	data, _ := json.Marshal(findings)
	os.WriteFile(f, data, 0644)

	var stdout, stderr bytes.Buffer
	code := runFindingsOwner([]string{"--input", f, "--format", "json"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d", code)
	}
	var result findingsOwnerResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Total != 3 {
		t.Errorf("total = %d, want 3", result.Total)
	}
	if len(result.ByChecker) != 2 {
		t.Errorf("checker count = %d, want 2", len(result.ByChecker))
	}
	if len(result.ByModel) != 2 {
		t.Errorf("model count = %d, want 2", len(result.ByModel))
	}
}

func TestFindingsOwnerUnknownDefaults(t *testing.T) {
	findings := []ownerFinding{
		{Checker: "", Model: "", Severity: "info"},
	}
	result := buildFindingsOwner(findings)
	if result.ByChecker[0].Owner != "unknown" {
		t.Errorf("expected unknown checker, got %s", result.ByChecker[0].Owner)
	}
	if result.ByModel[0].Owner != "unknown" {
		t.Errorf("expected unknown model, got %s", result.ByModel[0].Owner)
	}
}
