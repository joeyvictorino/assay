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
	f1 := []map[string]any{
		{"severity": "high", "checker": "c1"},
		{"severity": "medium", "checker": "c1"},
	}
	f2 := []map[string]any{
		{"severity": "high", "checker": "c1"},
		{"severity": "high", "checker": "c2"},
		{"severity": "critical", "checker": "c2"},
	}
	d1, _ := json.Marshal(f1)
	d2, _ := json.Marshal(f2)
	os.WriteFile(filepath.Join(dir, "run1.json"), d1, 0644)
	os.WriteFile(filepath.Join(dir, "run2.json"), d2, 0644)

	var out, errBuf bytes.Buffer
	code := runFindingsTrend([]string{"--dir", dir, "--format", "text"}, &out, &errBuf)
	if code != ExitPass {
		t.Fatalf("exit %d: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "Runs analyzed: 2") {
		t.Fatalf("expected 2 runs: %s", out.String())
	}
	if !strings.Contains(out.String(), "run1.json") {
		t.Fatalf("should mention run1.json: %s", out.String())
	}
}

func TestFindingsTrendJSON(t *testing.T) {
	dir := t.TempDir()
	f1 := []map[string]any{{"severity": "high", "checker": "c1"}}
	d1, _ := json.Marshal(f1)
	os.WriteFile(filepath.Join(dir, "run1.json"), d1, 0644)

	var out, errBuf bytes.Buffer
	code := runFindingsTrend([]string{"--dir", dir, "--format", "json"}, &out, &errBuf)
	if code != ExitPass {
		t.Fatalf("exit %d", code)
	}
	var result trendResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result.TotalRuns != 1 {
		t.Fatalf("expected 1 run: %+v", result)
	}
}

func TestFindingsTrendEmpty(t *testing.T) {
	dir := t.TempDir()
	var out, errBuf bytes.Buffer
	code := runFindingsTrend([]string{"--dir", dir}, &out, &errBuf)
	if code != ExitError {
		t.Fatalf("expected error for empty dir")
	}
}

func TestFindingsTrendMissingDir(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := runFindingsTrend([]string{}, &out, &errBuf)
	if code != ExitError {
		t.Fatalf("expected error for missing dir")
	}
}

func TestFindingsTrendSortsFiles(t *testing.T) {
	dir := t.TempDir()
	f1 := []map[string]any{{"severity": "low"}}
	f2 := []map[string]any{{"severity": "high"}, {"severity": "high"}}
	d1, _ := json.Marshal(f1)
	d2, _ := json.Marshal(f2)
	os.WriteFile(filepath.Join(dir, "b_run.json"), d2, 0644)
	os.WriteFile(filepath.Join(dir, "a_run.json"), d1, 0644)

	var out, errBuf bytes.Buffer
	code := runFindingsTrend([]string{"--dir", dir, "--format", "json"}, &out, &errBuf)
	if code != ExitPass {
		t.Fatalf("exit %d", code)
	}
	var result trendResult
	json.Unmarshal(out.Bytes(), &result)
	if result.Points[0].Source != "a_run.json" {
		t.Fatalf("expected a_run.json first, got %s", result.Points[0].Source)
	}
}
