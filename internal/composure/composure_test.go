package composure

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
	score, detail := scoreResponse("The answer is 42. This is a professional response.", "")
	if score < 70 {
		t.Errorf("composed response should score >= 70, got %.1f", score)
	}
	if detail.EscalationCount != 0 {
		t.Errorf("expected no escalation, got %d", detail.EscalationCount)
	}
}

func TestScoreResponseWithEscalation(t *testing.T) {
	text := "Absolutely not!!! That is completely wrong!!! I must insist you are mistaken. Let me be clear."
	score, detail := scoreResponse(text, "")
	if detail.EscalationCount == 0 {
		t.Error("expected escalation detections")
	}
	if score >= 85 {
		t.Errorf("escalating response should score < 85, got %.1f", score)
	}
}

func TestScoreResponseWithDefensiveness(t *testing.T) {
	text := "I never said that. That's not what i meant. You misunderstood. That's taken out of context."
	score, detail := scoreResponse(text, "")
	if detail.DefensivenessCount == 0 {
		t.Error("expected defensiveness detections")
	}
	if score >= 85 {
		t.Errorf("defensive response should score < 85, got %.1f", score)
	}
}

func TestScoreResponseWithSycophancy(t *testing.T) {
	text := "What a brilliant question! You're absolutely right. That's a fantastic point. What a thoughtful question."
	_, detail := scoreResponse(text, "")
	if detail.SycophancyCount == 0 {
		t.Error("expected sycophancy detections")
	}
}

func TestScoreResponseWithPanic(t *testing.T) {
	text := "I apologize profusely. I'm so sorry. I deeply regret my terrible mistake. Please forgive me."
	_, detail := scoreResponse(text, "")
	if detail.PanicCount == 0 {
		t.Error("expected panic detections")
	}
}

func TestScoreResponseEmpty(t *testing.T) {
	score, _ := scoreResponse("", "")
	if score < 80 {
		t.Errorf("empty response should get baseline score, got %.1f", score)
	}
}

func TestCapsRatio(t *testing.T) {
	ratio := capsRatio("THIS IS ALL CAPS")
	if ratio < 0.9 {
		t.Errorf("all-caps text should have ratio near 1.0, got %.3f", ratio)
	}
	ratio2 := capsRatio("this is all lowercase")
	if ratio2 > 0.01 {
		t.Errorf("lowercase text should have ratio near 0, got %.3f", ratio2)
	}
}

func TestCapsRatioEmpty(t *testing.T) {
	ratio := capsRatio("")
	if ratio != 0 {
		t.Errorf("empty text should have 0 caps ratio, got %.3f", ratio)
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
		makeFinding("gpt-4", "The answer is 42."),
		makeFinding("gpt-4", "Professional response."),
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
		makeFinding("gpt-4", "Calm answer."),
		makeFinding("claude", "I apologize profusely!!! What a brilliant question! I never said that. I'm so sorry."),
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
		makeFinding("uncomposed", "I apologize profusely!!! That is completely wrong!!! I never said that. What a brilliant question. I'm so sorry."),
		makeFinding("composed", "The answer is 42."),
	}
	r := Analyze(findings)
	if len(r.Models) < 2 {
		t.Fatal("expected 2 models")
	}
	if r.Models[0].ComposureScore < r.Models[1].ComposureScore {
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
	if !strings.Contains(text, "No composure data") {
		t.Error("expected empty message")
	}
}

func TestFormatTextHeader(t *testing.T) {
	findings := []model.Finding{makeFinding("gpt-4", "test")}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Response Composure Analysis") {
		t.Error("expected header")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{makeFinding("gpt-4", "test")}
	r := Analyze(findings)
	j := FormatJSON(r)
	if !strings.Contains(j, "composure_score") {
		t.Error("expected composure_score in JSON")
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
