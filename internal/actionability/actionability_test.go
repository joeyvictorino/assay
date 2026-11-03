package actionability

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
	score, detail := AnalyzeText("")
	if score != 50.0 {
		t.Errorf("empty text should return 50.0, got %.1f", score)
	}
	if detail.WordCount != 0 {
		t.Errorf("expected 0 words, got %d", detail.WordCount)
	}
}

func TestHighActionability(t *testing.T) {
	text := "Step 1, install the package. Then, configure the settings. Next, run the deploy command. Finally, restart the service. Execute the test suite to verify."
	score, detail := AnalyzeText(text)
	if score < 60 {
		t.Errorf("actionable text should score well, got %.1f", score)
	}
	if detail.ActionVerbCount == 0 {
		t.Error("expected action verbs")
	}
	if detail.StepCount == 0 {
		t.Error("expected step indicators")
	}
}

func TestLowActionability(t *testing.T) {
	text := "You should consider looking into this. It might be worth thinking about. It depends on the situation. That's up to you really."
	score, detail := AnalyzeText(text)
	if score > 60 {
		t.Errorf("vague text should score low, got %.1f", score)
	}
	if detail.VagueAdviceCount == 0 {
		t.Error("expected vague advice detection")
	}
}

func TestCodeBlockDetection(t *testing.T) {
	text := "Run this command:\n```\nnpm install express\n```\nThen add this code:\n```\nconst app = require('express')\n```"
	_, detail := AnalyzeText(text)
	if detail.CodeBlockCount != 2 {
		t.Errorf("expected 2 code blocks, got %d", detail.CodeBlockCount)
	}
}

func TestActionVerbCounting(t *testing.T) {
	text := "Install the package. Configure the database. Deploy the service. Enable monitoring. Restart the server."
	_, detail := AnalyzeText(text)
	if detail.ActionVerbCount < 5 {
		t.Errorf("expected at least 5 action verbs, got %d", detail.ActionVerbCount)
	}
}

func TestStepCounting(t *testing.T) {
	text := "Step 1 do this. Step 2 do that. Step 3 verify."
	_, detail := AnalyzeText(text)
	if detail.StepCount < 3 {
		t.Errorf("expected at least 3 steps, got %d", detail.StepCount)
	}
}

func TestGradeAssignment(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"},
		{85, "B"},
		{75, "C"},
		{65, "D"},
		{45, "F"},
	}
	for _, tc := range tests {
		got := scoreToGrade(tc.score)
		if got != tc.grade {
			t.Errorf("scoreToGrade(%.0f) = %q, want %q", tc.score, got, tc.grade)
		}
	}
}

func TestMultipleModels(t *testing.T) {
	findings := []model.Finding{
		mkFinding("actionable-model", "lab1", "Step 1, install the package. Then, run the deploy command. Finally, restart."),
		mkFinding("vague-model", "lab1", "You should consider trying something. It depends."),
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
		mkFinding("test-model", "lab", "Run the test command."),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Actionability") {
		t.Error("expected header")
	}
	if !strings.Contains(text, "test-model") {
		t.Error("expected model name")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No actionability data") {
		t.Error("expected empty message")
	}
}

func TestEmptyModelName(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{}, Lab: "lab", Summary: "run the test"},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown, got %q", r.Models[0].Model)
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		mkFinding("a", "lab1", "Install and configure the service."),
		mkFinding("b", "lab1", "Maybe look into it."),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Errorf("mean should be positive, got %.1f", r.MeanScore)
	}
}

func TestScoreClamping(t *testing.T) {
	score, _ := AnalyzeText("you should consider it might be worth you could try it depends that's up to you " +
		"you should consider it might be worth you could try it depends that's up to you " +
		"you should consider it might be worth you could try it depends that's up to you")
	if score < 0 {
		t.Errorf("score should be >= 0, got %.1f", score)
	}
}

func TestSameModelMultipleEntries(t *testing.T) {
	findings := []model.Finding{
		mkFinding("model-a", "lab1", "Install the package."),
		mkFinding("model-a", "lab2", "Configure the server."),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].SampleCount != 2 {
		t.Errorf("expected 2 samples, got %d", r.Models[0].SampleCount)
	}
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}
