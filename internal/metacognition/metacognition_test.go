package metacognition

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: "test-model"},
		Lab:     "lab1",
		Summary: summary,
	}
}

func TestCountPhrases(t *testing.T) {
	text := "I should note that my reasoning here is based on limited data"
	if c := countPhrases(text, selfAwarePhrases); c < 1 {
		t.Errorf("expected at least 1 self-aware phrase, got %d", c)
	}
}

func TestCountPhrasesEmpty(t *testing.T) {
	if c := countPhrases("", selfAwarePhrases); c != 0 {
		t.Errorf("expected 0 for empty text, got %d", c)
	}
}

func TestCountPhrasesNoMatch(t *testing.T) {
	if c := countPhrases("hello world", selfAwarePhrases); c != 0 {
		t.Errorf("expected 0 for non-matching text, got %d", c)
	}
}

func TestScoreResponseBasic(t *testing.T) {
	score, detail := scoreResponse("basic response without metacognitive markers", "")
	if score < 0 || score > 100 {
		t.Errorf("score out of range: %.1f", score)
	}
	if detail.ReflectiveCount < 0 {
		t.Error("reflective count should not be negative")
	}
}

func TestScoreResponseReflective(t *testing.T) {
	text := "Let me think through this. My reasoning is that we should consider the trade-offs. I should note that I'm not certain about this, and I have limited knowledge in this area."
	score, detail := scoreResponse(text, "")
	if detail.ReflectiveCount < 3 {
		t.Errorf("expected at least 3 reflective phrases, got %d", detail.ReflectiveCount)
	}
	if score < 40 {
		t.Errorf("reflective response should score above 40, got %.1f", score)
	}
}

func TestScoreResponseOverconfident(t *testing.T) {
	text := "Without a doubt this is absolutely the only correct answer. I guarantee it is undeniable."
	_, detail := scoreResponse(text, "")
	if detail.OverconfidentCount < 3 {
		t.Errorf("expected at least 3 overconfident phrases, got %d", detail.OverconfidentCount)
	}
}

func TestScoreResponseWithEvidence(t *testing.T) {
	score1, _ := scoreResponse("response", "")
	score2, _ := scoreResponse("response", "I should note my reasoning is based on step by step analysis")
	if score2 <= score1 {
		t.Errorf("evidence with metacognitive markers should increase score: %.1f vs %.1f", score2, score1)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{95, "A"},
		{85, "B"},
		{75, "C"},
		{65, "D"},
		{50, "F"},
		{0, "F"},
	}
	for _, tt := range tests {
		got := scoreToGrade(tt.score)
		if got != tt.want {
			t.Errorf("scoreToGrade(%.1f) = %q, want %q", tt.score, got, tt.want)
		}
	}
}

func TestEvidenceTextWithMarkers(t *testing.T) {
	f := model.Finding{
		Evidence: []model.Evidence{
			{Marker: "marker1"},
			{Marker: "marker2"},
		},
	}
	got := evidenceText(f)
	if got != "marker1 marker2" {
		t.Errorf("evidenceText = %q, want %q", got, "marker1 marker2")
	}
}

func TestEvidenceTextFallback(t *testing.T) {
	f := model.Finding{Summary: "summary text"}
	got := evidenceText(f)
	if got != "summary text" {
		t.Errorf("evidenceText = %q, want %q", got, "summary text")
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
}

func TestAnalyzeSingleFinding(t *testing.T) {
	r := Analyze([]model.Finding{finding("I should note that my reasoning here is uncertain")})
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "test-model" {
		t.Errorf("model = %q, want %q", r.Models[0].Model, "test-model")
	}
	if r.Models[0].Grade == "" {
		t.Error("grade should not be empty")
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "model-a"}, Lab: "lab1", Summary: "let me think through this step by step"},
		{Model: model.ModelRef{Model: "model-b"}, Lab: "lab1", Summary: "without a doubt this is absolutely correct"},
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" || r.WorstModel == "" {
		t.Error("best/worst model should not be empty")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	f := model.Finding{Lab: "lab1", Summary: "test"}
	r := Analyze([]model.Finding{f})
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown model, got %q", r.Models[0].Model)
	}
}

