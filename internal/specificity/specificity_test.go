package specificity

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(mod, lab, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: mod},
		Lab:     lab,
		Summary: summary,
	}
}

func TestAnalyzeTextSpecific(t *testing.T) {
	text := "According to the RFC 7231 specification, HTTP status code 429 indicates rate limiting. For example, Cloudflare returns this when exceeding 1000 requests per minute."
	score, detail := AnalyzeText(text)
	if score <= 50 {
		t.Errorf("specific text should score above 50, got %.1f", score)
	}
	if detail.NumberCount == 0 {
		t.Error("expected numbers to be counted")
	}
	if detail.ConcreteCount == 0 {
		t.Error("expected concrete phrases to be detected")
	}
}

func TestAnalyzeTextVague(t *testing.T) {
	text := "Maybe you could try some things. It seems like there are various factors and several issues that might be causing the problem somehow."
	score, detail := AnalyzeText(text)
	if score >= 50 {
		t.Errorf("vague text should score below 50, got %.1f", score)
	}
	if detail.HedgeCount == 0 {
		t.Error("expected hedges to be detected")
	}
	if detail.VagueCount == 0 {
		t.Error("expected vague phrases to be detected")
	}
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

func TestNumberCounting(t *testing.T) {
	text := "There are 42 items at $9.99 each for a total of 3 orders"
	_, detail := AnalyzeText(text)
	if detail.NumberCount != 3 {
		t.Errorf("expected 3 numbers, got %d", detail.NumberCount)
	}
}

func TestProperNounEstimation(t *testing.T) {
	count := estimateProperNouns("the Cloudflare API and Amazon Web Services handle this")
	if count < 3 {
		t.Errorf("expected at least 3 proper nouns, got %d", count)
	}
}

func TestHedgeDetection(t *testing.T) {
	text := "I think this could be an issue. Perhaps maybe we should look into it."
	count := countPhrases(text, hedgePhrases)
	if count < 3 {
		t.Errorf("expected at least 3 hedges, got %d", count)
	}
}

func TestConcreteDetection(t *testing.T) {
	text := "For example, Python's list comprehension works as follows. Specifically, including items such as dictionaries."
	count := countPhrases(text, concretePhrases)
	if count < 3 {
		t.Errorf("expected at least 3 concrete phrases, got %d", count)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{95, "A"}, {85, "B"}, {75, "C"}, {65, "D"}, {40, "F"},
	}
	for _, tt := range tests {
		if got := scoreToGrade(tt.score); got != tt.want {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestScoreClamping(t *testing.T) {
	vague := "maybe perhaps possibly might could be it seems sort of kind of " +
		"things stuff something somehow somewhere various several many some certain " +
		"basically essentially generally a lot issues factors"
	score, _ := AnalyzeText(vague)
	if score < 0 || score > 100 {
		t.Errorf("score should be clamped to [0,100], got %.1f", score)
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "lab1", "For example, the API returns status code 200 with a JSON body including 3 fields."),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "gpt-4" {
		t.Errorf("expected gpt-4, got %s", r.Models[0].Model)
	}
	if r.BestModel != "gpt-4" {
		t.Errorf("expected best model gpt-4, got %s", r.BestModel)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "lab1", "Specifically, step 1 involves calling the API at port 8080 with 5 retries."),
		finding("claude", "lab1", "Maybe try some things. It seems like there are various issues."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == r.WorstModel {
		t.Error("best and worst should differ with different quality texts")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		finding("", "lab1", "For instance, the response includes 7 data fields."),
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown, got %s", r.Models[0].Model)
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Errorf("expected 0 models, got %d", len(r.Models))
	}
}

func TestLabCounting(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "lab1", "Specifically there are 10 items."),
		finding("gpt-4", "lab2", "For example, 20 records."),
		finding("gpt-4", "lab1", "Including 5 entries."),
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
	if r.Models[0].SampleCount != 3 {
		t.Errorf("expected 3 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestFormatText(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "lab1", "For example, the 3 endpoints return JSON."),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Response Specificity Analysis") {
		t.Error("missing title")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Error("missing model name")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	text := FormatText(Report{})
	if !strings.Contains(text, "No specificity data") {
		t.Error("missing empty message")
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "lab1", "For example, 10 items specifically."),
		finding("m2", "lab1", "Maybe some things."),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Error("mean score should be positive")
	}
}
