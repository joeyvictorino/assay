package clarity

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func makeFinding(modelName, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: modelName},
		Summary: summary,
		Lab:     "test-lab",
	}
}

func TestScoreResponseClean(t *testing.T) {
	score, detail := scoreResponse("The function returns an integer. It takes two arguments.", "")
	if score < 60 {
		t.Errorf("clean response should score >= 60, got %.1f", score)
	}
	if detail.JargonCount != 0 {
		t.Errorf("no jargon expected, got %d", detail.JargonCount)
	}
}

func TestScoreResponseWithJargon(t *testing.T) {
	text := "This paradigm shift leverages synergy to operationalize the holistic approach and core competency."
	score, detail := scoreResponse(text, "")
	if detail.JargonCount == 0 {
		t.Error("expected jargon detections")
	}
	if score >= 85 {
		t.Errorf("jargon-heavy response should score < 85, got %.1f", score)
	}
}

func TestScoreResponseWithAmbiguity(t *testing.T) {
	text := "It depends on various factors. In some cases under certain circumstances it can vary. Sometimes in general things differ."
	_, detail := scoreResponse(text, "")
	if detail.AmbiguityCount == 0 {
		t.Error("expected ambiguity detections")
	}
}

func TestScoreResponseLongSentences(t *testing.T) {
	words := make([]string, 35)
	for i := range words {
		words[i] = "word"
	}
	text := strings.Join(words, " ") + "."
	_, detail := scoreResponse(text, "")
	if detail.LongSentences == 0 {
		t.Error("expected long sentence detection")
	}
}

func TestScoreResponseEmpty(t *testing.T) {
	score, _ := scoreResponse("", "")
	if score < 80 {
		t.Errorf("empty response should get baseline score, got %.1f", score)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"},
		{85, "A"},
		{75, "B"},
		{60, "C"},
		{45, "D"},
		{30, "F"},
	}
	for _, tt := range tests {
		got := scoreToGrade(tt.score)
		if got != tt.grade {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tt.score, got, tt.grade)
		}
	}
}

func TestSplitSentences(t *testing.T) {
	text := "Hello world. How are you? I am fine!"
	sentences := splitSentences(text)
	if len(sentences) != 3 {
		t.Errorf("expected 3 sentences, got %d", len(sentences))
	}
}

func TestSplitSentencesNoTerminator(t *testing.T) {
	text := "Hello world without punctuation"
	sentences := splitSentences(text)
	if len(sentences) != 1 {
		t.Errorf("expected 1 sentence, got %d", len(sentences))
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Error("empty findings should produce empty report")
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "The function returns 42."),
		makeFinding("gpt-4", "This is a clear answer."),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].SampleCount != 2 {
		t.Errorf("expected 2 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "Clear answer."),
		makeFinding("claude", "This paradigm shift leverages synergy to operationalize the holistic approach in some cases under certain circumstances."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" || r.WorstModel == "" {
		t.Error("expected best/worst model to be set")
	}
}

func TestAnalyzeSortsByScore(t *testing.T) {
	findings := []model.Finding{
		makeFinding("unclear", "This paradigm shift leverages synergy to operationalize it depends in some cases under certain circumstances it can vary sometimes in general."),
		makeFinding("clear", "Returns 42."),
	}
	r := Analyze(findings)
	if len(r.Models) < 2 {
		t.Fatal("expected 2 models")
	}
	if r.Models[0].ClarityScore < r.Models[1].ClarityScore {
		t.Error("models should be sorted by score descending")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		{Summary: "test", Lab: "lab1"},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown, got %s", r.Models[0].Model)
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No clarity data") {
		t.Error("expected empty message")
	}
}

func TestFormatTextHeader(t *testing.T) {
	findings := []model.Finding{makeFinding("gpt-4", "test")}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Response Clarity Analysis") {
		t.Error("expected header")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{makeFinding("gpt-4", "test")}
	r := Analyze(findings)
	j := FormatJSON(r)
	if !strings.Contains(j, "clarity_score") {
		t.Error("expected clarity_score in JSON")
	}
}

func TestEvidenceText(t *testing.T) {
	f := model.Finding{
		Summary: "fallback",
		Evidence: []model.Evidence{
			{Marker: "m1"},
			{Marker: "m2"},
		},
	}
	got := evidenceText(f)
	if got != "m1 m2" {
		t.Errorf("expected joined markers, got %q", got)
	}
}

func TestLabCount(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab1", Summary: "a"},
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab2", Summary: "b"},
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab1", Summary: "c"},
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}
