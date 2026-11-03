package pedagogical

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
	text := "for example this means the key idea to illustrate"
	if n := countPhrases(text, clarityPhrases); n < 1 {
		t.Fatalf("expected at least 1 clarity phrase match, got %d", n)
	}
	if n := countPhrases(text, examplePhrases); n < 1 {
		t.Fatalf("expected at least 1 example phrase match, got %d", n)
	}
}

func TestCountPhrasesEmpty(t *testing.T) {
	if n := countPhrases("", clarityPhrases); n != 0 {
		t.Fatalf("expected 0, got %d", n)
	}
}

func TestCountPhrasesCaseInsensitive(t *testing.T) {
	text := "FOR EXAMPLE THIS MEANS THE KEY IDEA"
	if n := countPhrases(text, clarityPhrases); n < 1 {
		t.Fatalf("expected at least 1 match, got %d", n)
	}
}

func TestScoreResponseBasic(t *testing.T) {
	score, detail := scoreResponse("for example this means the key idea, first then next to summarize", "")
	if score < 30 {
		t.Fatalf("expected score >= 30, got %.1f", score)
	}
	if detail.ExplanationCount < 1 {
		t.Fatal("expected at least 1 explanation count")
	}
}

func TestScoreResponseOversimplifyPenalty(t *testing.T) {
	good, _ := scoreResponse("for example this means the key idea to illustrate first then", "")
	bad, _ := scoreResponse("it is just trivially any idiot dead simple nothing complicated brainless", "")
	if bad >= good {
		t.Fatalf("oversimplification should reduce score: good=%.1f bad=%.1f", good, bad)
	}
}

func TestScoreResponseBounded(t *testing.T) {
	score, _ := scoreResponse(strings.Repeat("for example this means first then to summarize think of it like ", 50), "")
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
		finding("gpt-4", "for example this means the key idea to illustrate"),
		finding("gpt-4", "first then next to summarize the key takeaway"),
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
		finding("gpt-4", "for example this means to illustrate first then to summarize"),
		finding("claude-3", "it is just trivially any idiot dead simple nothing complicated"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.Models[0].PedagogicalScore < r.Models[1].PedagogicalScore {
		t.Fatal("models should be sorted by score descending")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		{Lab: "lab1", Summary: "for example this means"},
	}
	r := Analyze(findings)
	if r.BestModel != "unknown" {
		t.Fatalf("expected unknown model, got %s", r.BestModel)
	}
}

func TestAnalyzeMeanScore(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "for example this means to illustrate"),
		finding("m2", "it is just trivially"),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Fatalf("expected positive mean score, got %.1f", r.MeanScore)
	}
}

func TestAnalyzeLabCount(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab1", Summary: "for example"},
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab2", Summary: "this means"},
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Fatalf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestAnalyzeConcerns(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "it is just trivially any idiot dead simple nothing complicated brainless child could do it"),
	}
	r := Analyze(findings)
	if r.Models[0].Concerns < 1 {
		t.Fatal("expected at least 1 concern for heavy oversimplification")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No pedagogical data") {
		t.Fatal("expected empty message")
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "for example this means the key idea"),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Pedagogical") {
		t.Fatal("expected header")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Fatal("expected model name")
	}
	if !strings.Contains(text, "concept_clarity") {
		t.Fatal("expected detail field")
	}
}

func TestFormatJSONValid(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "for example this means to illustrate"),
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
