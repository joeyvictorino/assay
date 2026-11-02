package modelcontext

import (
	"strings"
	"testing"
)

func TestEmptyEntries(t *testing.T) {
	r := Analyze(nil, 0.8)
	if r.TotalRuns != 0 {
		t.Fatalf("expected 0 runs, got %d", r.TotalRuns)
	}
	if r.OverallScore != 100 {
		t.Fatalf("expected score 100, got %d", r.OverallScore)
	}
	if r.Grade != "A" {
		t.Fatalf("expected grade A, got %s", r.Grade)
	}
}

func TestSingleRunLowUtil(t *testing.T) {
	entries := []RunEntry{
		{Model: "gpt-4", PromptTokens: 1000, OutputTokens: 500, ContextWindow: 128000},
	}
	r := Analyze(entries, 0.8)
	if r.TotalRuns != 1 {
		t.Fatalf("expected 1 run, got %d", r.TotalRuns)
	}
	if len(r.Findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(r.Findings))
	}
	if r.OverallScore != 100 {
		t.Fatalf("expected score 100, got %d", r.OverallScore)
	}
}

func TestHighUtilization(t *testing.T) {
	entries := []RunEntry{
		{Model: "gpt-4", PromptTokens: 100000, OutputTokens: 20000, ContextWindow: 128000},
	}
	r := Analyze(entries, 0.8)
	found := false
	for _, f := range r.Findings {
		if f.Category == "high_utilization" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected high_utilization finding")
	}
}

func TestTruncation(t *testing.T) {
	entries := []RunEntry{
		{Model: "gpt-4", PromptTokens: 100000, OutputTokens: 28000, ContextWindow: 128000, Truncated: true},
	}
	r := Analyze(entries, 0.8)
	found := false
	for _, f := range r.Findings {
		if f.Category == "truncation" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected truncation finding")
	}
}

func TestIOImbalance(t *testing.T) {
	entries := []RunEntry{
		{Model: "gpt-4", PromptTokens: 100000, OutputTokens: 100, ContextWindow: 128000},
	}
	r := Analyze(entries, 0.8)
	found := false
	for _, f := range r.Findings {
		if f.Category == "io_imbalance" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected io_imbalance finding")
	}
}

func TestMultipleModels(t *testing.T) {
	entries := []RunEntry{
		{Model: "gpt-4", PromptTokens: 1000, OutputTokens: 500, ContextWindow: 128000},
		{Model: "claude-3", PromptTokens: 2000, OutputTokens: 1000, ContextWindow: 200000},
		{Model: "gpt-4", PromptTokens: 3000, OutputTokens: 1500, ContextWindow: 128000},
	}
	r := Analyze(entries, 0.8)
	if len(r.ModelStats) != 2 {
		t.Fatalf("expected 2 model stats, got %d", len(r.ModelStats))
	}
	if r.ModelStats[0].Model != "claude-3" {
		t.Fatalf("expected claude-3 first (sorted), got %s", r.ModelStats[0].Model)
	}
	if r.ModelStats[1].Runs != 2 {
		t.Fatalf("expected 2 runs for gpt-4, got %d", r.ModelStats[1].Runs)
	}
}

func TestDefaultContextWindow(t *testing.T) {
	entries := []RunEntry{
		{Model: "custom", PromptTokens: 1000, OutputTokens: 500, ContextWindow: 0},
	}
	r := Analyze(entries, 0.8)
	if r.ModelStats[0].AvgUtilization > 0.02 {
		t.Fatalf("utilization too high for default window: %f", r.ModelStats[0].AvgUtilization)
	}
}

func TestSeverityOrdering(t *testing.T) {
	entries := []RunEntry{
		{Model: "m1", PromptTokens: 100000, OutputTokens: 100, ContextWindow: 128000},
		{Model: "m1", PromptTokens: 126000, OutputTokens: 2000, ContextWindow: 128000, Truncated: true},
	}
	r := Analyze(entries, 0.5)
	if len(r.Findings) == 0 {
		t.Fatal("expected findings")
	}
	for i := 1; i < len(r.Findings); i++ {
		if sevOrder(r.Findings[i].Severity) < sevOrder(r.Findings[i-1].Severity) {
			t.Fatalf("findings not ordered: %s before %s",
				r.Findings[i-1].Severity, r.Findings[i].Severity)
		}
	}
}

func TestGradeRanges(t *testing.T) {
	tests := []struct {
		score int
		want  string
	}{
		{100, "A"}, {90, "A"}, {89, "B"}, {80, "B"},
		{79, "C"}, {70, "C"}, {69, "D"}, {60, "D"},
		{59, "F"}, {0, "F"},
	}
	for _, tt := range tests {
		if got := grade(tt.score); got != tt.want {
			t.Errorf("grade(%d) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil, 0.8)
	text := FormatText(r)
	if !strings.Contains(text, "Context Window") {
		t.Fatal("expected header")
	}
	if !strings.Contains(text, "No runs") {
		t.Fatal("expected no runs message")
	}
}

func TestFormatTextWithData(t *testing.T) {
	entries := []RunEntry{
		{Model: "gpt-4", PromptTokens: 100000, OutputTokens: 20000, ContextWindow: 128000},
	}
	r := Analyze(entries, 0.8)
	text := FormatText(r)
	if !strings.Contains(text, "gpt-4") {
		t.Fatal("expected model name")
	}
	if !strings.Contains(text, "utilization") {
		t.Fatal("expected utilization text")
	}
}

func TestFormatJSON(t *testing.T) {
	entries := []RunEntry{
		{Model: "gpt-4", PromptTokens: 1000, OutputTokens: 500, ContextWindow: 128000},
	}
	r := Analyze(entries, 0.8)
	j := FormatJSON(r)
	if !strings.Contains(j, `"total_runs"`) {
		t.Fatal("expected total_runs in JSON")
	}
}

func TestTruncatedPct(t *testing.T) {
	entries := []RunEntry{
		{Model: "m1", PromptTokens: 100000, OutputTokens: 28000, ContextWindow: 128000, Truncated: true},
		{Model: "m1", PromptTokens: 1000, OutputTokens: 500, ContextWindow: 128000},
	}
	r := Analyze(entries, 0.8)
	if r.ModelStats[0].TruncatedPct != 0.5 {
		t.Fatalf("expected 50%% truncation, got %f", r.ModelStats[0].TruncatedPct)
	}
}

func TestCriticalSeverity(t *testing.T) {
	entries := []RunEntry{
		{Model: "m1", PromptTokens: 120000, OutputTokens: 5000, ContextWindow: 128000},
	}
	r := Analyze(entries, 0.8)
	found := false
	for _, f := range r.Findings {
		if f.Category == "high_utilization" && f.Severity == "critical" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected critical severity for >95% utilization")
	}
}
