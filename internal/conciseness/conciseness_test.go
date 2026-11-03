package conciseness

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(m, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: m},
		Summary: summary,
	}
}

func TestCountWords(t *testing.T) {
	if got := countWords("hello world foo"); got != 3 {
		t.Errorf("countWords = %d, want 3", got)
	}
	if got := countWords(""); got != 0 {
		t.Errorf("countWords empty = %d, want 0", got)
	}
}

func TestCountPhrases(t *testing.T) {
	text := "it is important to note that we should note that it is important to note that"
	got := countPhrases(text, []string{"it is important to note that"})
	if got != 2 {
		t.Errorf("countPhrases = %d, want 2", got)
	}
}

func TestAnalyzeSampleClean(t *testing.T) {
	d := analyzeSample("The vulnerability exists in the authentication module.")
	if d.FillerCount != 0 || d.RedundantCount != 0 || d.HedgingCount != 0 {
		t.Errorf("expected zero padding for clean text, got filler=%d redundant=%d hedging=%d",
			d.FillerCount, d.RedundantCount, d.HedgingCount)
	}
}

func TestAnalyzeSampleVerbose(t *testing.T) {
	d := analyzeSample("It is important to note that basically the vulnerability is very important and perhaps could potentially be exploited")
	if d.FillerCount == 0 {
		t.Error("expected filler phrases")
	}
	if d.RedundantCount == 0 {
		t.Error("expected redundant qualifiers")
	}
	if d.HedgingCount == 0 {
		t.Error("expected hedging words")
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Errorf("expected 0 models, got %d", len(r.Models))
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "The authentication bypass allows unauthorized access."),
		finding("gpt-4", "SQL injection in the login form parameter."),
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
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "Clear and direct finding."),
		finding("claude", "It is important to note that basically the issue is very important and perhaps somewhat relevant."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" || r.WorstModel == "" {
		t.Error("expected best and worst models set")
	}
}

func TestConciseTextScoresHigher(t *testing.T) {
	concise := []model.Finding{
		finding("concise-model", "Authentication bypass in login endpoint."),
		finding("concise-model", "SQL injection via user input parameter."),
	}
	verbose := []model.Finding{
		finding("verbose-model", "It is important to note that basically the authentication perhaps could potentially be bypassed and it is worth mentioning that this is very important."),
		finding("verbose-model", "It should be noted that in order to exploit this maybe sort of kind of vulnerability due to the fact that it is arguably somewhat relevant."),
	}
	cr := Analyze(concise)
	vr := Analyze(verbose)
	if cr.Models[0].Score <= vr.Models[0].Score {
		t.Errorf("concise score %.2f should be > verbose score %.2f", cr.Models[0].Score, vr.Models[0].Score)
	}
}

func TestGrade(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{0.96, "A+"},
		{0.91, "A"},
		{0.86, "A-"},
		{0.81, "B+"},
		{0.76, "B"},
		{0.71, "B-"},
		{0.66, "C+"},
		{0.61, "C"},
		{0.56, "C-"},
		{0.51, "D"},
		{0.40, "F"},
	}
	for _, tt := range tests {
		if got := grade(tt.score); got != tt.want {
			t.Errorf("grade(%.2f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestFormatText(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("test-model", "A clean finding."),
	})
	text := FormatText(r)
	if !strings.Contains(text, "Conciseness Analysis") {
		t.Error("missing title")
	}
	if !strings.Contains(text, "test-model") {
		t.Error("missing model name")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	text := FormatText(Report{})
	if !strings.Contains(text, "No models analyzed") {
		t.Error("expected empty message")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("test-model", "Finding summary."),
	})
	j := FormatJSON(r)
	if !strings.Contains(j, "test-model") {
		t.Error("missing model in JSON")
	}
}

func TestScoreBounds(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m", "word"),
	})
	for _, m := range r.Models {
		if m.Score < 0 || m.Score > 1 {
			t.Errorf("score out of bounds: %.2f", m.Score)
		}
	}
}

func TestMeanScore(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("a", "Clean text."),
		finding("b", "Also clean."),
	})
	if r.MeanScore < 0 || r.MeanScore > 1 {
		t.Errorf("mean score out of bounds: %.2f", r.MeanScore)
	}
}

func TestSortedByScore(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("verbose", "It is important to note that basically perhaps maybe sort of kind of in order to due to the fact that."),
		finding("concise", "Direct finding."),
	})
	if len(r.Models) == 2 && r.Models[0].Score < r.Models[1].Score {
		t.Error("models should be sorted by score descending")
	}
}
