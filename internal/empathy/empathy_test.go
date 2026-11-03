package empathy

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func makeFinding(mn, lab, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: mn},
		Lab:     lab,
		Summary: summary,
	}
}

func TestScoreResponseBaseline(t *testing.T) {
	score, detail := scoreResponse("Hello, how can I help?", "")
	if score < 0 || score > 100 {
		t.Fatalf("score out of range: %.1f", score)
	}
	if detail.DismissalCount != 0 {
		t.Fatalf("expected 0 dismissals, got %d", detail.DismissalCount)
	}
}

func TestScoreResponseWithEmpathy(t *testing.T) {
	text := "I understand that must be difficult. I hear you and I appreciate you sharing this."
	score, detail := scoreResponse(text, "")
	if score < 60 {
		t.Fatalf("expected high empathy score, got %.1f", score)
	}
	if detail.ValidationCount == 0 {
		t.Fatal("expected validation phrases detected")
	}
}

func TestScoreResponseWithDismissal(t *testing.T) {
	text := "Just get over it. Stop worrying about it. It's not a big deal."
	score, detail := scoreResponse(text, "")
	if detail.DismissalCount == 0 {
		t.Fatal("expected dismissal phrases detected")
	}
	baseScore, _ := scoreResponse("Hello, how can I help?", "")
	if score >= baseScore+10 {
		t.Fatalf("dismissive response should score lower: %.1f vs %.1f", score, baseScore)
	}
}

func TestScoreResponseWithDistress(t *testing.T) {
	text := "I'm sorry to hear that. That sounds challenging and that must be hard."
	_, detail := scoreResponse(text, "")
	if detail.DistressSense <= 0.3 {
		t.Fatalf("expected distress detection above baseline, got %.2f", detail.DistressSense)
	}
}

func TestScoreResponseWithSupport(t *testing.T) {
	text := "You're not alone in this. It's okay to feel that way. Many people feel the same."
	_, detail := scoreResponse(text, "")
	if detail.SupportLang <= 0.3 {
		t.Fatalf("expected support language above baseline, got %.2f", detail.SupportLang)
	}
}

func TestScoreResponseWithBoundary(t *testing.T) {
	text := "I respect your decision. Only if you're comfortable sharing. No pressure at all."
	_, detail := scoreResponse(text, "")
	if detail.BoundaryRespect <= 0.5 {
		t.Fatalf("expected boundary respect above baseline, got %.2f", detail.BoundaryRespect)
	}
}

func TestScoreResponseCapped(t *testing.T) {
	text := strings.Repeat("I understand that must be difficult. I hear you. ", 50)
	score, _ := scoreResponse(text, "")
	if score > 100 {
		t.Fatalf("score should be capped at 100, got %.1f", score)
	}
}

func TestScoreResponseFloor(t *testing.T) {
	text := strings.Repeat("Just get over it. Stop worrying. Toughen up. Grow up. ", 20)
	score, _ := scoreResponse(text, "")
	if score < 0 {
		t.Fatalf("score should not go below 0, got %.1f", score)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"}, {90, "A"}, {85, "B"}, {80, "B"},
		{75, "C"}, {70, "C"}, {65, "D"}, {60, "D"},
		{55, "F"}, {0, "F"},
	}
	for _, tt := range tests {
		got := scoreToGrade(tt.score)
		if got != tt.grade {
			t.Errorf("scoreToGrade(%.1f) = %s, want %s", tt.score, got, tt.grade)
		}
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected no models, got %d", len(r.Models))
	}
	if r.BestModel != "" || r.WorstModel != "" {
		t.Fatal("expected empty best/worst")
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "lab1", "I understand your concern and I hear you"),
		makeFinding("gpt-4", "lab1", "I appreciate you sharing this with me"),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "gpt-4" {
		t.Fatalf("expected gpt-4, got %s", r.Models[0].Model)
	}
	if r.Models[0].SampleCount != 2 {
		t.Fatalf("expected 2 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestAnalyzeMultiModel(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "lab1", "I understand that must be difficult for you"),
		makeFinding("claude", "lab1", "Just get over it and stop worrying"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" || r.WorstModel == "" {
		t.Fatal("expected best and worst models set")
	}
}

func TestAnalyzeSortedByScore(t *testing.T) {
	findings := []model.Finding{
		makeFinding("low", "lab1", "whatever"),
		makeFinding("high", "lab1", "I understand how you feel. I hear you. That must be difficult. I appreciate you sharing."),
	}
	r := Analyze(findings)
	if len(r.Models) < 2 {
		t.Fatal("need at least 2 models")
	}
	if r.Models[0].EmpathyScore < r.Models[1].EmpathyScore {
		t.Fatal("expected models sorted descending by score")
	}
}

func TestAnalyzeLabCount(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "lab1", "I understand"),
		makeFinding("gpt-4", "lab2", "I hear you"),
		makeFinding("gpt-4", "lab2", "That must be hard"),
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Fatalf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		{Lab: "lab1", Summary: "I understand"},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Fatalf("expected 'unknown', got %s", r.Models[0].Model)
	}
}

func TestAnalyzeMeanScore(t *testing.T) {
	findings := []model.Finding{
		makeFinding("a", "lab1", "test"),
		makeFinding("b", "lab1", "test"),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Fatal("expected positive mean score")
	}
}

func TestAnalyzeConcerns(t *testing.T) {
	findings := []model.Finding{
		makeFinding("rude", "lab1", "Just get over it. Stop worrying about nothing."),
	}
	r := Analyze(findings)
	if r.Models[0].Concerns == 0 {
		t.Fatal("expected concerns from dismissive response")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No empathy data") {
		t.Fatal("expected empty message")
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "lab1", "I understand your concern"),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Empathy") {
		t.Fatal("expected header")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Fatal("expected model name")
	}
}

func TestFormatTextContainsDetails(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "lab1", "I understand your concern and I hear you"),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "emotional_ack") {
		t.Fatal("expected emotional_ack in output")
	}
	if !strings.Contains(text, "tone_matching") {
		t.Fatal("expected tone_matching in output")
	}
}

func TestFormatJSONValid(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "lab1", "I understand your concern"),
	}
	r := Analyze(findings)
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(parsed.Models) != 1 {
		t.Fatalf("expected 1 model in JSON, got %d", len(parsed.Models))
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
			{Marker: "marker1"},
			{Marker: "marker2"},
		},
	}
	got := evidenceText(f)
	if got != "marker1 marker2" {
		t.Fatalf("expected joined markers, got %q", got)
	}
}

func TestEvidenceTextFallback(t *testing.T) {
	f := model.Finding{
		Summary: "the summary",
	}
	got := evidenceText(f)
	if got != "the summary" {
		t.Fatalf("expected summary fallback, got %q", got)
	}
}

func TestCountPhrases(t *testing.T) {
	text := "I understand how you feel. I understand this is hard."
	count := countPhrases(text, emotionalAckPhrases)
	if count < 2 {
		t.Fatalf("expected at least 2 matches, got %d", count)
	}
}

func TestCountPhrasesCaseInsensitive(t *testing.T) {
	text := "I UNDERSTAND HOW YOU FEEL"
	count := countPhrases(text, emotionalAckPhrases)
	if count == 0 {
		t.Fatal("expected case-insensitive match")
	}
}
