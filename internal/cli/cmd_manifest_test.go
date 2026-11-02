package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifestNoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runManifest(nil, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected ExitError, got %d", code)
	}
}

func TestManifestHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runManifest([]string{"help"}, &stdout, &stderr)
	if code != ExitPass {
		t.Errorf("expected ExitPass, got %d", code)
	}
	if !strings.Contains(stdout.String(), "assay manifest create") {
		t.Errorf("expected usage in stdout, got: %s", stdout.String())
	}
}

func TestManifestUnknownAction(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runManifest([]string{"bogus"}, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected ExitError, got %d", code)
	}
	if !strings.Contains(stderr.String(), "unknown action") {
		t.Errorf("expected 'unknown action' in stderr, got: %s", stderr.String())
	}
}

func TestManifestCreateStdout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := manifestCreate([]string{
		"--name", "my-tool",
		"--capabilities", "http.read,db.write",
		"--version", "2.0.0",
	}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}

	var m map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &m); err != nil {
		t.Fatalf("output not valid JSON: %v\noutput: %s", err, stdout.String())
	}
	if m["name"] != "my-tool" {
		t.Errorf("name = %v, want my-tool", m["name"])
	}
	if m["version"] != "2.0.0" {
		t.Errorf("version = %v, want 2.0.0", m["version"])
	}
	caps, ok := m["capabilities"].([]any)
	if !ok || len(caps) != 2 {
		t.Fatalf("capabilities = %v, want [http.read, db.write]", m["capabilities"])
	}
	if caps[0] != "http.read" || caps[1] != "db.write" {
		t.Errorf("capabilities = %v", caps)
	}
}

func TestManifestCreateMissingFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := manifestCreate([]string{}, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected ExitError for missing flags, got %d", code)
	}
	if !strings.Contains(stderr.String(), "required") {
		t.Errorf("expected 'required' in stderr, got: %s", stderr.String())
	}
}

func TestManifestCreateToFile(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "sub", "tool.json")

	var stdout, stderr bytes.Buffer
	code := manifestCreate([]string{
		"--name", "file-tool",
		"--capabilities", "file.read",
		"--out", out,
	}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}

	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}
	if m["name"] != "file-tool" {
		t.Errorf("name = %v, want file-tool", m["name"])
	}
}

func TestManifestValidateValid(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "valid.json")
	os.WriteFile(f, []byte(`{
		"name":"test","version":"1.0.0","description":"desc",
		"risk_tier":"low","capabilities":["http.read"],"timeout_ms":5000
	}`), 0o644)

	var stdout, stderr bytes.Buffer
	code := manifestValidate([]string{"--manifest", f}, &stdout, &stderr)
	if code != ExitPass {
		t.Errorf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}
	var issues []any
	if err := json.Unmarshal(stdout.Bytes(), &issues); err != nil {
		t.Fatalf("output not valid JSON: %v\noutput: %s", err, stdout.String())
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues, got %d: %s", len(issues), stdout.String())
	}
}

func TestManifestValidateInvalid(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "bad.json")
	os.WriteFile(f, []byte(`{"name":"","version":"","description":"","risk_tier":"bogus","capabilities":[],"timeout_ms":-1}`), 0o644)

	var stdout, stderr bytes.Buffer
	code := manifestValidate([]string{"--manifest", f}, &stdout, &stderr)
	if code != ExitBlocked {
		t.Errorf("expected ExitBlocked, got %d; stderr: %s", code, stderr.String())
	}
	var issues []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &issues); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}
	if len(issues) < 4 {
		t.Errorf("expected at least 4 issues, got %d", len(issues))
	}
}

func TestManifestValidateMissingFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := manifestValidate([]string{}, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected ExitError, got %d", code)
	}
}

func TestManifestValidateMissingFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := manifestValidate([]string{"--manifest", "/nonexistent/file.json"}, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected ExitError, got %d", code)
	}
}

func TestManifestListText(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.json"), []byte(`{
		"name":"alpha","version":"1.0","description":"a","risk_tier":"low",
		"capabilities":["http.read"],"timeout_ms":5000
	}`), 0o644)
	os.WriteFile(filepath.Join(dir, "b.json"), []byte(`{
		"name":"beta","version":"2.0","description":"b","risk_tier":"high",
		"capabilities":["db.write","exec.run"],"timeout_ms":10000
	}`), 0o644)

	var stdout, stderr bytes.Buffer
	code := manifestList([]string{"--manifests", dir}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "alpha") || !strings.Contains(out, "beta") {
		t.Errorf("expected alpha and beta in output, got: %s", out)
	}
	if !strings.Contains(out, "2 manifest(s)") {
		t.Errorf("expected '2 manifest(s)' in output, got: %s", out)
	}
}

func TestManifestListJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "tool.json"), []byte(`{
		"name":"gamma","version":"3.0","description":"g","risk_tier":"medium",
		"capabilities":["file.write"],"timeout_ms":5000
	}`), 0o644)

	var stdout, stderr bytes.Buffer
	code := manifestList([]string{"--manifests", dir, "--format", "json"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}
	var entries []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &entries); err != nil {
		t.Fatalf("output not valid JSON: %v\noutput: %s", err, stdout.String())
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0]["name"] != "gamma" {
		t.Errorf("name = %v, want gamma", entries[0]["name"])
	}
}

func TestManifestListEmpty(t *testing.T) {
	dir := t.TempDir()

	var stdout, stderr bytes.Buffer
	code := manifestList([]string{"--manifests", dir}, &stdout, &stderr)
	if code != ExitError {
		t.Fatalf("expected ExitError for empty dir, got %d; stderr: %s; stdout: %s", code, stderr.String(), stdout.String())
	}
}

func TestManifestListMissingFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := manifestList([]string{}, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected ExitError, got %d", code)
	}
}

func TestSplitTrimmed(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"a,b,c", []string{"a", "b", "c"}},
		{" a , b , c ", []string{"a", "b", "c"}},
		{",,,", nil},
		{"single", []string{"single"}},
	}
	for _, tc := range tests {
		got := splitTrimmed(tc.input)
		if len(got) != len(tc.want) {
			t.Errorf("splitTrimmed(%q) len = %d, want %d", tc.input, len(got), len(tc.want))
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("splitTrimmed(%q)[%d] = %q, want %q", tc.input, i, got[i], tc.want[i])
			}
		}
	}
}
