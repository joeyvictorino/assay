package promptanalyze

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestAnalyze_Empty(t *testing.T) {
	pa := Analyze(nil)
	if pa.TotalPrompts != 0 {
		t.Errorf("expected 0 prompts, got %d", pa.TotalPrompts)
	}
	if len(pa.Prompts) != 0 {
		t.Errorf("expected empty prompts, got %d", len(pa.Prompts))
	}
}

func TestAnalyze_SinglePrompt(t *testing.T) {
	findings := []model.Finding{
		{PromptHash: "abc123", Lab: "sqli", Model: model.ModelRef{Model: "gpt-4"}, Severity: model.SevHigh, Summary: "SQL injection found"},
		{PromptHash: "abc123", Lab: "xss", Model: model.ModelRef{Model: "gpt-4"}, Severity: model.SevMedium, Summary: "XSS found"},
	}
	pa := Analyze(findings)
	if pa.TotalPrompts != 1 {
		t.Fatalf("expected 1 prompt, got %d", pa.TotalPrompts)
	}
	if pa.Prompts[0].FindingCount != 2 {
		t.Errorf("finding count = %d, want 2", pa.Prompts[0].FindingCount)
	}
	if pa.Prompts[0].AvgScore != 2.5 {
		t.Errorf("avg score = %f, want 2.5", pa.Prompts[0].AvgScore)
	}
	if len(pa.Prompts[0].CheckerHits) != 2 {
		t.Errorf("checker hits = %d, want 2", len(pa.Prompts[0].CheckerHits))
	}
}

func TestAnalyze_MultiplePrompts_SortedByFindings(t *testing.T) {
	findings := []model.Finding{
		{PromptHash: "low", Lab: "sqli", Model: model.ModelRef{Model: "m1"}, Severity: model.SevLow, Summary: "a"},
		{PromptHash: "high", Lab: "sqli", Model: model.ModelRef{Model: "m1"}, Severity: model.SevHigh, Summary: "b"},
		{PromptHash: "high", Lab: "xss", Model: model.ModelRef{Model: "m1"}, Severity: model.SevHigh, Summary: "c"},
		{PromptHash: "high", Lab: "rce", Model: model.ModelRef{Model: "m2"}, Severity: model.SevCritical, Summary: "d"},
	}
	pa := Analyze(findings)
	if pa.TotalPrompts != 2 {
		t.Fatalf("expected 2 prompts, got %d", pa.TotalPrompts)
	}
	if pa.Prompts[0].PromptHash != "high" {
		t.Errorf("expected 'high' first (3 findings), got %q", pa.Prompts[0].PromptHash)
	}
	if pa.MostEffective != "high" {
		t.Errorf("most effective = %q, want high", pa.MostEffective)
	}
	if pa.LeastEffective != "low" {
		t.Errorf("least effective = %q, want low", pa.LeastEffective)
	}
}

func TestAnalyze_MultipleModels(t *testing.T) {
	findings := []model.Finding{
		{PromptHash: "p1", Model: model.ModelRef{Model: "gpt-4"}, Severity: model.SevHigh, Summary: "a"},
		{PromptHash: "p1", Model: model.ModelRef{Model: "claude-3"}, Severity: model.SevMedium, Summary: "b"},
	}
	pa := Analyze(findings)
	if len(pa.Prompts[0].Models) != 2 {
		t.Errorf("models = %d, want 2", len(pa.Prompts[0].Models))
	}
}

func TestAnalyze_NoPromptHash_Generates(t *testing.T) {
	findings := []model.Finding{
		{Lab: "sqli", Model: model.ModelRef{Model: "m1"}, Severity: model.SevHigh, Summary: "finding with no hash"},
	}
	pa := Analyze(findings)
	if pa.TotalPrompts != 1 {
		t.Fatalf("expected 1 prompt, got %d", pa.TotalPrompts)
	}
	if pa.Prompts[0].PromptHash == "" {
		t.Error("expected generated hash")
	}
}

func TestSortByScore(t *testing.T) {
	prompts := []PromptEffectiveness{
		{PromptHash: "low", AvgScore: 1.0, FindingCount: 10},
		{PromptHash: "high", AvgScore: 4.0, FindingCount: 1},
	}
	SortByScore(prompts)
	if prompts[0].PromptHash != "high" {
		t.Errorf("expected 'high' first by score, got %q", prompts[0].PromptHash)
	}
}

func TestSortByFindings(t *testing.T) {
	prompts := []PromptEffectiveness{
		{PromptHash: "few", FindingCount: 1, AvgScore: 4.0},
		{PromptHash: "many", FindingCount: 10, AvgScore: 1.0},
	}
	SortByFindings(prompts)
	if prompts[0].PromptHash != "many" {
		t.Errorf("expected 'many' first by findings, got %q", prompts[0].PromptHash)
	}
}

func TestFormatText_Empty(t *testing.T) {
	out := FormatText(PromptAnalysis{})
	if !strings.Contains(out, "No prompts") {
		t.Error("expected 'No prompts' message")
	}
}

func TestFormatText_WithData(t *testing.T) {
	pa := PromptAnalysis{
		TotalPrompts: 1,
		MostEffective: "abc123",
		LeastEffective: "abc123",
		Prompts: []PromptEffectiveness{
			{PromptHash: "abc123", PromptPreview: "test prompt", FindingCount: 5, AvgScore: 3.0, Models: []string{"m1"}},
		},
	}
	out := FormatText(pa)
	if !strings.Contains(out, "abc123") {
		t.Error("expected prompt hash in output")
	}
	if !strings.Contains(out, "Total prompts: 1") {
		t.Error("expected total prompts in output")
	}
}

func TestPreviewTruncation(t *testing.T) {
	long := strings.Repeat("x", 100)
	findings := []model.Finding{
		{PromptHash: "", Summary: long, Severity: model.SevLow},
	}
	pa := Analyze(findings)
	if len(pa.Prompts[0].PromptPreview) > 80 {
		t.Errorf("preview too long: %d chars", len(pa.Prompts[0].PromptPreview))
	}
}
