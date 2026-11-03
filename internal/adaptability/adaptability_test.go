package adaptability

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
	text := "the response was formal and detailed with bullet list and table"
	if n := countPhrases(text, tonePhrases); n < 1 {
		t.Fatalf("expected at least 1 tone phrase match, got %d", n)
	}
	if n := countPhrases(text, formatSwitchPhrases); n < 1 {
		t.Fatalf("expected at least 1 format phrase match, got %d", n)
	}
}

func TestCountPhrasesEmpty(t *testing.T) {
	if n := countPhrases("", tonePhrases); n != 0 {
		t.Fatalf("expected 0, got %d", n)
	}
}

func TestCountPhrasesCaseInsensitive(t *testing.T) {
	text := "FORMAL TECHNICAL STEP-BY-STEP response"
	if n := countPhrases(text, tonePhrases); n < 1 {
		t.Fatalf("expected at least 1 match, got %d", n)
	}
}

func TestScoreResponseBasic(t *testing.T) {
	score, detail := scoreResponse("formal detailed response with bullet list for beginner audience", "")
	if score < 30 {
		t.Fatalf("expected score >= 30, got %.1f", score)
	}
	if detail.AdaptationSignalCount < 0 {
		t.Fatal("adaptation signal count should be non-negative")
	}
}

func TestScoreResponseRigidPenalty(t *testing.T) {
	adaptable, _ := scoreResponse("formal technical step-by-step detailed response for developer audience", "")
	rigid, _ := scoreResponse("i can only respond i cannot adapt i am unable to change my standard format", "")
	if rigid >= adaptable {
		t.Fatalf("rigidity should reduce score: adaptable=%.1f rigid=%.1f", adaptable, rigid)
	}
}

func TestScoreResponseBounded(t *testing.T) {
	score, _ := scoreResponse(strings.Repeat("formal detailed bullet list beginner developer expert ", 50), "")
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
		finding("gpt-4", "formal detailed response with bullet list tailored for developer audience"),
		finding("gpt-4", "casual simplified explanation with table for beginner audience"),
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
		finding("gpt-4", "formal detailed step-by-step for developer expert audience as you mentioned"),
		finding("claude-3", "i can only respond i cannot adapt regardless of the request"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.Models[0].AdaptabilityScore < r.Models[1].AdaptabilityScore {
		t.Fatal("models should be sorted by score descending")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		{Lab: "lab1", Summary: "formal detailed response"},
	}
	r := Analyze(findings)
	if r.BestModel != "unknown" {
		t.Fatalf("expected unknown model, got %s", r.BestModel)
	}
}

func TestAnalyzeMeanScore(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "formal detailed response for developer audience"),
		finding("m2", "i can only i cannot regardless of"),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Fatalf("expected positive mean score, got %.1f", r.MeanScore)
	}
}

func TestAnalyzeLabCount(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab1", Summary: "formal"},
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab2", Summary: "detailed"},
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Fatalf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestAnalyzeConcerns(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "i can only respond i cannot adapt i am unable to change outside my scope"),
	}
	r := Analyze(findings)
	if r.Models[0].Concerns < 1 {
		t.Fatal("expected at least 1 concern for rigid responses")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No adaptability data") {
		t.Fatal("expected empty message")
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "formal detailed response with bullet points for developer"),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Response Adaptability") {
		t.Fatal("expected header")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Fatal("expected model name")
	}
	if !strings.Contains(text, "tone_flexibility") {
		t.Fatal("expected detail field")
	}
}

func TestFormatJSONValid(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "formal detailed response with constraint requirement"),
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

func TestTonePhrasesDetected(t *testing.T) {
	_, detail := scoreResponse("professional technical conversational simplified", "")
	if detail.ToneFlexibility <= 0.2 {
		t.Fatalf("expected tone flexibility above baseline, got %.2f", detail.ToneFlexibility)
	}
}

func TestFormatPhrasesDetected(t *testing.T) {
	_, detail := scoreResponse("provided as a bullet list with code block and table", "")
	if detail.FormatAdaptation <= 0.2 {
		t.Fatalf("expected format adaptation above baseline, got %.2f", detail.FormatAdaptation)
	}
}

func TestAudiencePhrasesDetected(t *testing.T) {
	_, detail := scoreResponse("explanation tailored for beginner non-technical end user", "")
	if detail.AudienceAwareness <= 0.2 {
		t.Fatalf("expected audience awareness above baseline, got %.2f", detail.AudienceAwareness)
	}
}
