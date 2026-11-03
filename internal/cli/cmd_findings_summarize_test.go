package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestFindingsSummarizeBasic(t *testing.T) {
	dir := t.TempDir()
	findings := []map[string]any{
		{"severity": "critical", "checker": "c1", "class": "injection", "message": "sqli"},
		{"severity": "high", "checker": "c2", "class": "auth", "message": "weak"},
		{"severity": "low", "checker": "c1", "class": "info", "message": "leak"},
	}
	data, _ := json.Marshal(findings)
	path := dir + "/f.json"
	os.WriteFile(path, data, 0644)

	var stdout, stderr bytes.Buffer
	code := runFindingsSummarize([]string{"--input", path, "--format", "json"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}

	var result summarizeResult
	json.Unmarshal(stdout.Bytes(), &result)
	if result.Total != 3 {
		t.Fatalf("expected 3 total, got %d", result.Total)
	}
	if result.Grade == "" {
		t.Fatal("expected a grade")
	}
}

func TestFindingsSummarizeText(t *testing.T) {
	dir := t.TempDir()
	findings := []map[string]any{{"severity": "medium", "checker": "ch"}}
	data, _ := json.Marshal(findings)
	path := dir + "/f.json"
	os.WriteFile(path, data, 0644)

	var stdout, stderr bytes.Buffer
	code := runFindingsSummarize([]string{"--input", path}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout.String(), "Executive Summary") {
		t.Fatal("expected executive summary header")
	}
}

func TestFindingsSummarizeClean(t *testing.T) {
	dir := t.TempDir()
	data, _ := json.Marshal([]map[string]any{})
	path := dir + "/f.json"
	os.WriteFile(path, data, 0644)

	var stdout, stderr bytes.Buffer
	code := runFindingsSummarize([]string{"--input", path, "--format", "json"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d", code)
	}
	var result summarizeResult
	json.Unmarshal(stdout.Bytes(), &result)
	if result.Grade != "A" || result.Score != 100 {
		t.Fatalf("clean scan should be A/100, got %s/%d", result.Grade, result.Score)
	}
}

func TestFindingsSummarizeMissingInput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runFindingsSummarize(nil, &stdout, &stderr)
	if code != ExitError {
		t.Fatal("expected error exit")
	}
}

func TestSummaryScore(t *testing.T) {
	tests := []struct {
		bySev map[string]int
		want  int
	}{
		{map[string]int{}, 100},
		{map[string]int{"critical": 1}, 75},
		{map[string]int{"high": 2}, 80},
		{map[string]int{"critical": 4, "high": 5}, 0},
	}
	for _, tt := range tests {
		got := summaryScore(tt.bySev)
		if got != tt.want {
			t.Errorf("summaryScore(%v) = %d, want %d", tt.bySev, got, tt.want)
		}
	}
}

func TestBuildVerdict(t *testing.T) {
	v := buildVerdict("F", map[string]int{"critical": 5}, 5)
	if !strings.Contains(v, "Critical") {
		t.Fatalf("expected critical verdict, got %q", v)
	}
	v = buildVerdict("A", nil, 1)
	if !strings.Contains(v, "Minimal") {
		t.Fatalf("expected minimal risk, got %q", v)
	}
}

func TestBuildHighlights(t *testing.T) {
	hl := buildHighlights(map[string]int{"critical": 2}, map[string]int{"a": 1}, 2)
	found := false
	for _, h := range hl {
		if strings.Contains(h, "critical") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected critical highlight")
	}
}
