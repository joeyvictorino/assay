package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestFindingsPrioritizeBasic(t *testing.T) {
	dir := t.TempDir()
	findings := []map[string]any{
		{"severity": "critical", "checker": "c1", "message": "rce vuln", "class": "injection"},
		{"severity": "low", "checker": "c2", "message": "info leak", "class": "info"},
		{"severity": "high", "checker": "c1", "message": "sqli", "class": "injection"},
	}
	data, _ := json.Marshal(findings)
	path := dir + "/findings.json"
	os.WriteFile(path, data, 0644)

	var stdout, stderr bytes.Buffer
	code := runFindingsPrioritize([]string{"--input", path, "--format", "json"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}

	var result prioritizeResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Total != 3 {
		t.Fatalf("expected 3 total, got %d", result.Total)
	}
	if result.Prioritized[0].Severity != "critical" {
		t.Fatalf("expected critical first, got %s", result.Prioritized[0].Severity)
	}
}

func TestFindingsPrioritizeText(t *testing.T) {
	dir := t.TempDir()
	findings := []map[string]any{
		{"severity": "high", "checker": "ch", "message": "test finding"},
	}
	data, _ := json.Marshal(findings)
	path := dir + "/f.json"
	os.WriteFile(path, data, 0644)

	var stdout, stderr bytes.Buffer
	code := runFindingsPrioritize([]string{"--input", path}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout.String(), "Prioritization") {
		t.Fatal("expected text header")
	}
}

func TestFindingsPrioritizeMissingInput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runFindingsPrioritize(nil, &stdout, &stderr)
	if code != ExitError {
		t.Fatal("expected error exit")
	}
}

func TestFindingsPrioritizeTopN(t *testing.T) {
	dir := t.TempDir()
	var findings []map[string]any
	for i := 0; i < 20; i++ {
		findings = append(findings, map[string]any{
			"severity": "medium", "checker": "ch", "message": "finding",
		})
	}
	data, _ := json.Marshal(findings)
	path := dir + "/many.json"
	os.WriteFile(path, data, 0644)

	var stdout, stderr bytes.Buffer
	code := runFindingsPrioritize([]string{"--input", path, "--top", "5", "--format", "json"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d", code)
	}

	var result prioritizeResult
	json.Unmarshal(stdout.Bytes(), &result)
	if len(result.Prioritized) != 5 {
		t.Fatalf("expected 5, got %d", len(result.Prioritized))
	}
}

func TestSeverityScore(t *testing.T) {
	tests := []struct {
		sev  string
		want float64
	}{
		{"critical", 1.0},
		{"high", 0.8},
		{"medium", 0.5},
		{"low", 0.2},
		{"info", 0.1},
		{"unknown", 0.3},
	}
	for _, tt := range tests {
		got := severityScore(tt.sev)
		if got != tt.want {
			t.Errorf("severityScore(%q) = %f, want %f", tt.sev, got, tt.want)
		}
	}
}

func TestExploitabilityScore(t *testing.T) {
	f1 := map[string]any{"exploitable": true}
	if exploitabilityScore(f1) != 1.0 {
		t.Fatal("exploitable=true should score 1.0")
	}

	f2 := map[string]any{"class": "SQL Injection", "severity": "critical"}
	score := exploitabilityScore(f2)
	if score < 0.5 {
		t.Fatalf("injection+critical should score >0.5, got %f", score)
	}
}
