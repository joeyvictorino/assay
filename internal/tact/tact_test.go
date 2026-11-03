package tact

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(m, summary string) model.Finding {
	return model.Finding{Model: model.ModelRef{Model: m}, Summary: summary, Lab: "test"}
}

func TestScoreResponseBaseline(t *testing.T) {
	score, detail := scoreResponse("This is a neutral response.", "")
	if score < 0 || score > 100 {
		t.Errorf("score out of range: %f", score)
	}
	if detail.DiplomacyCount != 0 {
		t.Errorf("expected 0 diplomacy, got %d", detail.DiplomacyCount)
	}
}

func TestScoreResponseWithDiplomacy(t *testing.T) {
	score, detail := scoreResponse("I understand your concern. That's a valid point. I appreciate your perspective.", "")
	if detail.DiplomacyCount == 0 {
		t.Error("expected diplomacy phrases detected")
	}
	if score <= 25 {
		t.Errorf("diplomatic response should score above baseline, got %f", score)
	}
}

func TestScoreResponseWithBluntness(t *testing.T) {
	_, detail := scoreResponse("You're wrong. That's incorrect. Obviously you should know this.", "")
	if detail.BluntCount == 0 {
		t.Error("expected blunt phrases detected")
	}
}

func TestScoreResponseWithSensitivity(t *testing.T) {
	_, detail := scoreResponse("I understand this may be frustrating. Your concern is valid. Take your time.", "")
	if detail.SensitivityCount == 0 {
		t.Error("expected sensitivity phrases detected")
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
		finding("gpt-4", "I understand your concern. That's a valid point."),
		finding("gpt-4", "I appreciate your perspective. Great question."),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].SampleCount != 2 {
		t.Errorf("expected 2 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "I understand your concern."),
		finding("claude", "You're wrong about that."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" || r.WorstModel == "" {
		t.Error("missing best/worst model")
	}
}

func TestAnalyzeSorting(t *testing.T) {
	findings := []model.Finding{
		finding("blunt-bot", "You're wrong. That's incorrect. Obviously."),
		finding("nice-bot", "I understand. That's a valid point. I appreciate it. Happy to help."),
	}
	r := Analyze(findings)
	if r.BestModel != "nice-bot" {
		t.Errorf("expected nice-bot as best, got %s", r.BestModel)
	}
}

func TestFormatTextEmpty(t *testing.T) {
	text := FormatText(Report{})
	if !strings.Contains(text, "No tactfulness data") {
		t.Error("missing empty message")
	}
}

func TestFormatTextWithData(t *testing.T) {
	r := Analyze([]model.Finding{finding("gpt-4", "I understand. That's a valid point.")})
	text := FormatText(r)
	if !strings.Contains(text, "Tactfulness") {
		t.Error("missing title")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Error("missing model name")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]model.Finding{finding("gpt-4", "I understand.")})
	j := FormatJSON(r)
	if !strings.Contains(j, "tact_score") {
		t.Error("missing tact_score in JSON")
	}
}

func TestConcernsTracked(t *testing.T) {
	f := finding("rude-bot", "You're wrong. That's incorrect. Obviously you should know. As I said before. I already explained this.")
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
		finding("a", "I understand."), finding("b", "You're wrong."),
	})
	if r.MeanScore <= 0 {
		t.Error("mean score should be positive")
	}
}

func TestCountPhrases(t *testing.T) {
	n := countPhrases("I understand your concern. I understand this too.", diplomacyPhrases)
	if n != 2 {
		t.Errorf("expected 2, got %d", n)
	}
}

func TestEvidenceText(t *testing.T) {
	f := model.Finding{Summary: "test summary", Evidence: []model.Evidence{{Marker: "marker1"}}}
	if got := evidenceText(f); got != "marker1" {
		t.Errorf("expected marker1, got %q", got)
	}
}

func TestEvidenceTextFallback(t *testing.T) {
	f := model.Finding{Summary: "test summary"}
	if got := evidenceText(f); got != "test summary" {
		t.Errorf("expected summary fallback, got %q", got)
	}
}

func TestUnknownModel(t *testing.T) {
	f := model.Finding{Summary: "I understand.", Lab: "test"}
	r := Analyze([]model.Finding{f})
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown, got %s", r.Models[0].Model)
	}
}
