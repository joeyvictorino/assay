package sevdist

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(sev model.Severity, mdl string) model.Finding {
	return model.Finding{
		ID:       "f1",
		Severity: sev,
		Class:    model.ClassSQLi,
		Model:    model.ModelRef{Model: mdl},
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
	r := Analyze([]model.Finding{finding(model.SevHigh, "gpt-4")})
	if r.TotalFindings != 1 {
		t.Fatalf("want 1, got %d", r.TotalFindings)
	}
	if r.MedianSeverity != "high" {
		t.Fatalf("want median high, got %s", r.MedianSeverity)
	}
	if r.MeanRank != 3.0 {
		t.Fatalf("want mean 3.0, got %.2f", r.MeanRank)
	}
}

func TestDistribution(t *testing.T) {
	ff := []model.Finding{
		finding(model.SevLow, "gpt-4"),
		finding(model.SevLow, "gpt-4"),
		finding(model.SevHigh, "gpt-4"),
		finding(model.SevCritical, "gpt-4"),
	}
	r := Analyze(ff)
	if len(r.Overall) != 3 {
		t.Fatalf("want 3 severity buckets, got %d", len(r.Overall))
	}
	for _, sb := range r.Overall {
		if sb.Severity == "low" && sb.Count != 2 {
			t.Fatalf("want 2 low, got %d", sb.Count)
		}
	}
}

func TestByModel(t *testing.T) {
	ff := []model.Finding{
		finding(model.SevCritical, "gpt-4"),
		finding(model.SevLow, "claude-3"),
	}
	r := Analyze(ff)
	if len(r.ByModel) != 2 {
		t.Fatalf("want 2 models, got %d", len(r.ByModel))
	}
	if r.ByModel[0].Mean < r.ByModel[1].Mean {
		t.Fatal("models should be sorted by mean rank descending")
	}
}

func TestMedianSeverity(t *testing.T) {
	ff := []model.Finding{
		finding(model.SevLow, "gpt-4"),
		finding(model.SevMedium, "gpt-4"),
		finding(model.SevHigh, "gpt-4"),
	}
	r := Analyze(ff)
	if r.MedianSeverity != "medium" {
		t.Fatalf("want median medium, got %s", r.MedianSeverity)
	}
}

func TestHighSeverityRecommendation(t *testing.T) {
	ff := []model.Finding{
		finding(model.SevCritical, "gpt-4"),
		finding(model.SevCritical, "gpt-4"),
		finding(model.SevHigh, "gpt-4"),
	}
	r := Analyze(ff)
	if !strings.Contains(r.Recommendation, "prioritize") {
		t.Fatalf("should recommend prioritization, got %q", r.Recommendation)
	}
}

func TestUnknownModel(t *testing.T) {
	r := Analyze([]model.Finding{{Severity: model.SevLow}})
	if len(r.ByModel) != 1 {
		t.Fatalf("want 1 model, got %d", len(r.ByModel))
	}
	if r.ByModel[0].Model != "unknown" {
		t.Fatalf("want 'unknown' model, got %q", r.ByModel[0].Model)
	}
}

func TestFormatText(t *testing.T) {
	r := Analyze([]model.Finding{finding(model.SevHigh, "gpt-4")})
	text := FormatText(r)
	if !strings.Contains(text, "Severity Distribution") {
		t.Fatal("should contain header")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]model.Finding{finding(model.SevHigh, "gpt-4")})
	j := FormatJSON(r)
	if !strings.Contains(j, "median_severity") {
		t.Fatal("should contain median_severity field")
	}
}
