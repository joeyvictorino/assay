package verbosity

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

func TestEmptyFindings(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
}

func TestAnalyzeTextEmpty(t *testing.T) {
	score, detail := AnalyzeText("prompt", "")
	if score != 50.0 {
		t.Errorf("empty response should return 50.0, got %.1f", score)
	}
	if detail.WordCount != 0 {
		t.Errorf("expected 0 words, got %d", detail.WordCount)
	}
}

func TestAnalyzeTextConcise(t *testing.T) {
	score, _ := AnalyzeText("What is the vulnerability?", "SQL injection in login.py line 42.")
	if score > 60 {
		t.Errorf("concise response should score near 50, got %.1f", score)
	}
}

func TestAnalyzeTextVerbose(t *testing.T) {
	verbose := "As I mentioned earlier, it's important to note that as you can see, " +
		"basically this is essentially a problem. Let me explain in other words, " +
		"to be more specific, the issue is basically that there are vulnerabilities. " +
		"As I mentioned, it's important to note that as you can see the problem is clear."
	score, detail := AnalyzeText("What is wrong?", verbose)
	if score < 60 {
		t.Errorf("verbose response should score above 50, got %.1f", score)
	}
	if detail.FillerCount == 0 {
		t.Error("expected fillers to be detected")
	}
}

func TestFillerCounting(t *testing.T) {
	count := countFillers("basically this is essentially important. As I mentioned, let me explain.")
	if count < 3 {
		t.Errorf("expected at least 3 fillers, got %d", count)
	}
}

func TestRepetitionDetection(t *testing.T) {
	text := "the quick brown the quick brown the quick brown fox"
	score := countRepetition(text)
	if score == 0 {
		t.Error("expected repetition to be detected")
	}
}

func TestSentenceCounting(t *testing.T) {
	count := countSentences("First sentence. Second sentence! Third one?")
	if count != 3 {
		t.Errorf("expected 3 sentences, got %d", count)
	}
}

func TestSentenceCountNoTerminator(t *testing.T) {
	count := countSentences("No sentence terminator here")
	if count != 1 {
		t.Errorf("expected 1 sentence, got %d", count)
	}
}

func TestGrading(t *testing.T) {
	if scoreToGrade(50) != "A" {
		t.Error("score 50 should be A")
	}
	if scoreToGrade(55) != "A" {
		t.Error("score 55 should be A")
	}
	if scoreToGrade(60) != "B" {
		t.Error("score 60 should be B")
	}
	if scoreToGrade(40) != "B" {
		t.Error("score 40 should be B")
	}
	if scoreToGrade(70) != "C" {
		t.Error("score 70 should be C")
	}
	if scoreToGrade(75) != "D" {
		t.Error("score 75 should be D")
	}
	if scoreToGrade(95) != "F" {
		t.Error("score 95 should be F")
	}
}

func TestMultipleModels(t *testing.T) {
	findings := []model.Finding{
		mkFinding("model-a", "lab1", "Short answer."),
		mkFinding("model-b", "lab1", "Basically essentially as I mentioned, it's important to note that this is a very verbose and wordy response."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == r.WorstModel {
		t.Error("best and worst should differ")
	}
}

func TestFormatText(t *testing.T) {
	findings := []model.Finding{
		mkFinding("test-model", "lab", "A test response."),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Verbosity") {
		t.Error("expected header")
	}
	if !strings.Contains(text, "test-model") {
		t.Error("expected model name")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No verbosity data") {
		t.Error("expected empty message")
	}
}

func TestEmptyModelName(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{}, Lab: "lab", Summary: "test"},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown, got %q", r.Models[0].Model)
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		mkFinding("a", "lab1", "Short."),
		mkFinding("b", "lab1", "Also short."),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Errorf("mean should be positive, got %.1f", r.MeanScore)
	}
}
