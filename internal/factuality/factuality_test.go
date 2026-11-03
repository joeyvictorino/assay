package factuality

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(id, modelName, summary string) model.Finding {
	return model.Finding{
		ID:       id,
		Class:    "other",
		Severity: model.SevLow,
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

func TestAnalyzeSingleModel(t *testing.T) {
	f := []model.Finding{
		finding("f1", "model-a", "The earth is round. Water boils at 100 degrees Celsius. The sun is a star."),
	}
	r := Analyze(f)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "model-a" {
		t.Errorf("expected model-a, got %s", r.Models[0].Model)
	}
	if len(r.Models[0].Dimensions) != 4 {
		t.Errorf("expected 4 dimensions, got %d", len(r.Models[0].Dimensions))
	}
}

func TestAnalyzeMultiModel(t *testing.T) {
	f := []model.Finding{
		finding("f1", "model-a", "The capital of France is Paris. According to recent studies, GDP is rising."),
		finding("f2", "model-b", "stuff"),
	}
	r := Analyze(f)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.MostFactual == "" || r.LeastFactual == "" {
		t.Error("expected most/least factual to be set")
	}
}

func TestDimensionNames(t *testing.T) {
	f := []model.Finding{
		finding("f1", "m", "The sky is blue. Water is wet."),
	}
	r := Analyze(f)
	want := map[string]bool{
		"claim_density":           true,
		"internal_consistency":    true,
		"citation_presence":       true,
		"hedging_appropriateness": true,
	}
	for _, d := range r.Models[0].Dimensions {
		if !want[d.Name] {
			t.Errorf("unexpected dimension %q", d.Name)
		}
		delete(want, d.Name)
	}
	for name := range want {
		t.Errorf("missing dimension %q", name)
	}
}

func TestScoresInRange(t *testing.T) {
	f := []model.Finding{
		finding("f1", "m", "The earth is approximately 4.5 billion years old. According to NASA, the moon is 384400 km away."),
		finding("f2", "m", "Gravity is a fundamental force. Light travels at 299792458 meters per second."),
	}
	r := Analyze(f)
	for _, m := range r.Models {
		if m.OverallScore < 0 || m.OverallScore > 100 {
			t.Errorf("overall score %.1f out of range", m.OverallScore)
		}
		for _, d := range m.Dimensions {
			if d.Score < 0 || d.Score > 100 {
				t.Errorf("dimension %s score %.1f out of range", d.Name, d.Score)
			}
		}
	}
}

func TestUnknownModel(t *testing.T) {
	f := []model.Finding{
		{ID: "f1", Class: "other", Severity: model.SevLow, State: model.StateValidated, Summary: "test text"},
	}
	r := Analyze(f)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown', got %q", r.Models[0].Model)
	}
}

func TestGradeAssignment(t *testing.T) {
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

func TestClaimDetection(t *testing.T) {
	f := []model.Finding{
		finding("f1", "m", "The earth is round. Water is wet. The sky has color. Dogs are mammals."),
	}
	r := Analyze(f)
	if r.Models[0].ClaimCount < 2 {
		t.Errorf("expected at least 2 claims, got %d", r.Models[0].ClaimCount)
	}
}

func TestCitationDetection(t *testing.T) {
	f := []model.Finding{
		finding("f1", "m", "According to recent research, the speed of light is constant [1]. Source: Physics Today."),
	}
	r := Analyze(f)
	var cp float64
	for _, d := range r.Models[0].Dimensions {
		if d.Name == "citation_presence" {
			cp = d.Score
		}
	}
	if cp < 50 {
		t.Errorf("text with citations should score high on citation_presence, got %.1f", cp)
	}
}

func TestNoCitations(t *testing.T) {
	f := []model.Finding{
		finding("f1", "m", "The sky is blue. Grass is green. Water flows downhill."),
	}
	r := Analyze(f)
	var cp float64
	for _, d := range r.Models[0].Dimensions {
		if d.Name == "citation_presence" {
			cp = d.Score
		}
	}
	if cp > 50 {
		t.Errorf("text without citations should score low on citation_presence, got %.1f", cp)
	}
}

func TestHedgingDetection(t *testing.T) {
	f := []model.Finding{
		finding("f1", "m", "This might be true. It could possibly work. The results are probably correct. Perhaps the data shows a trend."),
	}
	r := Analyze(f)
	var ha float64
	for _, d := range r.Models[0].Dimensions {
		if d.Name == "hedging_appropriateness" {
			ha = d.Score
		}
	}
	if ha < 20 {
		t.Errorf("hedged text should have some hedging score, got %.1f", ha)
	}
}

func TestSplitSentences(t *testing.T) {
	sents := splitSentences("Hello world. This is a test! Another one? Done.")
	if len(sents) != 4 {
		t.Errorf("expected 4 sentences, got %d: %v", len(sents), sents)
	}
}

func TestIsVerifiableClaim(t *testing.T) {
	if !isVerifiableClaim("The earth is round") {
		t.Error("expected verifiable claim")
	}
	if isVerifiableClaim("Hello") {
		t.Error("greeting should not be a claim")
	}
}

func TestFormatTextOutput(t *testing.T) {
	f := []model.Finding{
		finding("f1", "model-a", "The earth is round. According to NASA, it orbits the sun."),
	}
	r := Analyze(f)
	text := FormatText(r)
	if !strings.Contains(text, "Factual Accuracy") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "model-a") {
		t.Error("missing model name")
	}
	if !strings.Contains(text, "claim_density") {
		t.Error("missing dimension name")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No factuality data") {
		t.Error("expected empty message")
	}
}

func TestFormatJSON(t *testing.T) {
	f := []model.Finding{
		finding("f1", "model-a", "The earth is a planet."),
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

func TestContradictionDetection(t *testing.T) {
	f := []model.Finding{
		finding("f1", "m", "The sky is blue and clear. The sky is not blue today. The sky has no color at all."),
	}
	r := Analyze(f)
	var ic float64
	for _, d := range r.Models[0].Dimensions {
		if d.Name == "internal_consistency" {
			ic = d.Score
		}
	}
	if ic >= 100 {
		t.Errorf("contradictory text should not score perfect consistency, got %.1f", ic)
	}
}
