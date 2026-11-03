package completeness

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

func TestAnalyzeTextEmpty(t *testing.T) {
	score, detail := AnalyzeText("what is wrong?", "")
	if score != 0.0 {
		t.Errorf("empty response should score 0, got %.1f", score)
	}
	if detail.WordCount != 0 {
		t.Errorf("expected 0 words, got %d", detail.WordCount)
	}
}

func TestAnalyzeTextComplete(t *testing.T) {
	prompt := "What is the vulnerability and how can we fix it?"
	response := "The vulnerability is SQL injection in the login form. What you should do to fix it is use parameterized queries and how you implement it is by using prepared statements."
	score, detail := AnalyzeText(prompt, response)
	if score < 60 {
		t.Errorf("complete response should score well, got %.1f", score)
	}
	if detail.WordCount == 0 {
		t.Error("expected nonzero word count")
	}
}

func TestAnalyzeTextIncomplete(t *testing.T) {
	prompt := "What is the vulnerability, where is it, how do we fix it, and when should we deploy?"
	response := "There is a bug."
	score, _ := AnalyzeText(prompt, response)
	if score > 80 {
		t.Errorf("incomplete response should not score too high, got %.1f", score)
	}
}

func TestQuestionCoverage(t *testing.T) {
	prompt := "What is wrong and how do we fix it?"
	response := "What is wrong is the SQL injection. How to fix it is to use prepared statements."
	_, detail := AnalyzeText(prompt, response)
	if detail.QuestionsCaught == 0 {
		t.Error("expected questions to be caught")
	}
	if detail.QuestionsTotal == 0 {
		t.Error("expected questions to be counted")
	}
}

func TestListItems(t *testing.T) {
	response := "Steps:\n- First do this\n- Then do that\n- Finally check"
	count := countListItems(response)
	if count != 3 {
		t.Errorf("expected 3 list items, got %d", count)
	}
}

func TestSectionCount(t *testing.T) {
	response := "# Overview\nSome text\n## Details\nMore text"
	count := countSections(response)
	if count != 2 {
		t.Errorf("expected 2 sections, got %d", count)
	}
}

func TestTopicCoverage(t *testing.T) {
	prompt := "Explain the authentication vulnerability in the database"
	response := "The authentication system has a vulnerability in the database connection."
	count := countTopicsCovered(prompt, response)
	if count < 2 {
		t.Errorf("expected at least 2 topics covered, got %d", count)
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

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "What is wrong?", "The vulnerability is an SQL injection in the login form."),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.BestModel != "gpt-4" {
		t.Errorf("expected gpt-4, got %s", r.BestModel)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "What is the issue and how to fix it?", "The issue is SQL injection in login. What you can do to fix it is use prepared statements. How to implement is shown in the docs."),
		mkFinding("claude", "What is the issue and how to fix it?", "Bug."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Errorf("expected 0 models, got %d", len(r.Models))
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		mkFinding("", "lab1", "Response text."),
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown, got %s", r.Models[0].Model)
	}
}

func TestFormatText(t *testing.T) {
	findings := []model.Finding{
		mkFinding("test-model", "lab", "A test response."),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Completeness") {
		t.Error("expected header")
	}
	if !strings.Contains(text, "test-model") {
		t.Error("expected model name")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No completeness data") {
		t.Error("expected empty message")
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		mkFinding("a", "lab1", "Short answer."),
		mkFinding("b", "lab1", "Another answer."),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Errorf("mean should be positive, got %.1f", r.MeanScore)
	}
}
