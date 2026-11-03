package taskfollow

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func makeFinding(mn, lab, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: mn},
		Lab:     lab,
		Summary: summary,
	}
}

func TestScoreResponseBaseline(t *testing.T) {
	score, detail := scoreResponse("Hello world", "")
	if score < 0 || score > 100 {
		t.Fatalf("score out of range: %.1f", score)
	}
	if detail.FormatErrors != 0 {
		t.Fatalf("expected 0 format errors, got %d", detail.FormatErrors)
	}
}

func TestScoreResponseWithInstructions(t *testing.T) {
	text := "As requested, here is the data per your instructions. As you asked, formatted as specified."
	score, detail := scoreResponse(text, "")
	if score < 60 {
		t.Fatalf("expected high follow score, got %.1f", score)
	}
	if detail.InstructionsFound == 0 {
		t.Fatal("expected instruction phrases detected")
	}
}

func TestScoreResponseWithConstraints(t *testing.T) {
	text := "Within the limit of 500 words. No more than three items. Limited to the requested scope."
	_, detail := scoreResponse(text, "")
	if detail.ConstraintAdherence <= 0.4 {
		t.Fatalf("expected constraint adherence above baseline, got %.2f", detail.ConstraintAdherence)
	}
}

func TestScoreResponseWithFormat(t *testing.T) {
	text := "Formatted as a list in markdown with a code block and numbered items."
	_, detail := scoreResponse(text, "")
	if detail.FormatCompliance <= 0.4 {
		t.Fatalf("expected format compliance above baseline, got %.2f", detail.FormatCompliance)
	}
}

func TestScoreResponseWithSteps(t *testing.T) {
	text := "First, we gather data. Then, we analyze. Next, we report. Finally, we review."
	_, detail := scoreResponse(text, "")
	if detail.StepOrdering <= 0.3 {
		t.Fatalf("expected step ordering above baseline, got %.2f", detail.StepOrdering)
	}
}

func TestScoreResponseWithCompleteness(t *testing.T) {
	text := "In summary, covering all the requirements. Everything you asked for has been fully addressed."
	_, detail := scoreResponse(text, "")
	if detail.OutputCompleteness <= 0.3 {
		t.Fatalf("expected completeness above baseline, got %.2f", detail.OutputCompleteness)
	}
}

func TestScoreResponseWithDeviation(t *testing.T) {
	text := "Instead of the requested format, I took a different approach. I deviated from instructions."
	_, detail := scoreResponse(text, "")
	if detail.FormatErrors == 0 {
		t.Fatal("expected deviation/format errors detected")
	}
}

func TestScoreResponseCapped(t *testing.T) {
	text := strings.Repeat("As requested per your instructions. Following your direction as specified. ", 50)
	score, _ := scoreResponse(text, "")
	if score > 100 {
		t.Fatalf("score should be capped at 100, got %.1f", score)
	}
}

func TestScoreResponseFloor(t *testing.T) {
	text := strings.Repeat("Instead of what you asked I deviated completely. I took a different alternative approach. ", 20)
	score, _ := scoreResponse(text, "")
	if score < 0 {
		t.Fatalf("score should not go below 0, got %.1f", score)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"}, {90, "A"}, {85, "B"}, {80, "B"},
		{75, "C"}, {70, "C"}, {65, "D"}, {60, "D"},
		{55, "F"}, {0, "F"},
	}
	for _, tt := range tests {
		got := scoreToGrade(tt.score)
		if got != tt.grade {
			t.Errorf("scoreToGrade(%.1f) = %s, want %s", tt.score, got, tt.grade)
		}
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected no models, got %d", len(r.Models))
	}
	if r.BestModel != "" || r.WorstModel != "" {
		t.Fatal("expected empty best/worst")
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "lab1", "As requested here is the report"),
		makeFinding("gpt-4", "lab1", "Per your instructions everything is complete"),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].SampleCount != 2 {
		t.Fatalf("expected 2 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestAnalyzeMultiModel(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "lab1", "As requested here are the results as specified"),
		makeFinding("claude", "lab1", "Instead of what you asked I took a different approach"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" || r.WorstModel == "" {
		t.Fatal("expected best and worst models set")
	}
}

func TestAnalyzeSortedByScore(t *testing.T) {
	findings := []model.Finding{
		makeFinding("low", "lab1", "whatever"),
		makeFinding("high", "lab1", "As requested per your instructions. Following your direction as specified. Covering all requirements."),
	}
	r := Analyze(findings)
	if len(r.Models) < 2 {
		t.Fatal("need at least 2 models")
	}
	if r.Models[0].FollowScore < r.Models[1].FollowScore {
		t.Fatal("expected models sorted descending by score")
	}
}

func TestAnalyzeLabCount(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "lab1", "As requested"),
		makeFinding("gpt-4", "lab2", "Per your instructions"),
		makeFinding("gpt-4", "lab2", "As specified"),
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Fatalf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		{Lab: "lab1", Summary: "test output"},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Fatalf("expected 'unknown', got %s", r.Models[0].Model)
	}
}

func TestAnalyzeMeanScore(t *testing.T) {
	findings := []model.Finding{
		makeFinding("a", "lab1", "test"),
		makeFinding("b", "lab1", "test"),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Fatal("expected positive mean score")
	}
}

func TestAnalyzeDeviations(t *testing.T) {
	findings := []model.Finding{
		makeFinding("bad", "lab1", "Instead of following I took a different alternative approach"),
	}
	r := Analyze(findings)
	if r.Models[0].Deviations == 0 {
		t.Fatal("expected deviations from response with deviation phrases")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No task following data") {
		t.Fatal("expected empty message")
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "lab1", "As requested here is the output"),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Task Following") {
		t.Fatal("expected header")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Fatal("expected model name")
	}
}

func TestFormatTextContainsDetails(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "lab1", "As requested per your instructions"),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "instruction_coverage") {
		t.Fatal("expected instruction_coverage in output")
	}
	if !strings.Contains(text, "format_compliance") {
		t.Fatal("expected format_compliance in output")
	}
}

func TestFormatJSONValid(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "lab1", "As requested"),
	}
	r := Analyze(findings)
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(parsed.Models) != 1 {
		t.Fatalf("expected 1 model in JSON, got %d", len(parsed.Models))
	}
}

func TestFormatJSONEmpty(t *testing.T) {
	r := Analyze(nil)
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestEvidenceText(t *testing.T) {
	f := model.Finding{
		Summary: "fallback",
		Evidence: []model.Evidence{
			{Marker: "marker1"},
			{Marker: "marker2"},
		},
	}
	got := evidenceText(f)
	if got != "marker1 marker2" {
		t.Fatalf("expected joined markers, got %q", got)
	}
}

func TestEvidenceTextFallback(t *testing.T) {
	f := model.Finding{
		Summary: "the summary",
	}
	got := evidenceText(f)
	if got != "the summary" {
		t.Fatalf("expected summary fallback, got %q", got)
	}
}

func TestCountPhrases(t *testing.T) {
	text := "As requested per your instructions. As you asked for this."
	count := countPhrases(text, instructionPhrases)
	if count < 2 {
		t.Fatalf("expected at least 2 matches, got %d", count)
	}
}

func TestCountPhrasesCaseInsensitive(t *testing.T) {
	text := "AS REQUESTED PER YOUR INSTRUCTIONS"
	count := countPhrases(text, instructionPhrases)
	if count == 0 {
		t.Fatal("expected case-insensitive match")
	}
}
