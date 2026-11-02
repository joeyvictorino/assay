package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckUnknownChecker(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "tool.json")
	os.WriteFile(manifest, []byte(`{"name":"example","version":"1.0","risk_tier":"low","capabilities":["http.read"]}`), 0o644)

	var stdout, stderr bytes.Buffer
	code := runCheck([]string{"--checker", "nonexistent_checker_xyz", "--manifest", manifest}, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected ExitError for unknown checker, got %d", code)
	}
	if !strings.Contains(stderr.String(), "unknown checker") {
		t.Errorf("expected 'unknown checker' in stderr, got: %s", stderr.String())
	}
}

func TestCheckMissingManifest(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCheck([]string{"--checker", "sqli", "--manifest", "/nonexistent/path.json"}, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected ExitError for missing manifest, got %d", code)
	}
}

func TestCheckMissingFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCheck([]string{}, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected ExitError for missing flags, got %d", code)
	}
	if !strings.Contains(stderr.String(), "required") {
		t.Errorf("expected 'required' in stderr, got: %s", stderr.String())
	}
}

func TestCheckValidCheckerNoEnv(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "tool.json")
	os.WriteFile(manifest, []byte(`{"name":"example","version":"1.0","risk_tier":"low","capabilities":["http.read"]}`), 0o644)

	var stdout, stderr bytes.Buffer
	code := runCheck([]string{"--checker", "sqli", "--manifest", manifest, "--format", "text"}, &stdout, &stderr)
	if code != ExitPass {
		t.Errorf("expected ExitPass (theorized without env), got %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Checker:") {
		t.Errorf("expected 'Checker:' in output, got: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Manifest:") {
		t.Errorf("expected 'Manifest:' in output, got: %s", stdout.String())
	}
}

func TestCheckJSONFormat(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "tool.json")
	os.WriteFile(manifest, []byte(`{"name":"example","version":"1.0","risk_tier":"low","capabilities":["http.read"]}`), 0o644)

	var stdout, stderr bytes.Buffer
	code := runCheck([]string{"--checker", "sqli", "--manifest", manifest, "--format", "json"}, &stdout, &stderr)
	if code != ExitPass {
		t.Errorf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, stdout.String())
	}
	if result["checker"] != "sqli" {
		t.Errorf("expected checker 'sqli', got %v", result["checker"])
	}
}

func TestNormalizeCheckerName(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"sqli", "sqli"},
		{"xss-reflected", "xss_reflected"},
		{"XSS-Stored", "xss_stored"},
		{" CORS ", "cors"},
	}
	for _, tc := range tests {
		got := normalizeCheckerName(tc.input)
		if got != tc.want {
			t.Errorf("normalizeCheckerName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
