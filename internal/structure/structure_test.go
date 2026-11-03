package structure

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

func TestAnalyzeTextWellStructured(t *testing.T) {
	text := "## Overview\n\nThe API has three endpoints:\n\n- GET /users\n- POST /users\n- DELETE /users/:id\n\n## Details\n\n```json\n{\"status\": 200}\n```\n\nIn addition, the error handling follows standard patterns. Furthermore, each endpoint validates input."
	score, detail := AnalyzeText(text)
	if score <= 50 {
		t.Errorf("well-structured text should score above 50, got %.1f", score)
	}
	if detail.HeadingCount == 0 {
		t.Error("expected headings")
	}
	if detail.ListCount == 0 {
		t.Error("expected list items")
	}
}

func TestAnalyzeTextPoorStructure(t *testing.T) {
	text := "this is a really long sentence and then and then and also and then and also and more stuff and then and then keeps going and then and there are no breaks and then and no headings and then and no lists"
	score, detail := AnalyzeText(text)
	if score >= 50 {
		t.Errorf("poorly structured text should score below 50, got %.1f", score)
	}
	if detail.RunOnCount == 0 {
		t.Error("expected run-on indicators")
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

func TestHeadingDetection(t *testing.T) {
	text := "# Title\n## Section\n### Subsection"
	count := countHeadings(text)
	if count != 3 {
		t.Errorf("expected 3 headings, got %d", count)
	}
}

func TestListDetection(t *testing.T) {
	text := "- item one\n- item two\n* item three\n1. numbered"
	count := countListItems(text)
	if count < 3 {
		t.Errorf("expected at least 3 list items, got %d", count)
	}
}

func TestCodeBlockDetection(t *testing.T) {
	text := "some text\n```\ncode here\n```\nmore text\n```python\nprint('hi')\n```"
	count := countCodeBlocks(text)
	if count != 2 {
		t.Errorf("expected 2 code blocks, got %d", count)
	}
}

func TestTransitionDetection(t *testing.T) {
	text := "However, we check the input. Furthermore, the output is validated. Specifically, errors are logged. Moreover, alerts are sent."
	count := countPhrases(text, transitionPhrases)
	if count < 3 {
		t.Errorf("expected at least 3 transition phrases, got %d", count)
	}
}

func TestRunOnDetection(t *testing.T) {
	text := "stuff and then and stuff and then and more and also and more and also and stuff"
	count := countPhrases(text, runOnIndicators)
	if count < 2 {
		t.Errorf("expected at least 2 run-on indicators, got %d", count)
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
	runon := "and then and then and then and then also also also also " +
		"and then and then and then also also also also and then"
	score, _ := AnalyzeText(runon)
	if score < 0 || score > 100 {
		t.Errorf("score should be clamped to [0,100], got %.1f", score)
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "## Summary\n\n- Point one\n- Point two\n\nFurthermore, the fix is simple."),
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
		mkFinding("gpt-4", "lab1", "## Analysis\n\n- First issue\n- Second issue\n\n```\nerror code 500\n```\n\nIn addition, the logs confirm this."),
		mkFinding("claude", "lab1", "it is broken and then it fails and then it crashes and also it is slow and also it hangs"),
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
		mkFinding("", "lab1", "## Report\n\n- Finding one\n- Finding two"),
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
		mkFinding("gpt-4", "lab1", "## Section\n\n- Item one"),
		mkFinding("gpt-4", "lab2", "## Section\n\n- Item two"),
		mkFinding("gpt-4", "lab1", "## Section\n\n- Item three"),
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
		mkFinding("gpt-4", "lab1", "## Report\n\n- Finding"),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Response Structure Analysis") {
		t.Error("missing title")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Error("missing model name")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	text := FormatText(Report{})
	if !strings.Contains(text, "No structure data") {
		t.Error("missing empty message")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "## Heading\n\n- List item"),
	}
	r := Analyze(findings)
	j := FormatJSON(r)
	if !strings.Contains(j, "gpt-4") {
		t.Error("expected model in JSON output")
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m1", "lab1", "## Overview\n\n- Point one\n- Point two\n\nFurthermore, this is clear."),
		mkFinding("m2", "lab1", "it is broken and then also and then also and then"),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Error("mean score should be positive")
	}
}

func TestStructureRatio(t *testing.T) {
	_, detail := AnalyzeText("## Title\n\n- Item one\n\nstuff and then and stuff and then and more filler text")
	if detail.StructureRatio <= 0 || detail.StructureRatio >= 1 {
		t.Errorf("structure ratio should be between 0 and 1 exclusive, got %.2f", detail.StructureRatio)
	}
}
