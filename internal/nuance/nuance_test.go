package nuance

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(mdl, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: mdl},
		Lab:     "lab1",
		Summary: summary,
	}
}

func TestCountPhrases(t *testing.T) {
	text := "however this is an edge case that depends on the context"
	if n := countPhrases(text, caveatPhrases); n < 1 {
		t.Fatalf("expected at least 1 caveat phrase match, got %d", n)
	}
	if n := countPhrases(text, edgeCasePhrases); n < 1 {
		t.Fatalf("expected at least 1 edge case phrase match, got %d", n)
	}
}

func TestCountPhrasesEmpty(t *testing.T) {
	if n := countPhrases("", caveatPhrases); n != 0 {
		t.Fatalf("expected 0, got %d", n)
	}
}

func TestCountPhrasesCaseInsensitive(t *testing.T) {
	text := "HOWEVER this is an EDGE CASE"
	if n := countPhrases(text, caveatPhrases); n < 1 {
		t.Fatalf("expected at least 1 match, got %d", n)
	}
}

func TestScoreResponseBasic(t *testing.T) {
	score, detail := scoreResponse("however there is an edge case to consider, it depends on the context", "")
	if score < 30 {
		t.Fatalf("expected score >= 30, got %.1f", score)
	}
	if detail.QualifierCount < 0 {
		t.Fatal("qualifier count should be non-negative")
	}
}

func TestScoreResponseAbsolutesPenalty(t *testing.T) {
	lowAbs, _ := scoreResponse("generally this tends to work in most cases", "")
	highAbs, _ := scoreResponse("always definitely guaranteed certainly undoubtedly invariably", "")
	if highAbs >= lowAbs {
		t.Fatalf("absolutes should reduce score: lowAbs=%.1f highAbs=%.1f", lowAbs, highAbs)
	}
}

func TestScoreResponseBounded(t *testing.T) {
	score, _ := scoreResponse(strings.Repeat("however edge case balanced view on one hand on the other hand ", 50), "")
	if score < 0 || score > 100 {
		t.Fatalf("score out of bounds: %.1f", score)
	}
}

func TestScoreResponseEmpty(t *testing.T) {
	score, _ := scoreResponse("", "")
	if score < 0 || score > 100 {
		t.Fatalf("score out of bounds: %.1f", score)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"},
		{85, "B"},
		{75, "C"},
		{65, "D"},
		{45, "F"},
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
		t.Fatalf("expected empty models, got %d", len(r.Models))
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "however there is an edge case here, it depends on context"),
		finding("gpt-4", "on the other hand this is a gray area with trade-offs"),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.BestModel != "gpt-4" {
		t.Fatalf("expected best model gpt-4, got %s", r.BestModel)
	}
	if r.Models[0].SampleCount != 2 {
		t.Fatalf("expected 2 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "however there is an edge case, on the other hand it depends"),
		finding("claude-3", "always definitely guaranteed impossible"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.Models[0].NuanceScore < r.Models[1].NuanceScore {
		t.Fatal("models should be sorted by score descending")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		{Lab: "lab1", Summary: "however edge case"},
	}
	r := Analyze(findings)
	if r.BestModel != "unknown" {
		t.Fatalf("expected unknown model, got %s", r.BestModel)
	}
}

func TestAnalyzeMeanScore(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "however edge case on the other hand"),
		finding("m2", "always definitely guaranteed"),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Fatalf("expected positive mean score, got %.1f", r.MeanScore)
	}
}

func TestAnalyzeLabCount(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab1", Summary: "however"},
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab2", Summary: "edge case"},
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Fatalf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestAnalyzeConcerns(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "always definitely guaranteed certainly undoubtedly absolutely"),
	}
	r := Analyze(findings)
	if r.Models[0].Concerns < 1 {
		t.Fatal("expected at least 1 concern for heavy absolute usage")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No nuance data") {
		t.Fatal("expected empty message")
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "however there is an edge case to consider"),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Response Nuance") {
		t.Fatal("expected header")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Fatal("expected model name")
	}
	if !strings.Contains(text, "edge_case_handling") {
		t.Fatal("expected detail field")
	}
}

func TestFormatJSONValid(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "however there is a caveat to note"),
	}
	r := Analyze(findings)
	out := FormatJSON(r)
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := data["models"]; !ok {
		t.Fatal("expected models key")
	}
}

func TestFormatJSONRoundtrip(t *testing.T) {
	r := Analyze(nil)
	out := FormatJSON(r)
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestEvidenceText(t *testing.T) {
	f := model.Finding{
		Summary: "test summary",
		Evidence: []model.Evidence{
			{Marker: "marker1"},
			{Marker: "marker2"},
		},
	}
	text := evidenceText(f)
	if !strings.Contains(text, "marker1") {
		t.Fatal("expected marker1")
	}
}

func TestEvidenceTextFallback(t *testing.T) {
	f := model.Finding{Summary: "fallback summary"}
	text := evidenceText(f)
	if text != "fallback summary" {
		t.Fatalf("expected fallback, got %s", text)
	}
}
