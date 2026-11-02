package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindingsExportCSV(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "findings.json")
	os.WriteFile(input, []byte(`[
		{"checker":"sqli","class":"injection","severity":"high","message":"sql injection"},
		{"checker":"xss","class":"xss","severity":"medium","message":"reflected xss"}
	]`), 0o644)

	var stdout, stderr bytes.Buffer
	code := runFindingsExport([]string{"--input", input, "--format", "csv"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines (header + 2 rows), got %d: %s", len(lines), stdout.String())
	}
	if !strings.Contains(lines[0], "checker") {
		t.Errorf("expected header with 'checker', got: %s", lines[0])
	}
}

func TestFindingsExportSARIF(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "findings.json")
	os.WriteFile(input, []byte(`[{"checker":"sqli","class":"inj","severity":"high","message":"test"}]`), 0o644)

	var stdout, stderr bytes.Buffer
	code := runFindingsExport([]string{"--input", input, "--format", "sarif"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d", code)
	}
	var sarif map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &sarif); err != nil {
		t.Fatalf("invalid SARIF JSON: %v", err)
	}
	if sarif["version"] != "2.1.0" {
		t.Errorf("version = %v, want 2.1.0", sarif["version"])
	}
	runs := sarif["runs"].([]any)
	results := runs[0].(map[string]any)["results"].([]any)
	if len(results) != 1 {
		t.Errorf("results count = %d, want 1", len(results))
	}
}

func TestFindingsExportMarkdown(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "findings.json")
	os.WriteFile(input, []byte(`[{"checker":"xss","class":"xss","severity":"medium","message":"reflected"}]`), 0o644)

	var stdout, stderr bytes.Buffer
	code := runFindingsExport([]string{"--input", input, "--format", "markdown"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "| Checker |") {
		t.Errorf("expected markdown header, got: %s", out)
	}
	if !strings.Contains(out, "| xss |") {
		t.Errorf("expected xss row, got: %s", out)
	}
}

func TestFindingsExportToFile(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "findings.json")
	output := filepath.Join(dir, "out.csv")
	os.WriteFile(input, []byte(`[{"checker":"a","class":"b","severity":"low","message":"c"}]`), 0o644)

	var stdout, stderr bytes.Buffer
	code := runFindingsExport([]string{"--input", input, "--format", "csv", "--output", output}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d", code)
	}
	raw, _ := os.ReadFile(output)
	if !strings.Contains(string(raw), "checker") {
		t.Errorf("expected CSV header in output file")
	}
}

func TestFindingsExportMissingFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runFindingsExport([]string{}, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected ExitError, got %d", code)
	}
}

func TestFindingsExportUnknownFormat(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "findings.json")
	os.WriteFile(input, []byte(`[]`), 0o644)

	var stdout, stderr bytes.Buffer
	code := runFindingsExport([]string{"--input", input, "--format", "xml"}, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected ExitError, got %d", code)
	}
}

func TestSarifLevel(t *testing.T) {
	tests := []struct{ sev, want string }{
		{"critical", "error"},
		{"high", "error"},
		{"medium", "warning"},
		{"low", "note"},
		{"info", "note"},
	}
	for _, tc := range tests {
		got := sarifLevel(tc.sev)
		if got != tc.want {
			t.Errorf("sarifLevel(%q) = %q, want %q", tc.sev, got, tc.want)
		}
	}
}

func TestEscMd(t *testing.T) {
	if got := escMd("a|b\nc"); got != "a\\|b c" {
		t.Errorf("escMd = %q", got)
	}
}
