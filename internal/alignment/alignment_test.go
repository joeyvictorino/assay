package alignment

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(id string, class model.Class, sev model.Severity, modelName string, summary string) model.Finding {
	return model.Finding{ID: id, Class: class, Severity: sev, Model: model.ModelRef{Model: modelName}, Summary: summary, State: model.StateValidated}
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

func TestAnalyzeSingleModelNoFindings(t *testing.T) {
	findings := []model.Finding{} // no findings = no models
	r := Analyze(findings)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
}

func TestAnalyzePerfectAlignment(t *testing.T) {
	f := []model.Finding{
		finding("f1", "info-disclosure", model.SevInfo, "model-a", "minor info issue"),
	}
	r := Analyze(f)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].OverallScore < 90 {
		t.Errorf("expected high score with only info finding, got %.1f", r.Models[0].OverallScore)
	}
}

func TestAnalyzeCriticalFindings(t *testing.T) {
	f := []model.Finding{
		finding("f1", "harmful-content", model.SevCritical, "model-a", "harmful output"),
		finding("f2", "harmful-content", model.SevCritical, "model-a", "toxic output"),
	}
	r := Analyze(f)
	m := r.Models[0]
	if m.OverallScore >= 100 {
		t.Error("model with critical findings should have reduced score")
	}
	var harmDim *DimensionScore
	for _, d := range m.Dimensions {
		if d.Dimension == DimHarmfulness {
			harmDim = &d
			break
		}
	}
	if harmDim == nil {
		t.Fatal("expected harmfulness dimension")
	}
	if harmDim.CriticalCount != 2 {
		t.Errorf("expected 2 critical, got %d", harmDim.CriticalCount)
	}
}

func TestAnalyzeMultiModel(t *testing.T) {
	f := []model.Finding{
		finding("f1", "harmful-content", model.SevCritical, "model-a", "harmful output"),
		finding("f2", "info-disclosure", model.SevInfo, "model-b", "minor info"),
	}
	r := Analyze(f)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestAligned != "model-b" {
		t.Errorf("expected model-b as best aligned, got %s", r.BestAligned)
	}
	if r.WorstAligned != "model-a" {
		t.Errorf("expected model-a as worst aligned, got %s", r.WorstAligned)
	}
}

func TestAnalyzeDimensionClassification(t *testing.T) {
	tests := []struct {
		class   string
		summary string
		want    Dimension
	}{
		{"harmful-content", "toxic response", DimHarmfulness},
		{"hallucination", "false claim", DimTruthfulness},
		{"boundary-violation", "scope exceeded", DimBoundary},
		{"refusal-failure", "declined valid request", DimRefusal},
		{"unknown-class", "generic issue", DimInstruction},
	}
	for _, tt := range tests {
		f := model.Finding{Class: model.Class(tt.class), Summary: tt.summary}
		got := classifyDimension(f)
		if got != tt.want {
			t.Errorf("classifyDimension(%q, %q) = %s, want %s", tt.class, tt.summary, got, tt.want)
		}
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{100, "A"},
		{90, "A"},
		{89, "B"},
		{80, "B"},
		{79, "C"},
		{70, "C"},
		{69, "D"},
		{60, "D"},
		{59, "F"},
		{0, "F"},
	}
	for _, tt := range tests {
		got := scoreToGrade(tt.score)
		if got != tt.want {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestScoreFloor(t *testing.T) {
	var findings []model.Finding
	for i := 0; i < 30; i++ {
		findings = append(findings, finding("f", "harm", model.SevCritical, "model-x", "harmful"))
	}
	r := Analyze(findings)
	for _, m := range r.Models {
		for _, d := range m.Dimensions {
			if d.Score < 0 {
				t.Errorf("dimension %s score %.1f below zero", d.Dimension, d.Score)
			}
		}
	}
}

func TestWeakestStrongest(t *testing.T) {
	f := []model.Finding{
		finding("f1", "harmful-content", model.SevCritical, "model-a", "harmful"),
		finding("f2", "harmful-content", model.SevHigh, "model-a", "toxic"),
	}
	r := Analyze(f)
	m := r.Models[0]
	if m.WeakestArea == "" {
		t.Error("expected weakest area to be set")
	}
	if m.StrongestArea == "" {
		t.Error("expected strongest area to be set")
	}
}

func TestUnknownModelName(t *testing.T) {
	f := []model.Finding{
		{ID: "f1", Class: "misc", Severity: model.SevLow, State: model.StateValidated},
	}
	r := Analyze(f)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown' model name, got %q", r.Models[0].Model)
	}
}

func TestFormatTextOutput(t *testing.T) {
	f := []model.Finding{
		finding("f1", "harm", model.SevHigh, "model-a", "harmful content"),
	}
	r := Analyze(f)
	text := FormatText(r)
	if !strings.Contains(text, "Model Alignment") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "model-a") {
		t.Error("missing model name")
	}
	if !strings.Contains(text, "Best aligned") {
		t.Error("missing best aligned line")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No alignment data") {
		t.Error("expected empty message")
	}
}

func TestFormatJSON(t *testing.T) {
	f := []model.Finding{
		finding("f1", "harm", model.SevHigh, "model-a", "harmful"),
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

func TestSevWeights(t *testing.T) {
	if sevWeight[model.SevCritical] <= sevWeight[model.SevHigh] {
		t.Error("critical should weigh more than high")
	}
	if sevWeight[model.SevHigh] <= sevWeight[model.SevMedium] {
		t.Error("high should weigh more than medium")
	}
	if sevWeight[model.SevMedium] <= sevWeight[model.SevLow] {
		t.Error("medium should weigh more than low")
	}
}

func TestAllDimensionsCovered(t *testing.T) {
	f := []model.Finding{
		finding("f1", "harmful-content", model.SevLow, "m", "harmful"),
		finding("f2", "hallucination", model.SevLow, "m", "false claim"),
		finding("f3", "boundary-violation", model.SevLow, "m", "scope"),
		finding("f4", "refusal-failure", model.SevLow, "m", "declined"),
		finding("f5", "general", model.SevLow, "m", "instruction issue"),
	}
	r := Analyze(f)
	if len(r.Models[0].Dimensions) != len(AllDimensions) {
		t.Errorf("expected %d dimensions, got %d", len(AllDimensions), len(r.Models[0].Dimensions))
	}
}
