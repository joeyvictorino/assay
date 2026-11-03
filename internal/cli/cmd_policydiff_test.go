package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/policy"
)

func writePolicy(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPolicyDiffIdentical(t *testing.T) {
	dir := t.TempDir()
	yaml := `
version: "1"
default_effect: deny
agents:
  probe:
    max_tier: high
    allowed_tools: [http_get]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules:
  - id: allow-get
    effect: allow
    match:
      tools: [http_get]
`
	before := writePolicy(t, dir, "before.yaml", yaml)
	after := writePolicy(t, dir, "after.yaml", yaml)

	code, stdout, stderr := run(t, "policy", "diff", "--before", before, "--after", after)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d; stderr: %s", code, stderr)
	}
	if !strings.Contains(stderr, "identical") {
		t.Fatalf("expected 'identical' in stderr, got %q", stderr)
	}
	_ = stdout
}

func TestPolicyDiffChanges(t *testing.T) {
	dir := t.TempDir()
	before := writePolicy(t, dir, "before.yaml", `
version: "1"
default_effect: deny
agents:
  probe:
    max_tier: high
    allowed_tools: [http_get]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules:
  - id: allow-get
    effect: allow
    match:
      tools: [http_get]
`)
	after := writePolicy(t, dir, "after.yaml", `
version: "2"
default_effect: allow
agents:
  probe:
    max_tier: high
    allowed_tools: [http_get]
    max_delegation_depth: 0
    budget: {max_usd: 1}
  recon:
    max_tier: medium
    allowed_tools: [http_get]
    max_delegation_depth: 0
    budget: {max_usd: 0.5}
rules:
  - id: deny-all
    effect: deny
    match:
      agents: ["*"]
`)

	// Text format
	code, stdout, stderr := run(t, "policy", "diff", "--before", before, "--after", after, "--format", "text")
	if code != ExitBlocked {
		t.Fatalf("expected ExitBlocked, got %d; stderr: %s", code, stderr)
	}
	for _, want := range []string{"version:", "default_effect:", "Rules added:", "Rules removed:", "Agents added:"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("text output missing %q", want)
		}
	}

	// JSON format
	code, stdout, _ = run(t, "policy", "diff", "--before", before, "--after", after, "--format", "json")
	if code != ExitBlocked {
		t.Fatalf("expected ExitBlocked for json, got %d", code)
	}
	var report policy.DiffReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if report.Identical {
		t.Fatal("expected not identical")
	}
	if report.VersionChanged == nil {
		t.Fatal("expected version change")
	}
}

func TestPolicyDiffMissingFlags(t *testing.T) {
	code, _, stderr := run(t, "policy", "diff")
	if code != ExitError {
		t.Fatalf("expected ExitError, got %d", code)
	}
	if !strings.Contains(stderr, "--before") || !strings.Contains(stderr, "--after") {
		t.Fatalf("expected flag error, got %q", stderr)
	}
}

func TestPolicyDiffBadFile(t *testing.T) {
	dir := t.TempDir()
	before := writePolicy(t, dir, "before.yaml", `
version: "1"
default_effect: deny
agents:
  probe:
    max_tier: high
    allowed_tools: [http_get]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules: []
`)
	code, _, stderr := run(t, "policy", "diff", "--before", before, "--after", "/nonexistent/path.yaml")
	if code != ExitError {
		t.Fatalf("expected ExitError, got %d", code)
	}
	if !strings.Contains(stderr, "error") && !strings.Contains(stderr, "no such file") {
		t.Fatalf("expected file error, got %q", stderr)
	}
}
