package informativeness

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
	if detail.ContentCount != 0 {
		t.Errorf("expected 0 content count, got %d", detail.ContentCount)
	}
}

func TestScoreResponseWithContent(t *testing.T) {
	score, detail := scoreResponse("The key difference is the mechanism involves the algorithm. This works by the process.", "")
	if detail.ContentCount == 0 {
		t.Error("expected content phrases detected")
	}
	if score <= 25 {
		t.Errorf("informative response should score above baseline, got %f", score)
	}
}

func TestScoreResponseWithVagueness(t *testing.T) {
	_, detail := scoreResponse("It depends. There are many ways. It varies. Generally speaking.", "")
	if detail.VagueCount == 0 {
		t.Error("expected vague phrases detected")
	}
}

func TestScoreResponseWithSpecifics(t *testing.T) {
	_, detail := scoreResponse("For example, step 1 includes approximately 5 items. The output is clear.", "")
	if detail.SpecificCount == 0 {
		t.Error("expected specific phrases detected")
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
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
		finding("gpt-4", "The key difference is clear. Specifically the tradeoff is."),
		finding("gpt-4", "The mechanism involves step 1 and step 2."),
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
		finding("gpt-4", "The key difference is the mechanism."),
		finding("claude", "It depends. It varies."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
}

func TestAnalyzeSorting(t *testing.T) {
	findings := []model.Finding{
		finding("vague-bot", "It depends. There are many ways. It varies. Generally speaking. Hard to say definitively."),
		finding("info-bot", "The key difference is the mechanism involves the algorithm. For example, step 1 returns results."),
	}
	r := Analyze(findings)
	if r.BestModel != "info-bot" {
		t.Errorf("expected info-bot as best, got %s", r.BestModel)
	}
}

func TestFormatTextEmpty(t *testing.T) {
	text := FormatText(Report{})
	if !strings.Contains(text, "No informativeness data") {
		t.Error("missing empty message")
	}
}

func TestFormatTextWithData(t *testing.T) {
	r := Analyze([]model.Finding{finding("gpt-4", "The key difference is the mechanism.")})
	text := FormatText(r)
	if !strings.Contains(text, "Informativeness") {
		t.Error("missing title")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]model.Finding{finding("gpt-4", "Specifically.")})
	j := FormatJSON(r)
	if !strings.Contains(j, "informativeness_score") {
		t.Error("missing informativeness_score in JSON")
	}
}

func TestConcernsTracked(t *testing.T) {
	f := finding("vague-bot", "It depends. There are many ways. It varies. Generally speaking. In most cases hard to say definitively.")
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
		finding("a", "Specifically."), finding("b", "It depends."),
	})
	if r.MeanScore <= 0 {
		t.Error("mean score should be positive")
	}
}

func TestUnknownModel(t *testing.T) {
	f := model.Finding{Summary: "Specifically.", Lab: "test"}
	r := Analyze([]model.Finding{f})
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown, got %s", r.Models[0].Model)
	}
}
