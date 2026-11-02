package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestFindingsTimelineBasic(t *testing.T) {
	dir := t.TempDir()
	run1 := []map[string]any{
		{"checker": "c1", "severity": "high", "message": "vuln1"},
		{"checker": "c2", "severity": "low", "message": "info1"},
	}
	run2 := []map[string]any{
		{"checker": "c1", "severity": "high", "message": "vuln1"},
		{"checker": "c3", "severity": "critical", "message": "new vuln"},
	}
	data1, _ := json.Marshal(run1)
	data2, _ := json.Marshal(run2)
	os.WriteFile(dir+"/2024-01-01.json", data1, 0644)
	os.WriteFile(dir+"/2024-01-02.json", data2, 0644)

	var stdout, stderr bytes.Buffer
	code := runFindingsTimeline([]string{"--dir", dir, "--format", "json"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}

	var result timelineResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.TotalRuns != 2 {
		t.Fatalf("expected 2 runs, got %d", result.TotalRuns)
	}
	if result.Entries[1].New != 1 {
		t.Fatalf("expected 1 new finding, got %d", result.Entries[1].New)
	}
	if result.Entries[1].Resolved != 1 {
		t.Fatalf("expected 1 resolved, got %d", result.Entries[1].Resolved)
	}
}

func TestFindingsTimelineText(t *testing.T) {
	dir := t.TempDir()
	findings := []map[string]any{{"severity": "medium", "checker": "ch", "message": "test"}}
	data, _ := json.Marshal(findings)
	os.WriteFile(dir+"/run.json", data, 0644)

	var stdout, stderr bytes.Buffer
	code := runFindingsTimeline([]string{"--dir", dir}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout.String(), "Timeline") {
		t.Fatal("expected text header")
	}
}

func TestFindingsTimelineMissingDir(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runFindingsTimeline(nil, &stdout, &stderr)
	if code != ExitError {
		t.Fatal("expected error exit")
	}
}

func TestFindingsTimelineEmptyDir(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := runFindingsTimeline([]string{"--dir", dir}, &stdout, &stderr)
	if code != ExitError {
		t.Fatal("expected error for empty dir")
	}
}

func TestFindingsTimelineNetChange(t *testing.T) {
	dir := t.TempDir()
	run1 := []map[string]any{
		{"checker": "c1", "severity": "high", "message": "a"},
		{"checker": "c2", "severity": "high", "message": "b"},
		{"checker": "c3", "severity": "high", "message": "c"},
	}
	run2 := []map[string]any{
		{"checker": "c1", "severity": "high", "message": "a"},
	}
	d1, _ := json.Marshal(run1)
	d2, _ := json.Marshal(run2)
	os.WriteFile(dir+"/01.json", d1, 0644)
	os.WriteFile(dir+"/02.json", d2, 0644)

	var stdout, stderr bytes.Buffer
	code := runFindingsTimeline([]string{"--dir", dir, "--format", "json"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d", code)
	}

	var result timelineResult
	json.Unmarshal(stdout.Bytes(), &result)
	if result.NetChange != -2 {
		t.Fatalf("expected net change -2, got %d", result.NetChange)
	}
}

func TestExtractDate(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"2024-01-15.json", "2024-01-15"},
		{"20240115.json", "20240115"},
		{"run_2024-01-15_extra.json", "2024-01-15"},
		{"arbitrary.json", "arbitrary"},
	}
	for _, tt := range tests {
		got := extractDate(tt.input)
		if got != tt.want {
			t.Errorf("extractDate(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFindingKey(t *testing.T) {
	f := map[string]any{"checker": "c1", "class": "injection", "message": "sqli"}
	key := findingKey(f)
	if key != "c1|injection|sqli" {
		t.Fatalf("unexpected key: %s", key)
	}
}

func TestFormatTimelineSev(t *testing.T) {
	m := map[string]int{"critical": 2, "low": 1}
	s := formatTimelineSev(m)
	if !strings.Contains(s, "c=2") {
		t.Fatalf("expected c=2, got %q", s)
	}
}
