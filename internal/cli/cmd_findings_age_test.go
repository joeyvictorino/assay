package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindingsAgeMissingInput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runFindingsAge(nil, &stdout, &stderr)
	if code != ExitError {
		t.Fatalf("expected ExitError, got %d", code)
	}
}

func TestFindingsAgeEmpty(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "findings.json")
	os.WriteFile(f, []byte("[]"), 0644)

	var stdout, stderr bytes.Buffer
	code := runFindingsAge([]string{"--input", f}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Total unique findings: 0") {
		t.Errorf("unexpected output: %s", stdout.String())
	}
}

func TestFindingsAgeText(t *testing.T) {
	records := []findingRecord{
		{ID: "f1", Checker: "check_a", Severity: "high", RunDate: "2025-01-01"},
		{ID: "f1", Checker: "check_a", Severity: "high", RunDate: "2025-01-10"},
		{ID: "f2", Checker: "check_b", Severity: "low", RunDate: "2025-01-05"},
	}
	dir := t.TempDir()
	f := filepath.Join(dir, "history.json")
	data, _ := json.Marshal(records)
	os.WriteFile(f, data, 0644)

	var stdout, stderr bytes.Buffer
	code := runFindingsAge([]string{"--input", f}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Total unique findings: 2") {
		t.Errorf("missing finding count: %s", out)
	}
	if !strings.Contains(out, "9d") {
		t.Errorf("missing age display for f1 (9 days): %s", out)
	}
}

func TestFindingsAgeJSON(t *testing.T) {
	records := []findingRecord{
		{ID: "f1", Checker: "c1", Severity: "high", RunDate: "2025-01-01"},
		{ID: "f1", Checker: "c1", Severity: "high", RunDate: "2025-02-01"},
	}
	dir := t.TempDir()
	f := filepath.Join(dir, "history.json")
	data, _ := json.Marshal(records)
	os.WriteFile(f, data, 0644)

	var stdout, stderr bytes.Buffer
	code := runFindingsAge([]string{"--input", f, "--format", "json"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d", code)
	}
	var result findingsAgeResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("total = %d, want 1", result.Total)
	}
	if result.Entries[0].AgeDays != 31 {
		t.Errorf("age = %d, want 31", result.Entries[0].AgeDays)
	}
}

func TestFindingsAgeBuckets(t *testing.T) {
	records := []findingRecord{
		{ID: "new", Checker: "c", Severity: "low", RunDate: "2025-01-01"},
		{ID: "week", Checker: "c", Severity: "low", RunDate: "2025-01-01"},
		{ID: "week", Checker: "c", Severity: "low", RunDate: "2025-01-05"},
		{ID: "month", Checker: "c", Severity: "low", RunDate: "2025-01-01"},
		{ID: "month", Checker: "c", Severity: "low", RunDate: "2025-01-20"},
	}
	result := buildFindingsAge(records)
	if result.ByAge["new"] != 1 {
		t.Errorf("new bucket = %d, want 1", result.ByAge["new"])
	}
	if result.ByAge["1_week"] != 1 {
		t.Errorf("week bucket = %d, want 1", result.ByAge["1_week"])
	}
	if result.ByAge["1_month"] != 1 {
		t.Errorf("month bucket = %d, want 1", result.ByAge["1_month"])
	}
}
