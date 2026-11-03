package readability

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(mn, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: mn},
		Lab:     "lab1",
		Summary: summary,
	}
}

func TestCountSyllablesSimple(t *testing.T) {
	tests := []struct {
		word string
		want int
	}{
		{"cat", 1},
		{"hello", 2},
		{"beautiful", 3},
		{"a", 1},
		{"the", 1},
		{"understanding", 4},
	}
	for _, tt := range tests {
		got := CountSyllables(tt.word)
		if got != tt.want {
			t.Errorf("CountSyllables(%q) = %d, want %d", tt.word, got, tt.want)
		}
	}
}

func TestCountSyllablesMinimum(t *testing.T) {
	if got := CountSyllables("x"); got != 1 {
		t.Errorf("single char should be 1 syllable, got %d", got)
	}
}

func TestExtractWords(t *testing.T) {
	words := ExtractWords("Hello, world! This is a test.")
	if len(words) != 6 {
		t.Errorf("expected 6 words, got %d: %v", len(words), words)
	}
}

func TestExtractWordsEmpty(t *testing.T) {
	words := ExtractWords("")
	if len(words) != 0 {
		t.Errorf("expected 0 words, got %d", len(words))
	}
}

func TestCountSentences(t *testing.T) {
	tests := []struct {
		text string
		want int
	}{
		{"Hello world.", 1},
		{"Hello! How are you? Fine.", 3},
		{"No punctuation here", 1},
		{"", 0},
	}
	for _, tt := range tests {
		got := CountSentences(tt.text)
		if got != tt.want {
			t.Errorf("CountSentences(%q) = %d, want %d", tt.text, got, tt.want)
		}
	}
}

func TestAnalyzeTextSimple(t *testing.T) {
	score, detail := AnalyzeText("The cat sat on the mat.")
	if score < 50 || score > 100 {
		t.Errorf("simple text score out of range: %.1f", score)
	}
	if detail.WordCount != 6 {
		t.Errorf("expected 6 words, got %d", detail.WordCount)
	}
	if detail.SentenceCount != 1 {
		t.Errorf("expected 1 sentence, got %d", detail.SentenceCount)
	}
}

func TestAnalyzeTextComplex(t *testing.T) {
	text := "The extraordinarily sophisticated implementation of the architectural paradigm necessitates comprehensive understanding of multifaceted computational methodologies."
	score, detail := AnalyzeText(text)
	if detail.ComplexWords == 0 {
		t.Error("expected complex words in academic text")
	}
	if detail.FleschKincaid < 10 {
		t.Errorf("academic text should have high FK grade, got %.1f", detail.FleschKincaid)
	}
	_ = score
}

func TestAnalyzeTextEmpty(t *testing.T) {
	score, detail := AnalyzeText("")
	if score != 50.0 {
		t.Errorf("empty text should score 50, got %.1f", score)
	}
	if detail.WordCount != 0 {
		t.Errorf("empty text should have 0 words, got %d", detail.WordCount)
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
		{40, "F"},
	}
	for _, tt := range tests {
		got := scoreToGrade(tt.score)
		if got != tt.want {
			t.Errorf("scoreToGrade(%.1f) = %q, want %q", tt.score, got, tt.want)
		}
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Error("expected empty report")
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("gpt-4", "The cat sat on the mat. It was happy."),
	})
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "gpt-4" {
		t.Errorf("expected gpt-4, got %s", r.Models[0].Model)
	}
	if r.Models[0].Grade == "" {
		t.Error("expected a grade")
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("gpt-4", "Simple words. Short text."),
		finding("claude", "The extraordinarily sophisticated architectural paradigm implementation necessitates understanding."),
	})
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" || r.WorstModel == "" {
		t.Error("best/worst should be set")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("", "Hello world."),
	})
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown, got %s", r.Models[0].Model)
	}
}

func TestAnalyzeLabCounting(t *testing.T) {
	r := Analyze([]model.Finding{
		{Model: model.ModelRef{Model: "m"}, Lab: "lab1", Summary: "Hello."},
		{Model: model.ModelRef{Model: "m"}, Lab: "lab2", Summary: "World."},
	})
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestAnalyzeSampleCounting(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m", "One."),
		finding("m", "Two."),
		finding("m", "Three."),
	})
	if r.Models[0].SampleCount != 3 {
		t.Errorf("expected 3 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestFormatTextEmpty(t *testing.T) {
	text := FormatText(Report{})
	if !strings.Contains(text, "No readability data") {
		t.Error("expected empty message")
	}
}

func TestFormatTextHeader(t *testing.T) {
	r := Analyze([]model.Finding{finding("m", "Hello world.")})
	text := FormatText(r)
	if !strings.Contains(text, "Response Readability Analysis") {
		t.Error("missing header")
	}
}

func TestGunningFogPositive(t *testing.T) {
	_, detail := AnalyzeText("This is a test sentence with some words.")
	if detail.GunningFog < 0 {
		t.Errorf("gunning fog should not be negative: %.1f", detail.GunningFog)
	}
}

func TestFleschKincaidPositive(t *testing.T) {
	_, detail := AnalyzeText("Complex multisyllabic terminology.")
	if detail.FleschKincaid < 0 {
		t.Errorf("FK should not be negative: %.1f", detail.FleschKincaid)
	}
}

func TestMeanScore(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("a", "Short text."),
		finding("b", "Another short text."),
	})
	if r.MeanScore <= 0 {
		t.Errorf("mean score should be positive, got %.1f", r.MeanScore)
	}
}
