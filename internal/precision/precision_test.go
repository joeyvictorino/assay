package precision

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
	text := "according to the specification, as documented in the technical reference"
	if n := countPhrases(text, citationPhrases); n < 2 {
		t.Fatalf("expected at least 2 citation phrase matches, got %d", n)
	}
}

func TestCountPhrasesEmpty(t *testing.T) {
	if n := countPhrases("", citationPhrases); n != 0 {
		t.Fatalf("expected 0, got %d", n)
	}
}

func TestCountPhrasesCaseInsensitive(t *testing.T) {
	text := "ACCORDING TO the SPECIFICATION"
	if n := countPhrases(text, citationPhrases); n < 1 {
		t.Fatalf("expected at least 1 match, got %d", n)
	}
}

func TestCountNumbers(t *testing.T) {
	text := "the value is 42 and the rate is 99.5 percent"
	n := countNumbers(text)
	if n < 2 {
		t.Fatalf("expected at least 2 numbers, got %d", n)
	}
}

func TestCountNumbersEmpty(t *testing.T) {
	if n := countNumbers(""); n != 0 {
		t.Fatalf("expected 0, got %d", n)
	}
}

func TestScoreResponseBasic(t *testing.T) {
	score, detail := scoreResponse("according to the specification, specifically the algorithm implementation uses this protocol", "")
	if score < 30 {
		t.Fatalf("expected score >= 30, got %.1f", score)
	}
	if detail.CitationCount < 1 {
		t.Fatal("expected at least 1 citation count")
	}
}

func TestScoreResponseVaguePenalty(t *testing.T) {
	precise, _ := scoreResponse("according to the specification, specifically the algorithm requires this implementation", "")
	vague, _ := scoreResponse("some people say it is believed many experts sources say it is known generally accepted", "")
	if vague >= precise {
		t.Fatalf("vague should reduce score: precise=%.1f vague=%.1f", precise, vague)
	}
}

func TestScoreResponseBounded(t *testing.T) {
	score, _ := scoreResponse(strings.Repeat("according to specifically algorithm implementation measured at 42 ", 50), "")
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
		finding("gpt-4", "according to the specification, the algorithm implementation is correct"),
		finding("gpt-4", "specifically the protocol configuration uses this parameter"),
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
		finding("gpt-4", "according to the specification specifically the algorithm implementation"),
		finding("claude-3", "some people say it is believed many experts sources say"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.Models[0].PrecisionScore < r.Models[1].PrecisionScore {
		t.Fatal("models should be sorted by score descending")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		{Lab: "lab1", Summary: "according to specifically"},
	}
	r := Analyze(findings)
	if r.BestModel != "unknown" {
		t.Fatalf("expected unknown model, got %s", r.BestModel)
	}
}

func TestAnalyzeMeanScore(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "according to the specification algorithm"),
		finding("m2", "some people say it is believed"),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Fatalf("expected positive mean score, got %.1f", r.MeanScore)
	}
}

func TestAnalyzeLabCount(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab1", Summary: "according to"},
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab2", Summary: "specifically"},
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Fatalf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestAnalyzeConcerns(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "some people say it is believed many experts sources say it is known generally accepted widely reported commonly thought"),
	}
	r := Analyze(findings)
	if r.Models[0].Concerns < 1 {
		t.Fatal("expected at least 1 concern for heavy vague usage")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No precision data") {
		t.Fatal("expected empty message")
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "according to the specification the algorithm works"),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Precision") {
		t.Fatal("expected header")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Fatal("expected model name")
	}
	if !strings.Contains(text, "factual_citation") {
		t.Fatal("expected detail field")
	}
}

func TestFormatJSONValid(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "according to specifically algorithm"),
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
