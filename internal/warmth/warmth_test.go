package warmth

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(mn, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: mn},
		Summary: summary,
		Lab:     "lab1",
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Errorf("expected empty models, got %d", len(r.Models))
	}
}

func TestAnalyzeSingleWarmResponse(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("gpt-4", "I understand your concern. Happy to help you with this. Great question!"),
	})
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].WarmthScore <= 50 {
		t.Errorf("warm response should score above 50, got %.1f", r.Models[0].WarmthScore)
	}
}

func TestAnalyzeColdResponse(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("gpt-4", "As I said before, that's wrong. You should know this. Obviously it cannot work."),
	})
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].WarmthScore >= 50 {
		t.Errorf("cold response should score below 50, got %.1f", r.Models[0].WarmthScore)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("warm-model", "Happy to help! Great question. I understand completely."),
		finding("cold-model", "That's wrong. You should know this. As I said."),
	})
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == r.WorstModel {
		t.Error("best and worst should differ")
	}
}

func TestGradeMapping(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"},
		{85, "B"},
		{75, "C"},
		{65, "D"},
		{45, "F"},
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
	if !strings.Contains(text, "No warmth data") {
		t.Error("empty report should say no data")
	}
}

func TestFormatTextHeader(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("gpt-4", "Happy to help!"),
	})
	text := FormatText(r)
	if !strings.Contains(text, "Response Warmth Analysis") {
		t.Error("missing report header")
	}
}

func TestUnknownModelName(t *testing.T) {
	r := Analyze([]model.Finding{
		{Summary: "test response", Lab: "lab1"},
	})
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown', got %s", r.Models[0].Model)
	}
}

func TestLabCounting(t *testing.T) {
	r := Analyze([]model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Summary: "test", Lab: "lab1"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "test", Lab: "lab2"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "test", Lab: "lab1"},
	})
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestSampleCounting(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "a"),
		finding("m1", "b"),
		finding("m1", "c"),
	})
	if r.Models[0].SampleCount != 3 {
		t.Errorf("expected 3 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestEncouragingBoostsScore(t *testing.T) {
	plain := Analyze([]model.Finding{finding("m1", "Here is the answer.")})
	warm := Analyze([]model.Finding{finding("m1", "Excellent! You're on the right track. Nice work on this.")})
	if warm.Models[0].WarmthScore <= plain.Models[0].WarmthScore {
		t.Errorf("encouraging response (%.1f) should score higher than plain (%.1f)",
			warm.Models[0].WarmthScore, plain.Models[0].WarmthScore)
	}
}

func TestDismissiveLowersScore(t *testing.T) {
	plain := Analyze([]model.Finding{finding("m1", "Here is the answer.")})
	dismiss := Analyze([]model.Finding{finding("m1", "That's trivial. It's simple. Anyone would know this.")})
	if dismiss.Models[0].WarmthScore >= plain.Models[0].WarmthScore {
		t.Errorf("dismissive response (%.1f) should score lower than plain (%.1f)",
			dismiss.Models[0].WarmthScore, plain.Models[0].WarmthScore)
	}
}

func TestMeanScore(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "Happy to help!"),
		finding("m2", "That's wrong."),
	})
	if r.MeanScore <= 0 {
		t.Error("mean score should be positive")
	}
}

func TestWarmthRatio(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "I understand. Happy to help. Great question. Of course."),
	})
	if r.Models[0].Detail.WarmthRatio < 0.5 {
		t.Errorf("expected high warmth ratio, got %.2f", r.Models[0].Detail.WarmthRatio)
	}
}

func TestScoreClamping(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", strings.Repeat("Happy to help! Great question! I understand! Excellent! ", 20)),
	})
	if r.Models[0].WarmthScore > 100 {
		t.Errorf("score should be capped at 100, got %.1f", r.Models[0].WarmthScore)
	}

	r2 := Analyze([]model.Finding{
		finding("m1", strings.Repeat("That's wrong. As I said. You should know. Not my problem. ", 20)),
	})
	if r2.Models[0].WarmthScore < 0 {
		t.Errorf("score should not go below 0, got %.1f", r2.Models[0].WarmthScore)
	}
}
