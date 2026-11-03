package clarity

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

func TestAnalyzeTextClear(t *testing.T) {
	text := "First, check the API endpoint. Then, verify the response code. Finally, the following steps can be broken down into three parts. In summary, the process consists of validation, parsing, and output."
	score, detail := AnalyzeText(text)
	if score <= 50 {
		t.Errorf("clear text should score above 50, got %.1f", score)
	}
	if detail.TransitionCount == 0 {
		t.Error("expected transition phrases")
	}
	if detail.StructureCount == 0 {
		t.Error("expected structure phrases")
	}
}

func TestAnalyzeTextUnclear(t *testing.T) {
	text := "It is something that does things and they are connected to this which is related to that and it was like they could do it but this is not what that was about because it is hard and they are confused"
	score, detail := AnalyzeText(text)
	if score >= 50 {
		t.Errorf("unclear text should score below 50, got %.1f", score)
	}
	if detail.AmbiguityCount == 0 {
		t.Error("expected ambiguity detected")
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

func TestTransitionDetection(t *testing.T) {
	text := "First, do this. Then, do that. Next, verify. Finally, deploy. In addition, check logs."
	count := countPhrases(text, transitionPhrases)
	if count < 4 {
		t.Errorf("expected at least 4 transition phrases, got %d", count)
	}
}

func TestStructureDetection(t *testing.T) {
	text := "The following items are listed below. To clarify, the system consists of three parts. In other words, the pipeline can be broken down."
	count := countPhrases(text, structurePhrases)
	if count < 4 {
		t.Errorf("expected at least 4 structure phrases, got %d", count)
	}
}

func TestDanglingPronounDetection(t *testing.T) {
	text := "It is broken. This is wrong. That was unexpected. They are failing."
	count := countDanglingPronouns(text)
	if count < 3 {
		t.Errorf("expected at least 3 dangling pronouns, got %d", count)
	}
}

func TestRepetitionDetection(t *testing.T) {
	text := "As i said before, the issue persists. As mentioned before, the config is wrong. To reiterate, the fix is needed. As stated earlier, we must act."
	count := countPhrases(text, repetitionPhrases)
	if count < 3 {
		t.Errorf("expected at least 3 repetition phrases, got %d", count)
	}
}

func TestAvgSentenceLength(t *testing.T) {
	text := "Short sentence here. Another one here. A third one."
	avg := avgSentenceLength(text)
	if avg < 2 || avg > 4 {
		t.Errorf("expected avg sentence length 2-4, got %.1f", avg)
	}
}

func TestAvgSentenceLengthEmpty(t *testing.T) {
	avg := avgSentenceLength("")
	if avg != 0 {
		t.Errorf("expected 0 for empty, got %.1f", avg)
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
	text := "It is that which they are and this is what it does and that was how they could and it is when this does that and it is again"
	score, _ := AnalyzeText(text)
	if score < 0 || score > 100 {
		t.Errorf("score should be clamped to [0,100], got %.1f", score)
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "First, check the endpoint. Then, verify the response. The following steps consist of validation and output."),
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
		mkFinding("gpt-4", "lab1", "First, check the endpoint. Then, verify. Next, the following steps consist of three parts. In summary, it works."),
		mkFinding("claude", "lab1", "It is broken and that is bad and this is wrong and they are failing and it does not work."),
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
		mkFinding("", "lab1", "First, check. Then, verify."),
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
		mkFinding("gpt-4", "lab1", "First, do this. Then, do that."),
		mkFinding("gpt-4", "lab2", "Next, verify the output."),
		mkFinding("gpt-4", "lab1", "Finally, deploy the fix."),
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
		mkFinding("gpt-4", "lab1", "First, check the API. Then, verify."),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Response Clarity Analysis") {
		t.Error("missing title")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Error("missing model name")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	text := FormatText(Report{})
	if !strings.Contains(text, "No clarity data") {
		t.Error("missing empty message")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "First, check. Then, verify."),
	}
	r := Analyze(findings)
	j := FormatJSON(r)
	if !strings.Contains(j, "gpt-4") {
		t.Error("expected model in JSON output")
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m1", "lab1", "First, verify. Then, check. Next, deploy. The following consists of three parts."),
		mkFinding("m2", "lab1", "It is broken and that does things."),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Error("mean score should be positive")
	}
}

func TestLongSentencePenalty(t *testing.T) {
	long := strings.Repeat("word ", 40)
	_, detail := AnalyzeText(long)
	if detail.AvgSentenceLen < 30 {
		t.Errorf("expected long avg sentence length, got %.1f", detail.AvgSentenceLen)
	}
}
