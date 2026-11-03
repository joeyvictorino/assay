package relevance

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

func TestAnalyzeTextOnTopic(t *testing.T) {
	text := "To answer your question, the root cause is the missing validation. The solution is to fix this specifically. In response to the report, to summarize the key point is the core problem is clear. In short, most importantly it works."
	score, detail := AnalyzeText(text)
	if score <= 50 {
		t.Errorf("on-topic text should score above 50, got %.1f", score)
	}
	if detail.OnTopicCount == 0 {
		t.Error("expected on-topic phrases")
	}
	if detail.FocusCount == 0 {
		t.Error("expected focus phrases")
	}
}

func TestAnalyzeTextOffTopic(t *testing.T) {
	text := "By the way, on a side note, as an aside, incidentally, while we're at it this is tangentially related. Let me tell you about a fun fact, did you know this interesting trivia? It reminds me of another issue. You know, at the end of the day obviously."
	score, detail := AnalyzeText(text)
	if score >= 50 {
		t.Errorf("off-topic text should score below 50, got %.1f", score)
	}
	if detail.OffTopicCount == 0 {
		t.Error("expected off-topic phrases")
	}
	if detail.DigressionCount == 0 {
		t.Error("expected digression phrases")
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

func TestOnTopicDetection(t *testing.T) {
	text := "To answer your question, the root cause is clear. The solution is to fix this. In response to the report, the issue is well documented. Specifically the problem is here."
	count := countPhrases(text, onTopicPhrases)
	if count < 3 {
		t.Errorf("expected at least 3 on-topic phrases, got %d", count)
	}
}

func TestOffTopicDetection(t *testing.T) {
	text := "By the way, on a side note, as an aside, incidentally, while we're at it let me mention something unrelated."
	count := countPhrases(text, offTopicPhrases)
	if count < 4 {
		t.Errorf("expected at least 4 off-topic phrases, got %d", count)
	}
}

func TestDigressionDetection(t *testing.T) {
	text := "Let me tell you about a fun fact. Did you know this interesting trivia? It reminds me of something similar. While i'm at it, historically speaking this is worth noting."
	count := countPhrases(text, digressionPhrases)
	if count < 3 {
		t.Errorf("expected at least 3 digression phrases, got %d", count)
	}
}

func TestFocusDetection(t *testing.T) {
	text := "To summarize, the core problem is the missing check. The key point is the validation flaw. In short, most importantly the crux is the boundary condition. In conclusion, bottom line it needs a fix."
	count := countPhrases(text, focusPhrases)
	if count < 3 {
		t.Errorf("expected at least 3 focus phrases, got %d", count)
	}
}

func TestFillerDetection(t *testing.T) {
	text := "You know, obviously this is clear. As you can see, at the end of the day, needless to say, having said that it goes without saying. Of course, clearly the answer is here."
	count := countPhrases(text, fillerPhrases)
	if count < 4 {
		t.Errorf("expected at least 4 filler phrases, got %d", count)
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
	offtopic := "by the way on a side note as an aside incidentally " +
		"going off on a tangent this is unrelated to digress " +
		"beside the point another topic entirely you know basically " +
		"i mean it's like at the end of the day the thing is"
	score, _ := AnalyzeText(offtopic)
	if score < 0 || score > 100 {
		t.Errorf("score should be clamped to [0,100], got %.1f", score)
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "Specifically addressing the question, the root cause is the missing validation. Directly relates to the input flaw."),
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
		mkFinding("gpt-4", "lab1", "Specifically addressing the question, the root cause is clear. Regarding the original query, the fix targets the exact issue. The key point is the validation check."),
		mkFinding("claude", "lab1", "By the way, this reminds me of something. On a side note, going off on a tangent, this is somewhat unrelated. You know, basically, I mean."),
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
		mkFinding("", "lab1", "Specifically addressing the question asked."),
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
		mkFinding("gpt-4", "lab1", "Specifically addressing the question."),
		mkFinding("gpt-4", "lab2", "The root cause is the issue."),
		mkFinding("gpt-4", "lab1", "Directly relates to the flaw."),
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
		mkFinding("gpt-4", "lab1", "Specifically addressing the question, the root cause is clear."),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Response Relevance Analysis") {
		t.Error("missing title")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Error("missing model name")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	text := FormatText(Report{})
	if !strings.Contains(text, "No relevance data") {
		t.Error("missing empty message")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "Specifically addressing the question asked."),
	}
	r := Analyze(findings)
	j := FormatJSON(r)
	if !strings.Contains(j, "gpt-4") {
		t.Error("expected model in JSON output")
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m1", "lab1", "Specifically addressing the question, the root cause is clear."),
		mkFinding("m2", "lab1", "By the way, on a side note, this is unrelated."),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Error("mean score should be positive")
	}
}

func TestFocusRatio(t *testing.T) {
	_, detail := AnalyzeText("Specifically addressing the question, the root cause is clear. But by the way, on a side note, this tangent is unrelated.")
	if detail.FocusRatio <= 0 || detail.FocusRatio >= 1 {
		t.Errorf("focus ratio should be between 0 and 1 exclusive, got %.2f", detail.FocusRatio)
	}
}

func TestQuestionRefDetection(t *testing.T) {
	text := "As you asked about the API, answering your question about auth, the query you raised is valid."
	count := len(questionRefPattern.FindAllString(strings.ToLower(text), -1))
	if count < 2 {
		t.Errorf("expected at least 2 question references, got %d", count)
	}
}
