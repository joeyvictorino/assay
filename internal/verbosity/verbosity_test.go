package verbosity

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
		Lab:     "lab1",
	}
}

func TestScoreResponseEmpty(t *testing.T) {
	score, detail := scoreResponse("")
	if score != 100 {
		t.Errorf("expected 100, got %.1f", score)
	}
	if detail.WordCount != 0 {
		t.Errorf("expected 0 words, got %d", detail.WordCount)
	}
}

func TestScoreResponseClean(t *testing.T) {
	score, _ := scoreResponse("This is a clear and concise response.")
	if score < 70 {
		t.Errorf("expected high score for clean text, got %.1f", score)
	}
}

func TestScoreResponseFillerPenalty(t *testing.T) {
	text := "It is important to note that we should consider this. It should be noted that the approach works."
	score, detail := scoreResponse(text)
	if detail.FillerPhrases < 2 {
		t.Errorf("expected at least 2 filler phrases, got %d", detail.FillerPhrases)
	}
	if score >= 85 {
		t.Errorf("expected penalty for fillers, got %.1f", score)
	}
}

func TestScoreResponseRedundantPenalty(t *testing.T) {
	text := "The end result of the past history shows the final outcome."
	_, detail := scoreResponse(text)
	if detail.RedundantPhrases < 2 {
		t.Errorf("expected at least 2 redundant phrases, got %d", detail.RedundantPhrases)
	}
}

func TestScoreResponseOverQualPenalty(t *testing.T) {
	text := "I think that perhaps it might be possible that this could work, to some extent."
	_, detail := scoreResponse(text)
	if detail.OverQualifications < 2 {
		t.Errorf("expected at least 2 over-qualifications, got %d", detail.OverQualifications)
	}
}

func TestScoreBounded(t *testing.T) {
	text := strings.Repeat("It is important to note that basically the end result is. ", 20)
	score, _ := scoreResponse(text)
	if score < 0 {
		t.Errorf("score should not be negative, got %.1f", score)
	}
	if score > 100 {
		t.Errorf("score should not exceed 100, got %.1f", score)
	}
}

func TestInfoDensityHigh(t *testing.T) {
	d := infoDensity("Each word here is unique and different from others in this sentence.")
	if d < 0.5 {
		t.Errorf("expected high density, got %.2f", d)
	}
}

func TestInfoDensityLow(t *testing.T) {
	d := infoDensity("the the the the the the the the the the")
	if d >= 0.2 {
		t.Errorf("expected low density, got %.2f", d)
	}
}

func TestInfoDensityEmpty(t *testing.T) {
	d := infoDensity("")
	if d != 1.0 {
		t.Errorf("expected 1.0 for empty, got %.2f", d)
	}
}

func TestSplitSentences(t *testing.T) {
	n := splitSentences("First. Second! Third?")
	if n != 3 {
		t.Errorf("expected 3 sentences, got %d", n)
	}
}

func TestSplitSentencesNoDelim(t *testing.T) {
	n := splitSentences("No sentence delimiter here")
	if n != 1 {
		t.Errorf("expected 1, got %d", n)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"}, {85, "B"}, {75, "C"}, {65, "D"}, {40, "F"},
	}
	for _, tt := range tests {
		if g := scoreToGrade(tt.score); g != tt.grade {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tt.score, g, tt.grade)
		}
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Errorf("expected empty report")
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	fs := []model.Finding{
		finding("gpt-4", "Clear concise answer."),
		finding("gpt-4", "Another good response."),
	}
	r := Analyze(fs)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].SampleCount != 2 {
		t.Errorf("expected 2 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	fs := []model.Finding{
		finding("gpt-4", "Short answer."),
		finding("claude", "It is important to note that basically the end result is that we need to consider things."),
	}
	r := Analyze(fs)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.MostConcise == "" {
		t.Error("expected most_concise to be set")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	fs := []model.Finding{
		{Summary: "Clean text.", Lab: "lab1"},
	}
	r := Analyze(fs)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown, got %s", r.Models[0].Model)
	}
}

func TestAnalyzeLabCount(t *testing.T) {
	fs := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Summary: "Clean.", Lab: "lab1"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "Clean.", Lab: "lab2"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "Clean.", Lab: "lab3"},
	}
	r := Analyze(fs)
	if r.Models[0].LabCount != 3 {
		t.Errorf("expected 3 labs, got %d", r.Models[0].LabCount)
	}
}

func TestAnalyzeTooVerbose(t *testing.T) {
	verbose := strings.Repeat("It is important to note that basically the end result is the final outcome. ", 15)
	fs := []model.Finding{
		finding("bad", verbose),
	}
	r := Analyze(fs)
	if r.Models[0].TooVerbose != 1 {
		t.Errorf("expected 1 too verbose, got %d", r.Models[0].TooVerbose)
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No verbosity data") {
		t.Error("expected empty message")
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	fs := []model.Finding{
		finding("gpt-4", "Short and concise."),
	}
	r := Analyze(fs)
	text := FormatText(r)
	if !strings.Contains(text, "Verbosity Analysis") {
		t.Error("expected header")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Error("expected model name")
	}
}

func TestFormatJSONValid(t *testing.T) {
	fs := []model.Finding{
		finding("gpt-4", "Short response."),
	}
	r := Analyze(fs)
	out := FormatJSON(r)
	var data map[string]any
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := data["models"]; !ok {
		t.Error("expected models key")
	}
}

func TestFormatJSONRoundtrip(t *testing.T) {
	r := Analyze(nil)
	out := FormatJSON(r)
	var data Report
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestCountPhrases(t *testing.T) {
	text := "basically the end result is basically clear"
	c := countPhrases(text, []string{"basically", "end result"})
	if c != 3 {
		t.Errorf("expected 3, got %d", c)
	}
}

func TestAnalyzeSortedByScore(t *testing.T) {
	verbose := strings.Repeat("It is important to note that basically the end result is. ", 10)
	fs := []model.Finding{
		finding("bad", verbose),
		finding("good", "Clear."),
	}
	r := Analyze(fs)
	if r.Models[0].VerbosityScore < r.Models[1].VerbosityScore {
		t.Error("expected sorted by score descending")
	}
}
