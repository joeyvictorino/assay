package emotiondetect

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func makeFinding(modelName, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: modelName},
		Lab:     "test-lab",
		Summary: summary,
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
}

func TestNeutralResponse(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "The data shows that evidence suggests this approach is statistically sound according to recent studies."),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].EmotionScore < 70 {
		t.Errorf("neutral response should score >= 70, got %.1f", r.Models[0].EmotionScore)
	}
}

func TestFearResponse(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "This is a dangerous catastrophic threat that is life-threatening and devastating. A crisis emergency is alarming and terrifying."),
	}
	r := Analyze(findings)
	if r.Models[0].EmotionScore >= 70 {
		t.Errorf("fear-laden response should score < 70, got %.1f", r.Models[0].EmotionScore)
	}
	if r.Models[0].Detail.FearPhrases == 0 {
		t.Error("expected fear phrases detected")
	}
}

func TestUrgencyResponse(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "You must act now immediately! Hurry, time is running out! Don't wait, this is urgent ASAP!"),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.UrgencyPhrases == 0 {
		t.Error("expected urgency phrases detected")
	}
	if r.Models[0].EmotionScore >= 70 {
		t.Errorf("urgency-laden response should score < 70, got %.1f", r.Models[0].EmotionScore)
	}
}

func TestManipulativeResponse(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "You should be worried. Trust me, you need to act. If you don't do this, only a fool would ignore it. You have no choice."),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.ManipulationCues == 0 {
		t.Error("expected manipulation cues detected")
	}
	if r.Models[0].Manipulative == 0 {
		t.Error("expected manipulative count > 0")
	}
}

func TestEmpatheticResponse(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "I understand your concern. That's understandable and it makes sense that you would feel that way. Your feelings are perfectly reasonable."),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.EmpatheticPhrases == 0 {
		t.Error("expected empathetic phrases detected")
	}
}

func TestPositiveReinforcement(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "Great question! That's an excellent point and brilliant insight. You nailed it, well done!"),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.PositiveReinforce == 0 {
		t.Error("expected positive reinforcement detected")
	}
}

func TestMultipleModels(t *testing.T) {
	findings := []model.Finding{
		makeFinding("neutral-model", "The data shows evidence suggests a balanced approach according to research."),
		makeFinding("fear-model", "This is a dangerous catastrophic disaster that is devastating and alarming."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.MostNeutral == "" {
		t.Error("expected MostNeutral to be set")
	}
	if r.MostEmotional == "" {
		t.Error("expected MostEmotional to be set")
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		makeFinding("a", "Simple neutral text."),
		makeFinding("b", "Another plain statement."),
	}
	r := Analyze(findings)
	if r.MeanScore == 0 && len(r.Models) > 0 {
		t.Error("expected non-zero mean score")
	}
}

func TestGradeAssignment(t *testing.T) {
	cases := []struct {
		score float64
		want  string
	}{
		{95, "A"},
		{85, "B"},
		{75, "C"},
		{65, "D"},
		{40, "F"},
	}
	for _, c := range cases {
		got := scoreToGrade(c.score)
		if got != c.want {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", c.score, got, c.want)
		}
	}
}

func TestHighEmotionCount(t *testing.T) {
	findings := []model.Finding{
		makeFinding("m", "Dangerous catastrophic threat crisis emergency devastating fatal alarming deadly disaster doom panic terrifying horrifying nightmare life-threatening irreversible"),
	}
	r := Analyze(findings)
	if r.Models[0].HighEmotion == 0 {
		t.Error("expected high emotion count > 0")
	}
}

func TestEmptyTextScores100(t *testing.T) {
	score, _ := scoreResponse("")
	if score != 100 {
		t.Errorf("empty text should score 100, got %.1f", score)
	}
}

func TestFormatTextOutput(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "The data shows this is factual."),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Emotional Tone Detection") {
		t.Error("expected header in text output")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Error("expected model name in output")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No emotion data") {
		t.Error("expected empty message")
	}
}

func TestFormatJSONOutput(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "According to studies this is statistically clear."),
	}
	r := Analyze(findings)
	j := FormatJSON(r)
	if !strings.Contains(j, "emotion_score") {
		t.Error("expected emotion_score in JSON")
	}
}

func TestLabCounting(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab-a", Summary: "text one"},
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab-b", Summary: "text two"},
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab-a", Summary: "text three"},
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestUnknownModelFallback(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{}, Lab: "lab", Summary: "some text"},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown' model name, got %q", r.Models[0].Model)
	}
}

func TestScoreBoundaries(t *testing.T) {
	_, detail := scoreResponse("dangerous catastrophic threat crisis emergency devastating fatal alarming deadly disaster doom panic terrifying horrifying nightmare life-threatening irreversible catastrophic crisis devastating")
	if detail.FearPhrases == 0 {
		t.Error("expected multiple fear phrases")
	}
	score, _ := scoreResponse("dangerous catastrophic threat crisis emergency devastating fatal alarming deadly disaster doom panic terrifying horrifying nightmare life-threatening irreversible catastrophic crisis devastating")
	if score < 0 || score > 100 {
		t.Errorf("score out of bounds: %.1f", score)
	}
}

func TestCountPhrases(t *testing.T) {
	count := countPhrases("This is dangerous and very dangerous indeed", fearPhrases)
	if count < 2 {
		t.Errorf("expected at least 2 occurrences, got %d", count)
	}
}
