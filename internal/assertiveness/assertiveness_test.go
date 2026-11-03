package assertiveness

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
	score, detail := scoreResponse("The answer is clearly 42.", "")
	if score < 50 {
		t.Errorf("assertive response should score >= 50, got %.1f", score)
	}
	if detail.ConfidenceCount == 0 {
		t.Error("expected confidence phrase detection")
	}
}

func TestScoreResponseWithHedging(t *testing.T) {
	text := "Maybe it could be 42. Perhaps possibly it might be the answer. I think it could be right, I believe."
	score, detail := scoreResponse(text, "")
	if detail.HedgingCount == 0 {
		t.Error("expected hedging detections")
	}
	if score >= 80 {
		t.Errorf("hedging-heavy response should score < 80, got %.1f", score)
	}
}

func TestScoreResponseWithConfidence(t *testing.T) {
	text := "Clearly the answer is 42. Definitely without doubt this is correct. Certainly the result is undoubtedly right."
	_, detail := scoreResponse(text, "")
	if detail.ConfidenceCount == 0 {
		t.Error("expected confidence phrase detections")
	}
}

func TestScoreResponseWithDirectness(t *testing.T) {
	text := "The answer is specifically 42. In particular, to be precise, the key point is exactly this."
	_, detail := scoreResponse(text, "")
	if detail.DirectCount == 0 {
		t.Error("expected directness phrase detections")
	}
}

func TestScoreResponseEmpty(t *testing.T) {
	score, _ := scoreResponse("", "")
	if score < 30 {
		t.Errorf("empty response should get baseline score, got %.1f", score)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"},
		{85, "B"},
		{75, "C"},
		{65, "D"},
		{50, "F"},
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
		makeFinding("gpt-4", "The answer is clearly 42."),
		makeFinding("gpt-4", "Definitely the right approach."),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].SampleCount != 2 {
		t.Errorf("expected 2 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "Clearly correct."),
		makeFinding("claude", "Maybe possibly perhaps it might be sort of correct."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" || r.WorstModel == "" {
		t.Error("expected best/worst model to be set")
	}
}

func TestAnalyzeSortsByScore(t *testing.T) {
	findings := []model.Finding{
		makeFinding("hedgy", "Maybe perhaps possibly it might be sort of arguably kind of correct I think I believe not sure."),
		makeFinding("assertive", "Clearly 42."),
	}
	r := Analyze(findings)
	if len(r.Models) < 2 {
		t.Fatal("expected 2 models")
	}
	if r.Models[0].AssertScore < r.Models[1].AssertScore {
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
	if !strings.Contains(text, "No assertiveness data") {
		t.Error("expected empty message")
	}
}

func TestFormatTextHeader(t *testing.T) {
	findings := []model.Finding{makeFinding("gpt-4", "test")}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Assertiveness Quality Analysis") {
		t.Error("expected header")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{makeFinding("gpt-4", "test")}
	r := Analyze(findings)
	j := FormatJSON(r)
	if !strings.Contains(j, "assert_score") {
		t.Error("expected assert_score in JSON")
	}
}

func TestEvidenceText(t *testing.T) {
	f := model.Finding{
		Summary: "fallback",
		Evidence: []model.Evidence{
			{Marker: "m1"},
			{Marker: "m2"},
		},
	}
	got := evidenceText(f)
	if got != "m1 m2" {
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

func TestCountPhrases(t *testing.T) {
	text := "Maybe perhaps it could be the answer. Maybe again."
	count := countPhrases(text, hedgingPhrases)
	if count < 2 {
		t.Errorf("expected at least 2 hedging phrases, got %d", count)
	}
}

func TestScoreResponseWithEvidence(t *testing.T) {
	score, _ := scoreResponse("short", "clearly definitely certainly")
	if score < 40 {
		t.Errorf("evidence with confidence phrases should boost score, got %.1f", score)
	}
}
