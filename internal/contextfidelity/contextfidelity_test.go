package contextfidelity

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
	text := "according to the context, this aligns with the document states"
	if n := countPhrases(text, attributionPhrases); n < 1 {
		t.Fatalf("expected at least 1 attribution phrase match, got %d", n)
	}
	if n := countPhrases(text, consistencyPhrases); n < 1 {
		t.Fatalf("expected at least 1 consistency phrase match, got %d", n)
	}
}

func TestCountPhrasesEmpty(t *testing.T) {
	if n := countPhrases("", attributionPhrases); n != 0 {
		t.Fatalf("expected 0, got %d", n)
	}
}

func TestCountPhrasesCaseInsensitive(t *testing.T) {
	text := "ACCORDING TO the SOURCE, this ALIGNS WITH expectations"
	if n := countPhrases(text, attributionPhrases); n < 1 {
		t.Fatalf("expected at least 1 match, got %d", n)
	}
}

func TestScoreResponseBasic(t *testing.T) {
	score, detail := scoreResponse("according to the context this aligns with the specification as documented and consistent with expectations", "")
	if score < 30 {
		t.Fatalf("expected score >= 30, got %.1f", score)
	}
	if detail.FaithfulCount < 0 {
		t.Fatal("faithful count should be non-negative")
	}
}

func TestScoreResponseDriftPenalty(t *testing.T) {
	faithful, _ := scoreResponse("according to the context this is consistent with the source as documented", "")
	drifty, _ := scoreResponse("in general broadly speaking one could argue tangentially on a related note while not directly related as an aside beyond the scope hypothetically in other contexts", "")
	if drifty >= faithful {
		t.Fatalf("drift should reduce score: faithful=%.1f drifty=%.1f", faithful, drifty)
	}
}

func TestScoreResponseFabricatedPenalty(t *testing.T) {
	faithful, _ := scoreResponse("according to the context this aligns with the reference as documented per the standard", "")
	fabricated, _ := scoreResponse("it is well known that everyone knows studies have shown experts agree it has been proven statistics show common knowledge widely accepted", "")
	if fabricated >= faithful {
		t.Fatalf("fabricated should reduce score: faithful=%.1f fabricated=%.1f", faithful, fabricated)
	}
}

func TestScoreResponseBounded(t *testing.T) {
	score, _ := scoreResponse(strings.Repeat("according to the context consistent with aligns with as documented specifically see section ", 50), "")
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
		finding("gpt-4", "according to the source this aligns with expectations"),
		finding("gpt-4", "consistent with the documentation as described in the reference"),
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
		finding("gpt-4", "according to the context this is consistent with the source as documented per the standard"),
		finding("claude-3", "in general broadly speaking one could argue it is well known that everyone knows studies have shown"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.Models[0].FidelityScore < r.Models[1].FidelityScore {
		t.Fatal("models should be sorted by score descending")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		{Lab: "lab1", Summary: "according to the source"},
	}
	r := Analyze(findings)
	if r.BestModel != "unknown" {
		t.Fatalf("expected unknown model, got %s", r.BestModel)
	}
}

func TestAnalyzeMeanScore(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "according to the context consistent with"),
		finding("m2", "in general broadly speaking"),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Fatalf("expected positive mean score, got %.1f", r.MeanScore)
	}
}

func TestAnalyzeLabCount(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab1", Summary: "according to"},
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab2", Summary: "consistent with"},
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Fatalf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestAnalyzeConcerns(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "it is well known that everyone knows studies have shown research indicates experts agree it has been proven statistics show data suggests according to experts the consensus is widely accepted common knowledge"),
	}
	r := Analyze(findings)
	if r.Models[0].Concerns < 1 {
		t.Fatal("expected at least 1 concern for heavy fabrication usage")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No fidelity data") {
		t.Fatal("expected empty message")
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "according to the source this is consistent with the context"),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Fidelity") {
		t.Fatal("expected header")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Fatal("expected model name")
	}
	if !strings.Contains(text, "source_attribution") {
		t.Fatal("expected detail field")
	}
}

func TestFormatJSONValid(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "according to the context consistent with the source"),
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
