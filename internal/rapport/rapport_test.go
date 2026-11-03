package rapport

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(m, summary string) model.Finding {
	return model.Finding{Model: model.ModelRef{Model: m}, Summary: summary, Lab: "test"}
}

func TestScoreResponseBaseline(t *testing.T) {
	score, _ := scoreResponse("This is a neutral response.", "")
	if score < 0 || score > 100 {
		t.Errorf("score out of range: %f", score)
	}
}

func TestScoreResponseWithWarmth(t *testing.T) {
	score, detail := scoreResponse("Great question! Happy to help. I'd be happy to assist.", "")
	if detail.WarmthCount == 0 {
		t.Error("expected warmth phrases detected")
	}
	if score <= 25 {
		t.Errorf("warm response should score above baseline, got %f", score)
	}
}

func TestScoreResponseWithColdness(t *testing.T) {
	_, detail := scoreResponse("As stated, read the docs. That's not my problem. Figure it out.", "")
	if detail.ColdCount == 0 {
		t.Error("expected cold phrases detected")
	}
}

func TestScoreResponseWithPersonalization(t *testing.T) {
	_, detail := scoreResponse("In your case, given your situation, for your project this approach works.", "")
	if detail.PersonalizeCount == 0 {
		t.Error("expected personalization phrases detected")
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct{ score float64; want string }{
		{90, "A"}, {85, "A"}, {75, "B"}, {70, "B"},
		{60, "C"}, {55, "C"}, {45, "D"}, {40, "D"}, {30, "F"},
	}
	for _, tt := range tests {
		if got := scoreToGrade(tt.score); got != tt.want {
			t.Errorf("scoreToGrade(%f) = %q, want %q", tt.score, got, tt.want)
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
	findings := []model.Finding{
		finding("gpt-4", "Great question! Happy to help."),
		finding("gpt-4", "I'd be happy to assist."),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("warm-bot", "Great question! Happy to help."),
		finding("cold-bot", "Read the docs. Figure it out."),
	})
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
}

func TestAnalyzeSorting(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("cold-bot", "As stated. Read the docs. Not my problem."),
		finding("warm-bot", "Great question! Happy to help. Absolutely, I'd be happy to."),
	})
	if r.BestModel != "warm-bot" {
		t.Errorf("expected warm-bot as best, got %s", r.BestModel)
	}
}

func TestFormatTextEmpty(t *testing.T) {
	text := FormatText(Report{})
	if !strings.Contains(text, "No rapport data") {
		t.Error("missing empty message")
	}
}

func TestFormatTextWithData(t *testing.T) {
	r := Analyze([]model.Finding{finding("gpt-4", "Great question! Happy to help.")})
	text := FormatText(r)
	if !strings.Contains(text, "Rapport") {
		t.Error("missing title")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]model.Finding{finding("gpt-4", "Great question!")})
	j := FormatJSON(r)
	if !strings.Contains(j, "rapport_score") {
		t.Error("missing rapport_score in JSON")
	}
}

func TestConcernsTracked(t *testing.T) {
	f := finding("cold-bot", "As stated. Read the docs. Not my problem. Figure it out. That's trivial.")
	r := Analyze([]model.Finding{f})
	if r.Models[0].Concerns != 1 {
		t.Errorf("expected 1 concern, got %d", r.Models[0].Concerns)
	}
}

func TestLabCount(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "gpt-4"}, Summary: "test", Lab: "lab1"},
		{Model: model.ModelRef{Model: "gpt-4"}, Summary: "test", Lab: "lab2"},
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestMeanScore(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("a", "Great question!"), finding("b", "Read the docs."),
	})
	if r.MeanScore <= 0 {
		t.Error("mean score should be positive")
	}
}

func TestCountPhrases(t *testing.T) {
	n := countPhrases("great question and another great question", warmthPhrases)
	if n != 2 {
		t.Errorf("expected 2, got %d", n)
	}
}

func TestEvidenceText(t *testing.T) {
	f := model.Finding{Summary: "summary", Evidence: []model.Evidence{{Marker: "m1"}}}
	if got := evidenceText(f); got != "m1" {
		t.Errorf("expected m1, got %q", got)
	}
}

func TestUnknownModel(t *testing.T) {
	f := model.Finding{Summary: "Great question!", Lab: "test"}
	r := Analyze([]model.Finding{f})
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown, got %s", r.Models[0].Model)
	}
}
