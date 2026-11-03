package paraphrase

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(modelName, summary, lab string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: modelName},
		Summary: summary,
		Lab:     lab,
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
		finding("gpt-4", "The answer is 42.", "lab-1"),
	})
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].ParaphraseScore <= 0 {
		t.Error("expected positive score")
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("gpt-4", "Answer A.", "lab-1"),
		finding("claude", "Answer B.", "lab-1"),
	})
	if len(r.Models) != 2 {
		t.Errorf("expected 2 models, got %d", len(r.Models))
	}
}

func TestConsistentParaphrases(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "The capital of France is Paris, a major European city.", "lab-1"),
		finding("m1", "Paris is the capital of France, known as a major European city.", "lab-1"),
	})
	if r.Models[0].ParaphraseScore < 50 {
		t.Errorf("consistent paraphrases should score well, got %.1f", r.Models[0].ParaphraseScore)
	}
}

func TestInconsistentParaphrases(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "The answer is definitely yes, absolutely correct.", "lab-1"),
		finding("m1", "The answer is definitely no, completely wrong.", "lab-1"),
	})
	baseline := Analyze([]model.Finding{
		finding("m1", "The answer is yes, that is correct and verified.", "lab-1"),
		finding("m1", "The answer is yes, confirmed and validated properly.", "lab-1"),
	})
	if r.Models[0].ParaphraseScore >= baseline.Models[0].ParaphraseScore {
		t.Error("inconsistent paraphrases should score lower")
	}
}

func TestGradeMapping(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"}, {85, "B"}, {75, "C"}, {65, "D"}, {45, "F"},
	}
	for _, tt := range tests {
		if got := scoreToGrade(tt.score); got != tt.grade {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tt.score, got, tt.grade)
		}
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No paraphrase data available") {
		t.Error("expected empty message")
	}
}

func TestFormatTextHeader(t *testing.T) {
	r := Analyze([]model.Finding{finding("test-model", "test", "lab-1")})
	text := FormatText(r)
	if !strings.Contains(text, "Paraphrase Consistency Analysis") {
		t.Error("missing title")
	}
}

func TestUnknownModelName(t *testing.T) {
	r := Analyze([]model.Finding{{Summary: "test", Lab: "lab1"}})
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown', got %s", r.Models[0].Model)
	}
}

func TestLabCounting(t *testing.T) {
	r := Analyze([]model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Summary: "a", Lab: "lab-1"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "b", Lab: "lab-2"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "c", Lab: "lab-1"},
	})
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestMeanScore(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "response one", "lab-1"),
		finding("m2", "response two", "lab-2"),
	})
	if r.MeanScore <= 0 {
		t.Error("expected positive mean score")
	}
}

func TestWordOverlap(t *testing.T) {
	overlap := wordOverlap("the quick brown fox jumps over", "the quick brown fox leaps high")
	if overlap < 0.3 {
		t.Errorf("expected moderate overlap, got %.2f", overlap)
	}
}

func TestWordOverlapNoMatch(t *testing.T) {
	overlap := wordOverlap("alpha beta gamma delta", "epsilon zeta theta iota")
	if overlap > 0.1 {
		t.Errorf("expected low overlap, got %.2f", overlap)
	}
}

func TestSingleVariantLab(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "Only one response in this lab.", "lab-1"),
	})
	if r.Models[0].ParaphraseScore < 70 {
		t.Errorf("single variant should get decent score, got %.1f", r.Models[0].ParaphraseScore)
	}
}
