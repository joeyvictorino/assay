package multimodality

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
	text := "The image shows a chart that displays data. The table indicates results."
	n := countPhrases(text, formatPhrases)
	if n < 2 {
		t.Errorf("expected at least 2 format phrases, got %d", n)
	}
}

func TestCountPhrasesEmpty(t *testing.T) {
	if n := countPhrases("", formatPhrases); n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

func TestCountPhrasesCaseInsensitive(t *testing.T) {
	n := countPhrases("THE IMAGE SHOWS data THE TABLE INDICATES results", formatPhrases)
	if n < 2 {
		t.Errorf("expected at least 2, got %d", n)
	}
}

func TestScoreResponseBasic(t *testing.T) {
	text := "The image shows a diagram. As the data shows, the trend shows increasing values. " +
		"The colors indicate the visual hierarchy. Combining text and image for multi-format analysis."
	score, detail := scoreResponse(text, "")
	if score < 40 {
		t.Errorf("expected score >= 40 for multimodal-rich text, got %.1f", score)
	}
	if detail.MultimodalCount < 4 {
		t.Errorf("expected at least 4 multimodal phrases, got %d", detail.MultimodalCount)
	}
}

func TestScoreResponseConfusedPenalty(t *testing.T) {
	text := "I cannot see the image. I don't have access to the file. Unable to view the content. " +
		"The format is not supported. I cannot read this type of data."
	score, detail := scoreResponse(text, "")
	if detail.FormatConfusedCount < 3 {
		t.Errorf("expected at least 3 format confused phrases, got %d", detail.FormatConfusedCount)
	}
	if score > 50 {
		t.Errorf("expected penalized score <= 50, got %.1f", score)
	}
}

func TestScoreResponseBounded(t *testing.T) {
	huge := strings.Repeat("the image shows the chart displays the table indicates ", 200)
	score, _ := scoreResponse(huge, "")
	if score > 100 {
		t.Errorf("score should not exceed 100, got %.1f", score)
	}
	if score < 0 {
		t.Errorf("score should not be negative, got %.1f", score)
	}

	bad := strings.Repeat("i cannot see the image unable to view i don't have access to ", 100)
	score2, _ := scoreResponse(bad, "")
	if score2 < 0 {
		t.Errorf("score should not be negative, got %.1f", score2)
	}
}

func TestScoreResponseEmpty(t *testing.T) {
	score, detail := scoreResponse("", "")
	if score < 0 || score > 100 {
		t.Errorf("empty score out of range: %.1f", score)
	}
	if detail.MultimodalCount != 0 {
		t.Errorf("expected 0 multimodal phrases for empty, got %d", detail.MultimodalCount)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"}, {90, "A"},
		{85, "B"}, {80, "B"},
		{75, "C"}, {70, "C"},
		{65, "D"}, {60, "D"},
		{55, "F"}, {0, "F"},
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
		t.Errorf("expected empty models, got %d", len(r.Models))
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "The image shows a diagram with the chart displays data"),
		finding("gpt-4", "The table indicates the trend shows increasing values"),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "gpt-4" {
		t.Errorf("expected gpt-4, got %s", r.Models[0].Model)
	}
	if r.Models[0].SampleCount != 2 {
		t.Errorf("expected 2 samples, got %d", r.Models[0].SampleCount)
	}
	if r.BestModel != "gpt-4" {
		t.Errorf("expected best=gpt-4, got %s", r.BestModel)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		finding("modelA", "The image shows the chart displays combining text and image the colors indicate"),
		finding("modelB", "i cannot see the image unable to view i don't have access to"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == r.WorstModel {
		t.Error("best and worst should differ")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	f := model.Finding{Lab: "lab1", Summary: "the image shows data"}
	r := Analyze([]model.Finding{f})
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown, got %s", r.Models[0].Model)
	}
}

func TestFormatTextEmpty(t *testing.T) {
	out := FormatText(Report{})
	if !strings.Contains(out, "No multimodality data") {
		t.Error("expected empty message")
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	r := Analyze([]model.Finding{finding("test", "the image shows the chart displays")})
	out := FormatText(r)
	if !strings.Contains(out, "Multimodality Analysis") {
		t.Error("expected header")
	}
	if !strings.Contains(out, "test") {
		t.Error("expected model name")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]model.Finding{finding("test", "the image shows data")})
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(parsed.Models) != 1 {
		t.Errorf("expected 1 model in JSON, got %d", len(parsed.Models))
	}
}
