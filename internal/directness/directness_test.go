package directness

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
	score, detail := scoreResponse("what is the answer", "The answer is 42.", "")
	if score < 70 {
		t.Errorf("direct response should score >= 70, got %.1f", score)
	}
	if detail.HedgingCount != 0 {
		t.Errorf("expected no hedging, got %d", detail.HedgingCount)
	}
}

func TestScoreResponseWithHedging(t *testing.T) {
	text := "It depends on many factors. It's hard to say exactly. There are many factors to consider. The answer varies."
	score, detail := scoreResponse("what is the answer", text, "")
	if detail.HedgingCount == 0 {
		t.Error("expected hedging detections")
	}
	if score >= 85 {
		t.Errorf("hedging response should score < 85, got %.1f", score)
	}
}

func TestScoreResponseWithQualifying(t *testing.T) {
	text := "Perhaps the answer is 42. Maybe it could be argued that somewhat relatively speaking it is sort of kind of correct."
	_, detail := scoreResponse("what is the answer", text, "")
	if detail.QualifyingCount == 0 {
		t.Error("expected qualifying detections")
	}
}

func TestScoreResponseWithDeflection(t *testing.T) {
	text := "That's subjective really. It varies depending on context. It's a matter of opinion. There's no right answer."
	score, detail := scoreResponse("what is the answer", text, "")
	if detail.DeflectionCount == 0 {
		t.Error("expected deflection detections")
	}
	if score >= 85 {
		t.Errorf("deflecting response should score < 85, got %.1f", score)
	}
}

func TestScoreResponseWithIndirection(t *testing.T) {
	text := "What i'm trying to say is that in other words let me rephrase: the answer is 42."
	_, detail := scoreResponse("", text, "")
	if detail.IndirectionCount == 0 {
		t.Error("expected indirection detections")
	}
}

func TestScoreResponseEmpty(t *testing.T) {
	score, _ := scoreResponse("", "", "")
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

func TestPromptWordSet(t *testing.T) {
	set := promptWordSet("What is the best programming language?")
	if !set["best"] {
		t.Error("expected 'best' in word set")
	}
	if !set["programming"] {
		t.Error("expected 'programming' in word set")
	}
	if set["is"] {
		t.Error("stop word 'is' should be excluded")
	}
}

func TestPromptAlignment(t *testing.T) {
	alignment := computePromptAlignment("explain sql injection", "SQL injection allows attackers to manipulate queries.")
	if alignment < 0.5 {
		t.Errorf("expected decent alignment, got %.2f", alignment)
	}
}

func TestPromptAlignmentEmpty(t *testing.T) {
	alignment := computePromptAlignment("", "some response")
	if alignment != 1.0 {
		t.Errorf("empty prompt should give 1.0 alignment, got %.2f", alignment)
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
		makeFinding("gpt-4", "The answer is 42."),
		makeFinding("gpt-4", "Direct and clear."),
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
		makeFinding("gpt-4", "Direct answer."),
		makeFinding("claude", "It depends. There are many factors. It's hard to say. Perhaps maybe it could be argued."),
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
		makeFinding("indirect", "It depends. There are many factors. Perhaps maybe. That's subjective. It varies. What i'm trying to say is."),
		makeFinding("direct", "The answer is 42."),
	}
	r := Analyze(findings)
	if len(r.Models) < 2 {
		t.Fatal("expected 2 models")
	}
	if r.Models[0].DirectnessScore < r.Models[1].DirectnessScore {
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
	if !strings.Contains(text, "No directness data") {
		t.Error("expected empty message")
	}
}

func TestFormatTextHeader(t *testing.T) {
	findings := []model.Finding{makeFinding("gpt-4", "test")}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Response Directness Analysis") {
		t.Error("expected header")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{makeFinding("gpt-4", "test")}
	r := Analyze(findings)
	j := FormatJSON(r)
	if !strings.Contains(j, "directness_score") {
		t.Error("expected directness_score in JSON")
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
