package modelfairness

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(id string, class model.Class, sev model.Severity, modelName, summary string) model.Finding {
	return model.Finding{
		ID:       id,
		Class:    class,
		Severity: sev,
		Model:    model.ModelRef{Model: modelName},
		Summary:  summary,
		State:    model.StateValidated,
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
	if r.MeanScore != 0 {
		t.Fatalf("expected 0 mean, got %.1f", r.MeanScore)
	}
}

func TestAnalyzeNoFindings(t *testing.T) {
	r := Analyze([]model.Finding{})
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
}

func TestAnalyzeSingleClassFair(t *testing.T) {
	f := []model.Finding{
		finding("f1", "sqli", model.SevHigh, "model-a", "sqli 1"),
		finding("f2", "sqli", model.SevHigh, "model-a", "sqli 2"),
	}
	r := Analyze(f)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	m := r.Models[0]
	if m.ClassGini != 0 {
		t.Errorf("single class should have 0 Gini, got %.3f", m.ClassGini)
	}
}

func TestAnalyzeEvenDistribution(t *testing.T) {
	f := []model.Finding{
		finding("f1", "sqli", model.SevHigh, "model-a", "sqli"),
		finding("f2", "xss-reflected", model.SevHigh, "model-a", "xss"),
		finding("f3", "idor", model.SevHigh, "model-a", "idor"),
		finding("f4", "ssrf", model.SevHigh, "model-a", "ssrf"),
	}
	r := Analyze(f)
	m := r.Models[0]
	if m.ClassGini > 0.1 {
		t.Errorf("even distribution should have low Gini, got %.3f", m.ClassGini)
	}
	if m.FairnessScore < 70 {
		t.Errorf("even distribution should have high fairness, got %.1f", m.FairnessScore)
	}
}

func TestAnalyzeSkewedDistribution(t *testing.T) {
	var f []model.Finding
	for i := 0; i < 20; i++ {
		f = append(f, finding("f", "sqli", model.SevHigh, "model-a", "sqli"))
	}
	f = append(f, finding("g", "xss-reflected", model.SevLow, "model-a", "xss"))
	r := Analyze(f)
	m := r.Models[0]
	if m.ClassConcentration < 0.9 {
		t.Errorf("skewed distribution should have high concentration, got %.3f", m.ClassConcentration)
	}
	if m.MostReportedClass != "sqli" {
		t.Errorf("expected sqli as most reported, got %s", m.MostReportedClass)
	}
}

func TestAnalyzeMultiModel(t *testing.T) {
	f := []model.Finding{
		finding("f1", "sqli", model.SevHigh, "model-a", "sqli"),
		finding("f2", "xss-reflected", model.SevHigh, "model-a", "xss"),
		finding("f3", "sqli", model.SevCritical, "model-b", "sqli"),
		finding("f4", "sqli", model.SevCritical, "model-b", "sqli2"),
		finding("f5", "sqli", model.SevCritical, "model-b", "sqli3"),
	}
	r := Analyze(f)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.Fairest == "" || r.LeastFair == "" {
		t.Error("fairest and least fair should be set")
	}
	if len(r.Pairwise) != 1 {
		t.Errorf("expected 1 pairwise comparison, got %d", len(r.Pairwise))
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	f := []model.Finding{
		{ID: "f1", Class: "sqli", Severity: model.SevLow, State: model.StateValidated},
	}
	r := Analyze(f)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown' model, got %q", r.Models[0].Model)
	}
}

func TestGiniCoefficient(t *testing.T) {
	equal := map[string]int{"a": 5, "b": 5, "c": 5}
	g := giniCoefficient(equal)
	if g > 0.05 {
		t.Errorf("equal distribution Gini should be near 0, got %.3f", g)
	}

	skewed := map[string]int{"a": 100, "b": 1}
	g2 := giniCoefficient(skewed)
	if g2 < 0.3 {
		t.Errorf("skewed distribution Gini should be high, got %.3f", g2)
	}

	single := map[string]int{"a": 10}
	g3 := giniCoefficient(single)
	if g3 != 0 {
		t.Errorf("single category Gini should be 0, got %.3f", g3)
	}
}

func TestPairwiseComparison(t *testing.T) {
	aFindings := []model.Finding{
		finding("f1", "sqli", model.SevHigh, "model-a", "sqli"),
		finding("f2", "xss-reflected", model.SevHigh, "model-a", "xss"),
	}
	bFindings := []model.Finding{
		finding("f3", "sqli", model.SevHigh, "model-b", "sqli"),
		finding("f4", "xss-reflected", model.SevHigh, "model-b", "xss"),
	}
	pc := comparePair(aFindings, bFindings, "model-a", "model-b")
	if pc.AgreementRate < 0.9 {
		t.Errorf("identical class distributions should have high agreement, got %.3f", pc.AgreementRate)
	}
	if pc.Divergence > 0.1 {
		t.Errorf("identical distributions should have low divergence, got %.3f", pc.Divergence)
	}
}

func TestScoreFloor(t *testing.T) {
	var f []model.Finding
	for i := 0; i < 100; i++ {
		f = append(f, finding("f", "sqli", model.SevCritical, "m", "sqli"))
	}
	f = append(f, finding("g", "xss-reflected", model.SevInfo, "m", "xss"))
	r := Analyze(f)
	if r.Models[0].FairnessScore < 0 {
		t.Errorf("score should not go below 0, got %.1f", r.Models[0].FairnessScore)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{100, "A"}, {90, "A"}, {89, "B"}, {80, "B"},
		{79, "C"}, {70, "C"}, {69, "D"}, {60, "D"},
		{59, "F"}, {0, "F"},
	}
	for _, tt := range tests {
		got := scoreToGrade(tt.score)
		if got != tt.want {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestFormatTextOutput(t *testing.T) {
	f := []model.Finding{
		finding("f1", "sqli", model.SevHigh, "model-a", "sqli"),
		finding("f2", "xss-reflected", model.SevMedium, "model-b", "xss"),
	}
	r := Analyze(f)
	text := FormatText(r)
	if !strings.Contains(text, "Model Fairness") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "model-a") {
		t.Error("missing model name")
	}
	if !strings.Contains(text, "Fairest") {
		t.Error("missing fairest line")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No fairness data") {
		t.Error("expected empty message")
	}
}

func TestFormatJSON(t *testing.T) {
	f := []model.Finding{
		finding("f1", "sqli", model.SevHigh, "model-a", "sqli"),
	}
	r := Analyze(f)
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(parsed.Models) != 1 {
		t.Errorf("expected 1 model, got %d", len(parsed.Models))
	}
}

func TestSeverityDist(t *testing.T) {
	f := []model.Finding{
		finding("f1", "sqli", model.SevHigh, "m", "a"),
		finding("f2", "sqli", model.SevLow, "m", "b"),
		finding("f3", "sqli", model.SevHigh, "m", "c"),
	}
	r := Analyze(f)
	if len(r.Models[0].SeverityDist) != 2 {
		t.Errorf("expected 2 severity levels, got %d", len(r.Models[0].SeverityDist))
	}
}
