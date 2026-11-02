package sevdist

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func f(mdl string, sev model.Severity) model.Finding {
	return model.Finding{
		Model:    model.ModelRef{Model: mdl},
		Severity: sev,
	}
}

func TestEmpty(t *testing.T) {
	r := Analyze(nil)
	if r.TotalFindings != 0 {
		t.Fatalf("want 0, got %d", r.TotalFindings)
	}
	if r.Recommendation != "No findings to analyze" {
		t.Fatalf("want no-findings recommendation, got %q", r.Recommendation)
	}
}

func TestSingleFinding(t *testing.T) {
	r := Analyze([]model.Finding{f("gpt-4", model.SevMedium)})
	if r.TotalFindings != 1 {
		t.Fatalf("want 1, got %d", r.TotalFindings)
	}
	if len(r.Overall) != 1 {
		t.Fatalf("want 1 severity bucket, got %d", len(r.Overall))
	}
	if r.Overall[0].Severity != "medium" {
		t.Fatalf("want medium, got %s", r.Overall[0].Severity)
	}
	if r.Overall[0].Pct != 100.0 {
		t.Fatalf("want 100%%, got %.1f%%", r.Overall[0].Pct)
	}
}

func TestMultipleSeverities(t *testing.T) {
	r := Analyze([]model.Finding{
		f("gpt-4", model.SevLow),
		f("gpt-4", model.SevMedium),
		f("gpt-4", model.SevHigh),
		f("gpt-4", model.SevCritical),
	})
	if len(r.Overall) != 4 {
		t.Fatalf("want 4 buckets, got %d", len(r.Overall))
	}
}

func TestMedianSev(t *testing.T) {
	r := Analyze([]model.Finding{
		f("gpt-4", model.SevLow),
		f("gpt-4", model.SevMedium),
		f("gpt-4", model.SevHigh),
	})
	if r.MedianSev != "medium" {
		t.Fatalf("want median medium, got %s", r.MedianSev)
	}
}

func TestP90Sev(t *testing.T) {
	ff := make([]model.Finding, 0, 10)
	for i := 0; i < 5; i++ {
		ff = append(ff, f("gpt-4", model.SevLow))
	}
	for i := 0; i < 5; i++ {
		ff = append(ff, f("gpt-4", model.SevCritical))
	}
	r := Analyze(ff)
	if r.P90Sev != "critical" {
		t.Fatalf("want P90 critical, got %s", r.P90Sev)
	}
}

func TestByModel(t *testing.T) {
	r := Analyze([]model.Finding{
		f("gpt-4", model.SevLow),
		f("claude-3", model.SevHigh),
	})
	if len(r.ByModel) != 2 {
		t.Fatalf("want 2 models, got %d", len(r.ByModel))
	}
}

func TestMeanRank(t *testing.T) {
	r := Analyze([]model.Finding{
		f("gpt-4", model.SevLow),
		f("gpt-4", model.SevHigh),
	})
	expected := 2.0 // (1+3)/2
	if r.MeanRank != expected {
		t.Fatalf("want mean rank %.2f, got %.2f", expected, r.MeanRank)
	}
}

func TestPercentile(t *testing.T) {
	ranks := []float64{1, 2, 3, 4, 5}
	p50 := percentile(ranks, 50)
	if p50 != 3.0 {
		t.Fatalf("want p50=3.0, got %.1f", p50)
	}
	p0 := percentile(ranks, 0)
	if p0 != 1.0 {
		t.Fatalf("want p0=1.0, got %.1f", p0)
	}
}

func TestPercentileEmpty(t *testing.T) {
	p := percentile(nil, 50)
	if p != 0 {
		t.Fatalf("want 0 for empty, got %.1f", p)
	}
}

func TestFormatText(t *testing.T) {
	r := Analyze([]model.Finding{f("gpt-4", model.SevMedium)})
	text := FormatText(r)
	if !strings.Contains(text, "Severity Distribution") {
		t.Fatal("should contain header")
	}
	if !strings.Contains(text, "medium") {
		t.Fatal("should contain severity name")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]model.Finding{f("gpt-4", model.SevMedium)})
	j := FormatJSON(r)
	if !strings.Contains(j, "median_severity") {
		t.Fatal("should contain median_severity field")
	}
}

func TestRecommendation(t *testing.T) {
	r := Analyze([]model.Finding{f("gpt-4", model.SevHigh)})
	if !strings.Contains(r.Recommendation, "high") {
		t.Fatalf("recommendation should mention median severity, got %q", r.Recommendation)
	}
}
