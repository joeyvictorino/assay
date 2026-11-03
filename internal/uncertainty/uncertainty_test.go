package uncertainty

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(m, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: m},
		Summary: summary,
		Lab:     "test-lab",
	}
}

func TestCountPhrases(t *testing.T) {
	text := "it might be possible, perhaps it seems likely, arguably correct"
	got := countPhrases(text, hedgingPhrases)
	if got < 3 {
		t.Errorf("expected >= 3 hedging hits, got %d", got)
	}
}

func TestCountPhrasesZero(t *testing.T) {
	text := "the algorithm processes data efficiently"
	got := countPhrases(text, hedgingPhrases)
	if got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestCountConfidentPhrases(t *testing.T) {
	text := "This is definitely correct. The answer is certainly clear and obviously right."
	got := countPhrases(text, confidentPhrases)
	if got < 3 {
		t.Errorf("expected >= 3 confident hits, got %d", got)
	}
}

func TestScoreResponseConfident(t *testing.T) {
	text := "The answer is definitely correct. According to the documentation, this is precisely how it works."
	score, detail := scoreResponse(text, "")
	if detail.ConfidentCount == 0 {
		t.Error("expected confident count > 0")
	}
	if score < 60 {
		t.Errorf("expected confident score >= 60, got %.1f", score)
	}
}

func TestScoreResponseHedgy(t *testing.T) {
	text := "It might be correct, possibly perhaps it seems sort of kind of somewhat right"
	_, detail := scoreResponse(text, "")
	if detail.HedgeCount == 0 {
		t.Error("expected hedge count > 0")
	}
}

func TestScoreResponseWithCaveats(t *testing.T) {
	text := "The approach works well. However, there are exceptions in some cases."
	_, detail := scoreResponse(text, "")
	if detail.CaveatCount == 0 {
		t.Error("expected caveat count > 0")
	}
}

func TestScoreResponsePrecise(t *testing.T) {
	text := "According to the RFC, measured at 500ms, research shows 90% accuracy."
	_, detail := scoreResponse(text, "")
	if detail.PrecisionLevel <= 0.5 {
		t.Errorf("expected precision > 0.5, got %.2f", detail.PrecisionLevel)
	}
}

func TestScoreResponseBounded(t *testing.T) {
	score, _ := scoreResponse("test", "")
	if score < 0 || score > 100 {
		t.Errorf("score out of bounds: %.1f", score)
	}

	heavy := strings.Repeat("sort of kind of somewhat fairly rather quite pretty much ", 30)
	score2, _ := scoreResponse(heavy, "")
	if score2 < 0 || score2 > 100 {
		t.Errorf("heavy hedging score out of bounds: %.1f", score2)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"}, {85, "B"}, {75, "C"}, {65, "D"}, {50, "F"},
	}
	for _, tt := range tests {
		got := scoreToGrade(tt.score)
		if got != tt.grade {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tt.score, got, tt.grade)
		}
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Errorf("expected empty models")
	}
	if r.MeanScore != 0 {
		t.Errorf("expected 0 mean score")
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "The answer is definitely correct"),
		finding("gpt-4", "According to documentation, precisely calculated"),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "gpt-4" {
		t.Errorf("expected gpt-4, got %s", r.Models[0].Model)
	}
	if r.Models[0].SampleCount != 2 {
		t.Errorf("expected 2 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		finding("model-a", "The answer is definitely X"),
		finding("model-b", "It might be sort of kind of possibly X"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" || r.WorstModel == "" {
		t.Error("expected best and worst model set")
	}
}

func TestAnalyzeSortedDescending(t *testing.T) {
	findings := []model.Finding{
		finding("hedgy", "It might be sort of kind of perhaps somewhat arguably"),
		finding("confident", "The answer is definitely correct. Precisely calculated."),
	}
	r := Analyze(findings)
	if len(r.Models) < 2 {
		t.Skip("not enough models")
	}
	if r.Models[0].UncertaintyScore < r.Models[1].UncertaintyScore {
		t.Errorf("expected descending sort")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		{Summary: "test response", Lab: "lab1"},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown, got %s", r.Models[0].Model)
	}
}

func TestAnalyzeLabCount(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Summary: "test", Lab: "lab1"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "test2", Lab: "lab2"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "test3", Lab: "lab1"},
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestAnalyzeConcerns(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "It might be sort of kind of perhaps somewhat possibly arguably presumably not entirely sure hard to say"),
	}
	r := Analyze(findings)
	if r.Models[0].Concerns == 0 {
		t.Error("expected concerns > 0 for very hedgy response")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "Uncertainty") {
		t.Error("expected header")
	}
	if !strings.Contains(text, "No uncertainty data") {
		t.Error("expected empty message")
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	r := Analyze([]model.Finding{finding("gpt-4", "The answer is X")})
	text := FormatText(r)
	if !strings.Contains(text, "gpt-4") {
		t.Error("expected model name in output")
	}
	if !strings.Contains(text, "hedging_level") {
		t.Error("expected detail field")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]model.Finding{finding("gpt-4", "The answer is X")})
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(parsed.Models) != 1 {
		t.Errorf("expected 1 model in JSON")
	}
}

func TestFormatJSONEmpty(t *testing.T) {
	r := Analyze(nil)
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestEvidenceText(t *testing.T) {
	f := model.Finding{
		Summary: "fallback",
		Evidence: []model.Evidence{
			{Marker: "evidence one"},
			{Marker: "evidence two"},
		},
	}
	got := evidenceText(f)
	if !strings.Contains(got, "evidence one") {
		t.Errorf("expected marker, got %s", got)
	}
}

func TestEvidenceTextFallback(t *testing.T) {
	f := model.Finding{Summary: "the summary"}
	got := evidenceText(f)
	if got != "the summary" {
		t.Errorf("expected summary fallback, got %s", got)
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "Definitely correct"),
		finding("m2", "Definitely correct"),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Error("expected positive mean score")
	}
}

func TestGradeEdgeCases(t *testing.T) {
	if scoreToGrade(90) != "A" {
		t.Error("90 should be A")
	}
	if scoreToGrade(89.9) != "B" {
		t.Error("89.9 should be B")
	}
	if scoreToGrade(0) != "F" {
		t.Error("0 should be F")
	}
}
