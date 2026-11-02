package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvTextNoPolicy(t *testing.T) {
	orig, _ := os.Getwd()
	dir := t.TempDir()
	os.Chdir(dir)
	defer os.Chdir(orig)

	var stdout, stderr bytes.Buffer
	code := runEnv([]string{"--format", "text"}, &stdout, &stderr)
	if code != ExitBlocked {
		t.Errorf("expected ExitBlocked (issues present), got %d; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Version:") {
		t.Errorf("expected 'Version:' in output, got: %s", out)
	}
	if !strings.Contains(out, "Checkers:") {
		t.Errorf("expected 'Checkers:' in output, got: %s", out)
	}
	if !strings.Contains(out, "Policy:     (none)") {
		t.Errorf("expected 'Policy:     (none)' in output, got: %s", out)
	}
	if !strings.Contains(out, "no policy file found") {
		t.Errorf("expected 'no policy file found' in issues, got: %s", out)
	}
}

func TestEnvJSONNoPolicy(t *testing.T) {
	orig, _ := os.Getwd()
	dir := t.TempDir()
	os.Chdir(dir)
	defer os.Chdir(orig)

	var stdout, stderr bytes.Buffer
	code := runEnv([]string{"--format", "json"}, &stdout, &stderr)
	if code != ExitBlocked {
		t.Errorf("expected ExitBlocked, got %d", code)
	}

	var report EnvReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("output not valid JSON: %v\noutput: %s", err, stdout.String())
	}
	if report.CheckerCount == 0 {
		t.Errorf("expected at least one checker")
	}
	if len(report.Checkers) != report.CheckerCount {
		t.Errorf("checker list length %d != count %d", len(report.Checkers), report.CheckerCount)
	}
	if len(report.Issues) == 0 {
		t.Errorf("expected issues when no policy/manifests found")
	}
}

func TestEnvWithPolicyFlag(t *testing.T) {
	orig, _ := os.Getwd()
	dir := t.TempDir()
	os.Chdir(dir)
	defer os.Chdir(orig)

	pf := filepath.Join(dir, "policy.yaml")
	os.WriteFile(pf, []byte("version: \"1\"\ndefault_effect: allow\nrules: []\nagents:\n  default:\n    allowed_tools: [\"*\"]\n    max_tier: critical\n    budget:\n      max_usd: 10\n      max_latency_ms: 60000\n"), 0o644)

	var stdout, stderr bytes.Buffer
	code := runEnv([]string{"--policy", pf, "--format", "json"}, &stdout, &stderr)
	if code != ExitBlocked {
		t.Errorf("expected ExitBlocked (no manifests), got %d; stderr: %s", code, stderr.String())
	}

	var report EnvReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}
	if !report.PolicyValid {
		t.Errorf("expected PolicyValid=true, issues: %v", report.Issues)
	}
	if report.PolicyPath != pf {
		t.Errorf("PolicyPath = %q, want %q", report.PolicyPath, pf)
	}
}

func TestEnvWithManifestsFlag(t *testing.T) {
	orig, _ := os.Getwd()
	dir := t.TempDir()
	os.Chdir(dir)
	defer os.Chdir(orig)

	mDir := filepath.Join(dir, "manifests")
	os.MkdirAll(mDir, 0o755)
	os.WriteFile(filepath.Join(mDir, "a.json"), []byte(`{"name":"a"}`), 0o644)
	os.WriteFile(filepath.Join(mDir, "b.json"), []byte(`{"name":"b"}`), 0o644)

	var stdout, stderr bytes.Buffer
	code := runEnv([]string{"--manifests", mDir, "--format", "json"}, &stdout, &stderr)

	var report EnvReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}
	if report.ManifestCount != 2 {
		t.Errorf("ManifestCount = %d, want 2", report.ManifestCount)
	}
	if report.ManifestDir != mDir {
		t.Errorf("ManifestDir = %q, want %q", report.ManifestDir, mDir)
	}
	_ = code
}

func TestEnvAutoDiscoverPolicy(t *testing.T) {
	orig, _ := os.Getwd()
	dir := t.TempDir()
	os.Chdir(dir)
	defer os.Chdir(orig)

	os.WriteFile(filepath.Join(dir, "policy.yaml"), []byte("version: \"1\"\ndefault_effect: allow\nrules: []\nagents:\n  default:\n    allowed_tools: [\"*\"]\n    max_tier: critical\n    budget:\n      max_usd: 10\n      max_latency_ms: 60000\n"), 0o644)

	var stdout, stderr bytes.Buffer
	runEnv([]string{"--format", "json"}, &stdout, &stderr)

	var report EnvReport
	json.Unmarshal(stdout.Bytes(), &report)
	if report.PolicyPath != "policy.yaml" {
		t.Errorf("expected auto-discovered 'policy.yaml', got %q", report.PolicyPath)
	}
	if !report.PolicyValid {
		t.Errorf("expected PolicyValid=true, issues: %v", report.Issues)
	}
}

func TestEnvAutoDiscoverManifests(t *testing.T) {
	orig, _ := os.Getwd()
	dir := t.TempDir()
	os.Chdir(dir)
	defer os.Chdir(orig)

	mDir := filepath.Join(dir, "manifests")
	os.MkdirAll(mDir, 0o755)
	os.WriteFile(filepath.Join(mDir, "tool.json"), []byte(`{"name":"t"}`), 0o644)

	var stdout, stderr bytes.Buffer
	runEnv([]string{"--format", "json"}, &stdout, &stderr)

	var report EnvReport
	json.Unmarshal(stdout.Bytes(), &report)
	if report.ManifestDir != "manifests" {
		t.Errorf("expected auto-discovered 'manifests', got %q", report.ManifestDir)
	}
	if report.ManifestCount != 1 {
		t.Errorf("ManifestCount = %d, want 1", report.ManifestCount)
	}
}

func TestEnvBadPolicyPath(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runEnv([]string{"--policy", "/nonexistent/policy.yaml", "--format", "json"}, &stdout, &stderr)
	if code != ExitBlocked {
		t.Errorf("expected ExitBlocked, got %d", code)
	}
	var report EnvReport
	json.Unmarshal(stdout.Bytes(), &report)
	if report.PolicyValid {
		t.Errorf("expected PolicyValid=false for nonexistent file")
	}
}

func TestEnvBadManifestsDir(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runEnv([]string{"--manifests", "/nonexistent/dir", "--format", "json"}, &stdout, &stderr)
	if code != ExitBlocked {
		t.Errorf("expected ExitBlocked, got %d", code)
	}
	var report EnvReport
	json.Unmarshal(stdout.Bytes(), &report)
	found := false
	for _, issue := range report.Issues {
		if strings.Contains(issue, "manifests:") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected manifests issue, got: %v", report.Issues)
	}
}

func TestEnvUnknownFormat(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runEnv([]string{"--format", "xml"}, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected ExitError, got %d", code)
	}
	if !strings.Contains(stderr.String(), "unknown format") {
		t.Errorf("expected 'unknown format' in stderr, got: %s", stderr.String())
	}
}

func TestCountManifests(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.json"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.json"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# readme"), 0o644)

	count, err := countManifests(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

func TestCountManifestsEmpty(t *testing.T) {
	dir := t.TempDir()
	count, err := countManifests(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

func TestCountManifestsBadDir(t *testing.T) {
	_, err := countManifests("/nonexistent/dir")
	if err == nil {
		t.Errorf("expected error for nonexistent dir")
	}
}
