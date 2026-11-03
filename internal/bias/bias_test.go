package bias

import (
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func f(class model.Class, sev model.Severity) model.Finding {
	return model.Finding{Class: class, Severity: sev, Location: model.Location{Method: "GET", PathTemplate: "/api/test"}}
}

func TestSeverityClusteringAllSame(t *testing.T) {
	findings := []model.Finding{
		f(model.ClassSQLi, model.SevHigh),
		f(model.ClassXSSReflected, model.SevHigh),
		f(model.ClassIDOR, model.SevHigh),
	}
	r := AnalyzeBiasFromFindings(findings)
	bs := findBias(r, "severity-clustering")
	if bs.Score != 1.0 {
		t.Errorf("score = %f, want 1.0", bs.Score)
	}
	if !bs.Detected {
		t.Error("expected detected=true")
	}
}

func TestSeverityClusteringSpread(t *testing.T) {
	findings := []model.Finding{
		f(model.ClassSQLi, model.SevHigh),
		f(model.ClassXSSReflected, model.SevMedium),
		f(model.ClassIDOR, model.SevLow),
		f(model.ClassAuthMissing, model.SevCritical),
	}
	r := AnalyzeBiasFromFindings(findings)
	bs := findBias(r, "severity-clustering")
	if bs.Score != 0.0 {
		t.Errorf("score = %f, want 0.0", bs.Score)
	}
}

func TestPositionalBiasTooFew(t *testing.T) {
	findings := []model.Finding{
		f(model.ClassSQLi, model.SevHigh),
		f(model.ClassXSSReflected, model.SevLow),
	}
	r := AnalyzeBiasFromFindings(findings)
	bs := findBias(r, "positional-bias")
	if bs.Score != 0 {
		t.Errorf("score = %f, want 0", bs.Score)
	}
}

func TestPositionalBiasDetected(t *testing.T) {
	findings := []model.Finding{
		f(model.ClassSQLi, model.SevCritical),
		f(model.ClassXSSReflected, model.SevCritical),
		f(model.ClassIDOR, model.SevLow),
		f(model.ClassAuthMissing, model.SevLow),
		f(model.ClassSQLi, model.SevLow),
		f(model.ClassXSSStored, model.SevLow),
		f(model.ClassIDOR, model.SevCritical),
		f(model.ClassAuthMissing, model.SevCritical),
	}
	r := AnalyzeBiasFromFindings(findings)
	bs := findBias(r, "positional-bias")
	if bs.Score < 0.3 {
		t.Errorf("score = %f, expected some positional bias signal", bs.Score)
	}
}

func TestAnchoringBiasTooFew(t *testing.T) {
	findings := []model.Finding{
		f(model.ClassSQLi, model.SevHigh),
	}
	r := AnalyzeBiasFromFindings(findings)
	bs := findBias(r, "anchoring-bias")
	if bs.Score != 0 {
		t.Errorf("score = %f, want 0", bs.Score)
	}
}

func TestAnchoringBiasHighMatch(t *testing.T) {
	findings := []model.Finding{
		f(model.ClassSQLi, model.SevHigh),
		f(model.ClassXSSReflected, model.SevHigh),
		f(model.ClassIDOR, model.SevHigh),
		f(model.ClassAuthMissing, model.SevHigh),
		f(model.ClassSQLi, model.SevMedium),
	}
	r := AnalyzeBiasFromFindings(findings)
	bs := findBias(r, "anchoring-bias")
	if bs.Score < 0.4 {
		t.Errorf("score = %f, want >= 0.4 (75%% match first severity)", bs.Score)
	}
	if !bs.Detected {
		t.Error("expected detected=true")
	}
}

func TestCategoryBiasSingleCategory(t *testing.T) {
	findings := []model.Finding{
		f(model.ClassSQLi, model.SevHigh),
		f(model.ClassSQLi, model.SevMedium),
		f(model.ClassSQLi, model.SevLow),
	}
	r := AnalyzeBiasFromFindings(findings)
	bs := findBias(r, "category-bias")
	if bs.Score != 1.0 {
		t.Errorf("score = %f, want 1.0", bs.Score)
	}
}

func TestCategoryBiasSpread(t *testing.T) {
	findings := []model.Finding{
		f(model.ClassSQLi, model.SevHigh),
		f(model.ClassXSSReflected, model.SevMedium),
		f(model.ClassIDOR, model.SevLow),
		f(model.ClassAuthMissing, model.SevCritical),
	}
	r := AnalyzeBiasFromFindings(findings)
	bs := findBias(r, "category-bias")
	if bs.Score != 0.0 {
		t.Errorf("score = %f, want 0.0", bs.Score)
	}
}

func TestOverallRiskHigh(t *testing.T) {
	findings := []model.Finding{
		f(model.ClassSQLi, model.SevHigh),
		f(model.ClassSQLi, model.SevHigh),
		f(model.ClassSQLi, model.SevHigh),
	}
	r := AnalyzeBiasFromFindings(findings)
	if r.OverallRisk != "high" {
		t.Errorf("risk = %q, want high", r.OverallRisk)
	}
}

func TestEmptyFindings(t *testing.T) {
	r := AnalyzeBiasFromFindings(nil)
	if r.OverallRisk != "none" {
		t.Errorf("risk = %q, want none", r.OverallRisk)
	}
	if len(r.Biases) != 4 {
		t.Errorf("biases count = %d, want 4", len(r.Biases))
	}
}

func TestFormatTextOutput(t *testing.T) {
	findings := []model.Finding{
		f(model.ClassSQLi, model.SevHigh),
		f(model.ClassXSSReflected, model.SevMedium),
	}
	r := AnalyzeBiasFromFindings(findings)
	text := FormatText(r)
	if text == "" {
		t.Fatal("empty output")
	}
	if len(text) < 50 {
		t.Errorf("output too short: %d chars", len(text))
	}
}

func findBias(r BiasReport, name string) BiasScore {
	for _, b := range r.Biases {
		if b.Name == name {
			return b
		}
	}
	return BiasScore{}
}
