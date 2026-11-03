package maxtokens

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func findings(entries ...model.Finding) []model.Finding {
	return entries
}

func finding(mdl string, tokens int, sev model.Severity) model.Finding {
	return model.Finding{
		Model:     model.ModelRef{Model: mdl},
		MaxTokens: tokens,
		Severity:  sev,
	}
}

func TestEmpty(t *testing.T) {
	r := Analyze(nil)
	if r.TotalFindings != 0 {
		t.Fatalf("want 0, got %d", r.TotalFindings)
	}
}

func TestSingleFinding(t *testing.T) {
	r := Analyze(findings(finding("gpt-4", 1024, model.SevMedium)))
	if r.TotalFindings != 1 {
		t.Fatalf("want 1, got %d", r.TotalFindings)
	}
	if len(r.Overall) != 1 {
		t.Fatalf("want 1 bucket, got %d", len(r.Overall))
	}
	if r.Overall[0].MaxTokens != 1024 {
		t.Fatalf("want tokens 1024, got %d", r.Overall[0].MaxTokens)
	}
}

func TestMultipleBuckets(t *testing.T) {
	r := Analyze(findings(
		finding("gpt-4", 256, model.SevLow),
		finding("gpt-4", 1024, model.SevMedium),
		finding("gpt-4", 4096, model.SevHigh),
	))
	if len(r.Overall) != 3 {
		t.Fatalf("want 3 buckets, got %d", len(r.Overall))
	}
	if r.OptimalTokens != 256 {
		t.Fatalf("want optimal at 256, got %d", r.OptimalTokens)
	}
}

func TestByModel(t *testing.T) {
	r := Analyze(findings(
		finding("gpt-4", 1024, model.SevLow),
		finding("claude-3", 1024, model.SevMedium),
	))
	if len(r.ByModel) != 2 {
		t.Fatalf("want 2 models, got %d", len(r.ByModel))
	}
}

func TestQualityScore(t *testing.T) {
	r := Analyze(findings(finding("gpt-4", 1024, model.SevCritical)))
	if r.Overall[0].QualityScore != 0.0 {
		t.Fatalf("want quality 0.0 for critical, got %.2f", r.Overall[0].QualityScore)
	}

	r2 := Analyze(findings(finding("gpt-4", 1024, model.SevLow)))
	if r2.Overall[0].QualityScore < 0.7 {
		t.Fatalf("want quality > 0.7 for low, got %.2f", r2.Overall[0].QualityScore)
	}
}

func TestZeroTokensSkipped(t *testing.T) {
	r := Analyze(findings(finding("gpt-4", 0, model.SevMedium)))
	if len(r.Overall) != 0 {
		t.Fatalf("want 0 buckets for zero-token finding, got %d", len(r.Overall))
	}
	if r.Recommendation != "No max_tokens data available" {
		t.Fatalf("want no-data recommendation, got %q", r.Recommendation)
	}
}

func TestRecommendation(t *testing.T) {
	r := Analyze(findings(finding("gpt-4", 1024, model.SevLow)))
	if !strings.Contains(r.Recommendation, "1024") {
		t.Fatalf("recommendation should mention optimal tokens, got %q", r.Recommendation)
	}
}

func TestFormatText(t *testing.T) {
	r := Analyze(findings(finding("gpt-4", 1024, model.SevMedium)))
	text := FormatText(r)
	if !strings.Contains(text, "Max-Tokens") {
		t.Fatal("should contain header")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze(findings(finding("gpt-4", 1024, model.SevMedium)))
	j := FormatJSON(r)
	if !strings.Contains(j, "optimal_max_tokens") {
		t.Fatal("should contain optimal_max_tokens field")
	}
}

func TestSeverityToNum(t *testing.T) {
	tests := []struct {
		sev  model.Severity
		want float64
	}{
		{model.SevCritical, 4},
		{model.SevHigh, 3},
		{model.SevMedium, 2},
		{model.SevLow, 1},
		{model.Severity("unknown"), 0},
	}
	for _, tt := range tests {
		got := severityToNum(tt.sev)
		if got != tt.want {
			t.Errorf("severityToNum(%q) = %.0f, want %.0f", tt.sev, got, tt.want)
		}
	}
}
