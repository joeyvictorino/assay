package promptlength

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(summary string, sev model.Severity, m, provider string) model.Finding {
	return model.Finding{
		Summary:  summary,
		Severity: sev,
		Model:    model.ModelRef{Model: m, Provider: provider},
		State:    model.StateValidated,
	}
}

func TestEmptyFindings(t *testing.T) {
	r := Analyze(nil)
	if r.Summary.TotalFindings != 0 {
		t.Fatalf("expected 0, got %d", r.Summary.TotalFindings)
	}
}

func TestSingleFinding(t *testing.T) {
	ff := []model.Finding{
		finding("short summary", model.SevHigh, "gpt-4", "openai"),
	}
	r := Analyze(ff)
	if r.Summary.TotalFindings != 1 {
		t.Fatalf("expected 1, got %d", r.Summary.TotalFindings)
	}
}

func TestLengthBuckets(t *testing.T) {
	ff := []model.Finding{
		finding(strings.Repeat("a", 50), model.SevLow, "m", "p"),
		finding(strings.Repeat("b", 150), model.SevMedium, "m", "p"),
		finding(strings.Repeat("c", 300), model.SevHigh, "m", "p"),
		finding(strings.Repeat("d", 600), model.SevCritical, "m", "p"),
	}
	r := Analyze(ff)
	if r.Summary.ShortCount != 1 {
		t.Fatalf("expected 1 short, got %d", r.Summary.ShortCount)
	}
	if r.Summary.MediumCount != 1 {
		t.Fatalf("expected 1 medium, got %d", r.Summary.MediumCount)
	}
	if r.Summary.LongCount != 1 {
		t.Fatalf("expected 1 long, got %d", r.Summary.LongCount)
	}
	if r.Summary.VeryLongCount != 1 {
		t.Fatalf("expected 1 very long, got %d", r.Summary.VeryLongCount)
	}
}

func TestBucketSeverity(t *testing.T) {
	ff := []model.Finding{
		finding(strings.Repeat("a", 50), model.SevHigh, "m", "p"),
		finding(strings.Repeat("b", 60), model.SevCritical, "m", "p"),
	}
	r := Analyze(ff)
	shortBucket := r.Buckets[0]
	if shortBucket.Count != 2 {
		t.Fatalf("expected 2 in short bucket, got %d", shortBucket.Count)
	}
	if shortBucket.AvgSeverity < 3.0 {
		t.Fatalf("expected avg severity >= 3.0, got %.1f", shortBucket.AvgSeverity)
	}
}

func TestValidatedPercentage(t *testing.T) {
	ff := []model.Finding{
		{Summary: strings.Repeat("x", 50), Severity: model.SevLow, Model: model.ModelRef{Model: "m"}, State: model.StateValidated},
		{Summary: strings.Repeat("y", 60), Severity: model.SevLow, Model: model.ModelRef{Model: "m"}, State: "theorized"},
	}
	r := Analyze(ff)
	if r.Buckets[0].ValidatedPct != 50.0 {
		t.Fatalf("expected 50%% validated, got %.1f%%", r.Buckets[0].ValidatedPct)
	}
}

func TestPerModelImpact(t *testing.T) {
	ff := []model.Finding{
		finding(strings.Repeat("a", 200), model.SevHigh, "claude", "anthropic"),
		finding(strings.Repeat("b", 100), model.SevMedium, "claude", "anthropic"),
		finding(strings.Repeat("c", 50), model.SevLow, "gpt-4", "openai"),
	}
	r := Analyze(ff)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.Models[0].TotalFindings != 2 {
		t.Fatalf("expected first model to have 2 findings, got %d", r.Models[0].TotalFindings)
	}
}

func TestPromptHash(t *testing.T) {
	ff := []model.Finding{
		{Summary: "test", Severity: model.SevLow, Model: model.ModelRef{Model: "m"}, PromptHash: "abc123"},
		{Summary: "test2", Severity: model.SevLow, Model: model.ModelRef{Model: "m"}},
	}
	r := Analyze(ff)
	if r.Summary.WithPromptHash != 1 {
		t.Fatalf("expected 1 with prompt hash, got %d", r.Summary.WithPromptHash)
	}
}

func TestAverageSummaryLength(t *testing.T) {
	ff := []model.Finding{
		finding(strings.Repeat("a", 100), model.SevLow, "m", "p"),
		finding(strings.Repeat("b", 200), model.SevLow, "m", "p"),
	}
	r := Analyze(ff)
	if r.Summary.AvgSummaryLen != 150 {
		t.Fatalf("expected avg 150, got %d", r.Summary.AvgSummaryLen)
	}
}

func TestMedianSummaryLength(t *testing.T) {
	ff := []model.Finding{
		finding(strings.Repeat("a", 100), model.SevLow, "m", "p"),
		finding(strings.Repeat("b", 200), model.SevLow, "m", "p"),
		finding(strings.Repeat("c", 300), model.SevLow, "m", "p"),
	}
	r := Analyze(ff)
	if r.Summary.MedianSummaryLen != 200 {
		t.Fatalf("expected median 200, got %d", r.Summary.MedianSummaryLen)
	}
}

func TestCorrelation(t *testing.T) {
	xs := []float64{1, 2, 3, 4, 5}
	ys := []float64{1, 2, 3, 4, 5}
	c := correlation(xs, ys)
	if c < 0.99 {
		t.Fatalf("expected correlation ~1.0, got %.2f", c)
	}
}

func TestCorrelationNegative(t *testing.T) {
	xs := []float64{1, 2, 3, 4, 5}
	ys := []float64{5, 4, 3, 2, 1}
	c := correlation(xs, ys)
	if c > -0.99 {
		t.Fatalf("expected correlation ~-1.0, got %.2f", c)
	}
}

func TestFormatText(t *testing.T) {
	ff := []model.Finding{
		finding("test finding summary", model.SevHigh, "gpt-4", "openai"),
	}
	r := Analyze(ff)
	text := FormatText(r)
	if !strings.Contains(text, "Prompt Length Impact") {
		t.Fatal("expected header")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Fatal("expected model name")
	}
}

func TestFormatJSON(t *testing.T) {
	ff := []model.Finding{
		finding("test", model.SevLow, "m", "p"),
	}
	r := Analyze(ff)
	j := FormatJSON(r)
	if !strings.Contains(j, "\"total_findings\"") {
		t.Fatal("expected JSON field")
	}
}

func TestEmptyModelSkipped(t *testing.T) {
	ff := []model.Finding{
		{Summary: "test", Severity: model.SevLow, Model: model.ModelRef{Model: ""}},
	}
	r := Analyze(ff)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 model entries, got %d", len(r.Models))
	}
}