func TestAnalyzeSampleAndLabCount(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m"}, Lab: "lab1", Summary: "a"},
		{Model: model.ModelRef{Model: "m"}, Lab: "lab2", Summary: "b"},
		{Model: model.ModelRef{Model: "m"}, Lab: "lab2", Summary: "c"},
	}
	r := Analyze(findings)
	if r.Models[0].SampleCount != 3 {
		t.Errorf("sample count = %d, want 3", r.Models[0].SampleCount)
	}
	if r.Models[0].LabCount != 2 {
		t.Errorf("lab count = %d, want 2", r.Models[0].LabCount)
	}
}

func TestAnalyzeSorting(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "low"}, Lab: "l", Summary: "hello"},
		{Model: model.ModelRef{Model: "high"}, Lab: "l", Summary: "I should note my reasoning is based on step by step analysis. Let me think through the key factors."},
	}
	r := Analyze(findings)
	if r.Models[0].MetacognitionScore < r.Models[1].MetacognitionScore {
		t.Error("models should be sorted by score descending")
	}
}

func TestAnalyzeMeanScore(t *testing.T) {
	r := Analyze([]model.Finding{finding("test")})
	if r.MeanScore < 0 || r.MeanScore > 100 {
		t.Errorf("mean score out of range: %.1f", r.MeanScore)
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No metacognition data") {
		t.Error("expected empty message in text output")
	}
}

func TestFormatTextWithData(t *testing.T) {
	r := Analyze([]model.Finding{finding("I should note my reasoning here")})
	text := FormatText(r)
	if !strings.Contains(text, "Metacognition Analysis") {
		t.Error("expected header")
	}
	if !strings.Contains(text, "test-model") {
		t.Error("expected model name")
	}
	if !strings.Contains(text, "self_awareness") {
		t.Error("expected detail field")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]model.Finding{finding("test")})
	j := FormatJSON(r)
	if !strings.Contains(j, "metacognition_score") {
		t.Error("expected metacognition_score in JSON")
	}
	if !strings.Contains(j, "test-model") {
		t.Error("expected model name in JSON")
	}
}

func TestFormatJSONEmpty(t *testing.T) {
	r := Analyze(nil)
	j := FormatJSON(r)
	if !strings.Contains(j, "models") {
		t.Error("expected models key in JSON")
	}
}

func TestScoreClampedAbove(t *testing.T) {
	text := strings.Repeat("I should note my reasoning here is step by step working through this. ", 50)
	score, _ := scoreResponse(text, text)
	if score > 100 {
		t.Errorf("score should be clamped to 100, got %.1f", score)
	}
}

func TestScoreClampedBelow(t *testing.T) {
	text := strings.Repeat("without a doubt I guarantee this is absolutely the only correct answer undeniable. ", 20)
	score, _ := scoreResponse(text, "")
	if score < 0 {
		t.Errorf("score should be clamped to 0, got %.1f", score)
	}
}

func TestConcernsCount(t *testing.T) {
	text := "Without a doubt this is absolutely the only correct answer. I guarantee it is undeniable. There is no question."
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m"}, Lab: "l", Summary: text},
	}
	r := Analyze(findings)
	if r.Models[0].Concerns < 1 {
		t.Error("expected at least 1 concern for overconfident response")
	}
}

func TestTransparencyPhrases(t *testing.T) {
	text := "My reasoning is that the key factors are important. Here's why I think this."
	c := countPhrases(text, transparencyPhrases)
	if c < 2 {
		t.Errorf("expected at least 2 transparency phrases, got %d", c)
	}
}

func TestUncertaintyPhrases(t *testing.T) {
	text := "I'm not certain about this and I could be wrong. This is my best understanding."
	c := countPhrases(text, uncertaintyPhrases)
	if c < 2 {
		t.Errorf("expected at least 2 uncertainty phrases, got %d", c)
	}
}
