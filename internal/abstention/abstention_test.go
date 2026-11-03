package abstention

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(modelName, summary string, evidence ...string) model.Finding {
	var evs []model.Evidence
	for _, e := range evidence {
		evs = append(evs, model.Evidence{Marker: e})
	}
	return model.Finding{
		Model:    model.ModelRef{Model: modelName},
		Summary:  summary,
		Evidence: evs,
		Lab:      "test-lab",
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
		finding("gpt-4", "The answer is 42.", "clear factual question"),
	})
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].AbstentionScore <= 0 {
		t.Error("expected positive score")
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("gpt-4", "Answer.", "evidence"),
		finding("claude", "Response.", "evidence"),
	})
	if len(r.Models) != 2 {
		t.Errorf("expected 2 models, got %d", len(r.Models))
	}
}

func TestAppropriateAbstention(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "I don't know the answer to this question.", "unknown ambiguous data"),
	})
	if r.Models[0].Detail.AppropriateAbstentions != 1 {
		t.Error("expected appropriate abstention")
	}
	if r.Models[0].AbstentionScore < 50 {
		t.Errorf("appropriate abstention should score well, got %.1f", r.Models[0].AbstentionScore)
	}
}

func TestInappropriateAbstention(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "I'm not sure about this.", "clear definitive answer available"),
	})
	if r.Models[0].Detail.InappropriateAbstentions != 1 {
		t.Error("expected inappropriate abstention")
	}
}

func TestMissedAbstention(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "The answer is definitely 42.", "unknown insufficient data"),
	})
	if r.Models[0].Detail.MissedAbstentions != 1 {
		t.Error("expected missed abstention")
	}
}

func TestConfidentCorrectAnswer(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "The answer is clearly Paris.", "Paris is the capital of France"),
	})
	if r.Models[0].AbstentionScore < 50 {
		t.Errorf("confident correct answer should score well, got %.1f", r.Models[0].AbstentionScore)
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
	if !strings.Contains(text, "No abstention data available") {
		t.Error("expected empty message")
	}
}

func TestFormatTextHeader(t *testing.T) {
	r := Analyze([]model.Finding{finding("test-model", "test", "evidence")})
	text := FormatText(r)
	if !strings.Contains(text, "Abstention Detection Analysis") {
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
		finding("m1", "response one", "evidence one"),
		finding("m2", "response two", "evidence two"),
	})
	if r.MeanScore <= 0 {
		t.Error("expected positive mean score")
	}
}

func TestAbstentionRate(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "I don't know.", "unknown data"),
		finding("m1", "The answer is yes.", "clear evidence"),
	})
	if r.Models[0].Detail.AbstentionRate < 0.4 || r.Models[0].Detail.AbstentionRate > 0.6 {
		t.Errorf("expected ~50%% abstention rate, got %.2f", r.Models[0].Detail.AbstentionRate)
	}
}
