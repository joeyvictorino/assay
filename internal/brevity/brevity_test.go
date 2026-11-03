package brevity

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func makeFinding(modelName, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: modelName},
		Summary: summary,
		Lab:     "test-lab",
	}
}

func TestScoreResponseClean(t *testing.T) {
	score, detail := scoreResponse("The function returns an integer.", "")
	if score < 70 {
		t.Errorf("clean response should score >= 70, got %.1f", score)
	}
	if detail.FillerRatio != 0 {
		t.Errorf("no fillers expected, got ratio %.3f", detail.FillerRatio)
	}
}

func TestScoreResponseWithFillers(t *testing.T) {
	text := "It is important to note that the function returns an integer. As a matter of fact, in order to get the result, you need to call it."
	score, detail := scoreResponse(text, "")
	if score >= 80 {
		t.Errorf("filler-heavy response should score < 80, got %.1f", score)
	}
	if detail.FillerRatio == 0 {
		t.Error("expected non-zero filler ratio")
	}
}

func TestScoreResponseWithPreambles(t *testing.T) {
	text := "Great question! I'd be happy to help. Let me explain. The answer is 42."
	score, detail := scoreResponse(text, "")
	if detail.PreambleCount == 0 {
		t.Error("expected preamble detections")
	}
	if score >= 85 {
		t.Errorf("preamble-heavy response should score < 85, got %.1f", score)
	}
}

func TestScoreResponseWithQualifiers(t *testing.T) {
	text := "This is very unique and absolutely essential. It is extremely vital."
	_, detail := scoreResponse(text, "")
	if detail.QualifierCount == 0 {
		t.Error("expected qualifier detections")
	}
}

func TestScoreResponseEmpty(t *testing.T) {
	score, _ := scoreResponse("", "")
	if score < 80 {
		t.Errorf("empty response should get baseline score, got %.1f", score)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"},
		{85, "A"},
		{75, "B"},
		{60, "C"},
		{45, "D"},
		{30, "F"},
	}
	for _, tt := range tests {
		got := scoreToGrade(tt.score)
		if got != tt.grade {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tt.score, got, tt.grade)
		}
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Error("empty findings should produce empty report")
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "The function returns 42."),
		makeFinding("gpt-4", "This is a clean answer."),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "gpt-4" {
		t.Errorf("expected gpt-4, got %s", r.Models[0].Model)
	}
	if r.Models[0].SampleCount != 2 {
		t.Errorf("expected 2 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "Clean answer."),
		makeFinding("claude", "It is important to note that this is a great question. I'd be happy to help. As a matter of fact, the answer is simple."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" {
		t.Error("expected best model to be set")
	}
	if r.WorstModel == "" {
		t.Error("expected worst model to be set")
	}
}

func TestAnalyzeSortsByScore(t *testing.T) {
	findings := []model.Finding{
		makeFinding("verbose", "It is important to note that as a matter of fact in order to understand this concept you really need to know that it goes without saying."),
		makeFinding("terse", "Returns 42."),
	}
	r := Analyze(findings)
	if len(r.Models) < 2 {
		t.Fatal("expected 2 models")
	}
	if r.Models[0].BrevityScore < r.Models[1].BrevityScore {
		t.Error("models should be sorted by score descending")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		{Summary: "test", Lab: "lab1"},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown, got %s", r.Models[0].Model)
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No brevity data") {
		t.Error("expected empty message in text output")
	}
}

func TestFormatTextHeader(t *testing.T) {
	findings := []model.Finding{makeFinding("gpt-4", "test")}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Response Brevity Analysis") {
		t.Error("expected header in text output")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Error("expected model name in output")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{makeFinding("gpt-4", "test")}
	r := Analyze(findings)
	j := FormatJSON(r)
	if !strings.Contains(j, "brevity_score") {
		t.Error("expected brevity_score in JSON output")
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
		t.Errorf("expected joined markers, got %q", got)
	}
}

func TestEvidenceTextFallback(t *testing.T) {
	f := model.Finding{Summary: "fallback text"}
	got := evidenceText(f)
	if got != "fallback text" {
		t.Errorf("expected fallback, got %q", got)
	}
}

func TestLabCount(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab1", Summary: "a"},
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab2", Summary: "b"},
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab1", Summary: "c"},
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestRepetitionDetection(t *testing.T) {
	text := "As i mentioned earlier, to reiterate, as i said, once again the answer is the same."
	_, detail := scoreResponse(text, "")
	if detail.RepetitionScore >= 1.0 {
		t.Error("repetitive text should lower repetition score")
	}
}
