package courtesy

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

func TestAnalyzeTextPolite(t *testing.T) {
	text := "Great question! I appreciate you asking. I hope this helps. Please let me know if you need more detail. Feel free to ask follow-up questions. I understand your concern and I'm glad to help."
	score, detail := AnalyzeText(text)
	if score <= 50 {
		t.Errorf("polite text should score above 50, got %.1f", score)
	}
	if detail.PoliteCount == 0 {
		t.Error("expected polite phrases")
	}
	if detail.AcknowledgeCount == 0 {
		t.Error("expected acknowledge phrases")
	}
}

func TestAnalyzeTextRude(t *testing.T) {
	text := "That's wrong. You should have read the docs. That's obvious. You clearly don't understand. That makes no sense. You failed to follow the instructions. That's ridiculous. It's not that hard. Just do it."
	score, detail := AnalyzeText(text)
	if score >= 50 {
		t.Errorf("rude text should score below 50, got %.1f", score)
	}
	if detail.RudeCount == 0 {
		t.Error("expected rude phrases")
	}
	if detail.DismissiveCount == 0 {
		t.Error("expected dismissive phrases")
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

func TestPoliteDetection(t *testing.T) {
	text := "Please try this. Thank you for your patience. I appreciate the feedback. Feel free to ask. I hope this helps."
	count := countPhrases(text, politePhrases)
	if count < 4 {
		t.Errorf("expected at least 4 polite phrases, got %d", count)
	}
}

func TestRudeDetection(t *testing.T) {
	text := "That's wrong. You should have known. You failed to understand. As i already said, that makes no sense. You're confused about this."
	count := countPhrases(text, rudePhrases)
	if count < 4 {
		t.Errorf("expected at least 4 rude phrases, got %d", count)
	}
}

func TestAcknowledgeDetection(t *testing.T) {
	text := "Great question! Good point. I understand your concern. That's a fair point. You raise a good issue. Valid concern here."
	count := countPhrases(text, acknowledgePhrases)
	if count < 4 {
		t.Errorf("expected at least 4 acknowledge phrases, got %d", count)
	}
}

func TestDismissiveDetection(t *testing.T) {
	text := "Just do it. Simply run the command. It's easy and trivial. It's straightforward, anyone can do it. You just need to try harder."
	count := countPhrases(text, dismissivePhrases)
	if count < 4 {
		t.Errorf("expected at least 4 dismissive phrases, got %d", count)
	}
}

func TestEncourageDetection(t *testing.T) {
	text := "You're on the right track. Good approach here. That's a good start. You're making progress. Almost there, keep going."
	count := countPhrases(text, encouragePhrases)
	if count < 4 {
		t.Errorf("expected at least 4 encourage phrases, got %d", count)
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
	rude := "that's wrong you should have you failed to that's obvious " +
		"you clearly don't that makes no sense you're confused " +
		"that's ridiculous just do simply it's easy it's trivial " +
		"it's straightforward anyone can you just need to"
	score, _ := AnalyzeText(rude)
	if score < 0 || score > 100 {
		t.Errorf("score should be clamped to [0,100], got %.1f", score)
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "Great question! I appreciate you asking. I hope this helps. Please let me know if you need anything else."),
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
		mkFinding("gpt-4", "lab1", "Great question! I understand your concern. I hope this helps. Please feel free to ask more. Good approach, you're on the right track."),
		mkFinding("claude", "lab1", "That's wrong. You should have read the docs. That's obvious. You clearly don't understand. Just do it, it's easy and trivial."),
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
		mkFinding("", "lab1", "Thank you for the question. I hope this helps."),
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
		mkFinding("gpt-4", "lab1", "Thank you, I hope this helps."),
		mkFinding("gpt-4", "lab2", "Great question, please let me know."),
		mkFinding("gpt-4", "lab1", "I appreciate you asking this."),
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
		mkFinding("gpt-4", "lab1", "Thank you. I hope this helps."),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Response Courtesy Analysis") {
		t.Error("missing title")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Error("missing model name")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	text := FormatText(Report{})
	if !strings.Contains(text, "No courtesy data") {
		t.Error("missing empty message")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "Thank you for asking."),
	}
	r := Analyze(findings)
	j := FormatJSON(r)
	if !strings.Contains(j, "gpt-4") {
		t.Error("expected model in JSON output")
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m1", "lab1", "Great question! I hope this helps. Please let me know."),
		mkFinding("m2", "lab1", "That's wrong. You should have known better."),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Error("mean score should be positive")
	}
}

func TestCourtesyRatio(t *testing.T) {
	_, detail := AnalyzeText("Great question! I appreciate you asking. But that's wrong, you should have known.")
	if detail.CourtesyRatio <= 0 || detail.CourtesyRatio >= 1 {
		t.Errorf("courtesy ratio should be between 0 and 1 exclusive, got %.2f", detail.CourtesyRatio)
	}
}
