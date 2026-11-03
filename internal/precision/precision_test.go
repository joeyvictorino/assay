package precision

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func mkFinding(modelName, lab, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: modelName},
		Lab:     lab,
		Summary: summary,
	}
}

func TestAnalyzeTextEmpty(t *testing.T) {
	score, detail := AnalyzeText("")
	if score != 50.0 {
		t.Errorf("empty text should score 50, got %.1f", score)
	}
	if detail.WordCount != 0 {
		t.Errorf("expected 0 words, got %d", detail.WordCount)
	}
}

func TestAnalyzeTextPrecise(t *testing.T) {
	text := "The API returns exactly 42 results with 99.5% accuracy per RFC 7231. The latency is 150ms at the 95th percentile."
	score, detail := AnalyzeText(text)
	if score <= 50 {
		t.Errorf("precise text should score above 50, got %.1f", score)
	}
	if detail.NumberCount == 0 {
		t.Error("expected numbers to be detected")
	}
	if detail.PercentCount == 0 {
		t.Error("expected percentages to be detected")
	}
}

func TestAnalyzeTextImprecise(t *testing.T) {
	text := "Usually some things happen often. Many cases generally show that several factors are typically involved in various ways."
	score, detail := AnalyzeText(text)
	if score >= 50 {
		t.Errorf("imprecise text should score below 50, got %.1f", score)
	}
	if detail.WeaselCount == 0 {
		t.Error("expected weasel words to be detected")
	}
	if detail.QuantifierCount == 0 {
		t.Error("expected quantifiers to be detected")
	}
}

func TestNumberCounting(t *testing.T) {
	text := "There are 42 items at 9.99 each for 3 orders"
	_, detail := AnalyzeText(text)
	if detail.NumberCount != 3 {
		t.Errorf("expected 3 numbers, got %d", detail.NumberCount)
	}
}

func TestPercentCounting(t *testing.T) {
	text := "Success rate is 99.5% and error rate is 0.5%"
	_, detail := AnalyzeText(text)
	if detail.PercentCount != 2 {
		t.Errorf("expected 2 percents, got %d", detail.PercentCount)
	}
}

func TestCitationDetection(t *testing.T) {
	text := "Per RFC 7231 and CVE-2023-1234, the issue relates to CWE-89"
	_, detail := AnalyzeText(text)
	if detail.CitationCount < 3 {
		t.Errorf("expected at least 3 citations, got %d", detail.CitationCount)
	}
}

func TestWeaselWordDetection(t *testing.T) {
	text := "This usually happens and frequently causes issues. It typically results in problems."
	count := countPhrases(text, weaselPhrases)
	if count < 3 {
		t.Errorf("expected at least 3 weasel words, got %d", count)
	}
}

func TestQuantifierDetection(t *testing.T) {
	text := "Some users and many systems have several issues with various configurations."
	count := countPhrases(text, quantifierPhrases)
	if count < 3 {
		t.Errorf("expected at least 3 quantifiers, got %d", count)
	}
}

func TestScoreClamping(t *testing.T) {
	imprecise := "usually often generally typically commonly frequently sometimes occasionally " +
		"some many few several various a lot numerous"
	score, _ := AnalyzeText(imprecise)
	if score < 0 || score > 100 {
		t.Errorf("score should be clamped to [0,100], got %.1f", score)
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

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "The API returns 42 results per RFC 7231 with 99% accuracy."),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.BestModel != "gpt-4" {
		t.Errorf("expected gpt-4, got %s", r.BestModel)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "Exactly 42 items per RFC 7231 at 99.5% success rate."),
		mkFinding("claude", "lab1", "Usually some things are generally involved in various ways."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == r.WorstModel {
		t.Error("best and worst should differ")
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Errorf("expected 0 models, got %d", len(r.Models))
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		mkFinding("", "lab1", "There are 10 items."),
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown, got %s", r.Models[0].Model)
	}
}

func TestFormatText(t *testing.T) {
	findings := []model.Finding{
		mkFinding("test-model", "lab", "A test with 42 items."),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Precision") {
		t.Error("expected header")
	}
	if !strings.Contains(text, "test-model") {
		t.Error("expected model name")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No precision data") {
		t.Error("expected empty message")
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		mkFinding("a", "lab1", "Exactly 10 items."),
		mkFinding("b", "lab1", "About some things."),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Errorf("mean should be positive, got %.1f", r.MeanScore)
	}
}
