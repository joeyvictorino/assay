package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPolicyScoreMissingFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := policyScore([]string{}, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected ExitError, got %d", code)
	}
	if !strings.Contains(stderr.String(), "required") {
		t.Errorf("expected 'required' in stderr, got: %s", stderr.String())
	}
}

func TestPolicyScoreClean(t *testing.T) {
	dir := t.TempDir()
	policyFile := filepath.Join(dir, "policy.yaml")
	manifestDir := filepath.Join(dir, "manifests")
	os.MkdirAll(manifestDir, 0o755)

	os.WriteFile(policyFile, []byte(`version: "1"
default_effect: deny
agents:
  tester:
    max_tier: low
    allowed_tools: [example]
    max_delegation_depth: 0
    budget: {max_usd: 1, max_latency_ms: 1000}
rules:
  - id: allow-example
    effect: allow
    match:
      tools: [example]
    rationale: test
`), 0o644)
	os.WriteFile(filepath.Join(manifestDir, "example.json"),
		[]byte(`{"name":"example","version":"1.0","risk_tier":"low","capabilities":["http.read"]}`), 0o644)

	var stdout, stderr bytes.Buffer
	code := policyScore([]string{
		"--policy", policyFile,
		"--manifests", manifestDir,
	}, &stdout, &stderr)
	if code != ExitPass {
		t.Errorf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Posture Score:") {
		t.Errorf("expected 'Posture Score:' in output, got: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Grade:") {
		t.Errorf("expected 'Grade:' in output, got: %s", stdout.String())
	}
}

func TestPolicyScoreJSON(t *testing.T) {
	dir := t.TempDir()
	policyFile := filepath.Join(dir, "policy.yaml")
	manifestDir := filepath.Join(dir, "manifests")
	os.MkdirAll(manifestDir, 0o755)

	os.WriteFile(policyFile, []byte(`version: "1"
default_effect: deny
agents:
  tester:
    max_tier: low
    allowed_tools: [example]
    max_delegation_depth: 0
    budget: {max_usd: 1, max_latency_ms: 1000}
rules:
  - id: allow-example
    effect: allow
    match:
      tools: [example]
    rationale: test
`), 0o644)
	os.WriteFile(filepath.Join(manifestDir, "example.json"),
		[]byte(`{"name":"example","version":"1.0","risk_tier":"low","capabilities":["http.read"]}`), 0o644)

	var stdout, stderr bytes.Buffer
	code := policyScore([]string{
		"--policy", policyFile,
		"--manifests", manifestDir,
		"--format", "json",
	}, &stdout, &stderr)
	if code != ExitPass {
		t.Errorf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, stdout.String())
	}
	if result["grade"] == nil {
		t.Error("expected 'grade' in JSON output")
	}
	if result["score"] == nil {
		t.Error("expected 'score' in JSON output")
	}
}

func TestGradeFromScore(t *testing.T) {
	tests := []struct {
		score int
		want  string
	}{
		{100, "A"},
		{90, "A"},
		{89, "B"},
		{80, "B"},
		{79, "C"},
		{70, "C"},
		{69, "D"},
		{60, "D"},
		{59, "F"},
		{0, "F"},
	}
	for _, tc := range tests {
		got := gradeFromScore(tc.score)
		if got != tc.want {
			t.Errorf("gradeFromScore(%d) = %q, want %q", tc.score, got, tc.want)
		}
	}
}
