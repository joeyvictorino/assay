package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindingsTrendBasic(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "run1.json"), []byte(`[
		{"checker":"sqli","severity":"high"},
		{"checker":"xss","severity":"medium"}
	]`), 0o644)
	os.WriteFile(filepath.Join(dir, "run2.json"), []byte(`[
		{"checker":"sqli","severity":"high"},
		{"checker":"sqli","severity":"high"},
		{"checker":"xss","severity":"low"}
	]`), 0o644)

	var stdout, stderr bytes.Buffer
	code := runFindingsTrend([]string{"--dir", dir, "--format", "json"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}

	var result trendResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout.String())
	}
	if result.TotalRuns != 2 {
		t.Errorf("TotalRuns = %d, want 2", result.TotalRuns)
	}
	if result.Points[0].Total != 2 {
		t.Errorf("run1 total = %d, want 2", result.Points[0].Total)
	}
	if result.Points[1].Total != 3 {
		t.Errorf("run2 total = %d, want 3", result.Points[1].Total)
	}
}

func TestFindingsTrendText(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.json"), []byte(`[{"checker":"c","severity":"low"}]`), 0o644)

	var stdout, stderr bytes.Buffer
	code := runFindingsTrend([]string{"--dir", dir, "--format", "text"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "Findings Trend Analysis") {
		t.Errorf("expected header in output, got: %s", out)
	}
	if !strings.Contains(out, "Runs analyzed: 1") {
		t.Errorf("expected 'Runs analyzed: 1', got: %s", out)
	}
}

func TestFindingsTrendMissingDir(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runFindingsTrend([]string{}, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected ExitError, got %d", code)
	}
}

func TestFindingsTrendEmptyDir(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := runFindingsTrend([]string{"--dir", dir}, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected ExitError for empty dir, got %d", code)
	}
}

func TestFindingsTrendSkipsBadFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bad.json"), []byte("not json"), 0o644)
	os.WriteFile(filepath.Join(dir, "good.json"), []byte(`[{"checker":"a","severity":"high"}]`), 0o644)

	var stdout, stderr bytes.Buffer
	code := runFindingsTrend([]string{"--dir", dir, "--format", "json"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}

	var result trendResult
	json.Unmarshal(stdout.Bytes(), &result)
	if result.TotalRuns != 1 {
		t.Errorf("TotalRuns = %d, want 1 (bad file skipped)", result.TotalRuns)
	}
}

func TestFormatSevMap(t *testing.T) {
	m := map[string]int{"high": 3, "low": 1}
	s := formatSevMap(m)
	if !strings.Contains(s, "high=3") || !strings.Contains(s, "low=1") {
		t.Errorf("formatSevMap = %q, expected high=3 and low=1", s)
	}
}
