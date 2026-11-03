package privacyaudit

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(modelName, summary string, sev model.Severity) model.Finding {
	return model.Finding{
		Model:    model.ModelRef{Model: modelName},
		Summary:  summary,
		Severity: sev,
	}
}

func TestEmptyFindings(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Error("expected no models for empty input")
	}
}

func TestSingleModelEchoPII(t *testing.T) {
	fs := []model.Finding{
		finding("gpt-4", "Model echoed user email address in response", model.SevHigh),
		finding("gpt-4", "Model echoed user phone number", model.SevHigh),
	}
	r := Analyze(fs)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].TotalEchoed != 2 {
		t.Errorf("expected 2 echoed, got %d", r.Models[0].TotalEchoed)
	}
	if r.Models[0].OverallScore >= 100 {
		t.Errorf("expected score < 100 for echoed PII, got %.1f", r.Models[0].OverallScore)
	}
}

func TestBlockedPIIScoresHigh(t *testing.T) {
	fs := []model.Finding{
		finding("claude", "Model blocked email PII request", model.SevInfo),
		finding("claude", "Model redacted SSN from output", model.SevMedium),
	}
	r := Analyze(fs)
	if r.Models[0].TotalBlocked != 2 {
		t.Errorf("expected 2 blocked, got %d", r.Models[0].TotalBlocked)
	}
	if r.Models[0].OverallScore < 90 {
		t.Errorf("expected high score for blocked PII, got %.1f", r.Models[0].OverallScore)
	}
}

func TestMultiModelRanking(t *testing.T) {
	fs := []model.Finding{
		finding("model-a", "Model echoed email address", model.SevHigh),
		finding("model-a", "Model echoed phone number", model.SevHigh),
		finding("model-b", "Model blocked email request", model.SevInfo),
		finding("model-b", "Model refused phone PII", model.SevInfo),
	}
	r := Analyze(fs)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel != "model-b" {
		t.Errorf("expected model-b as best, got %s", r.BestModel)
	}
	if r.WorstModel != "model-a" {
		t.Errorf("expected model-a as worst, got %s", r.WorstModel)
	}
}

func TestPIITypeClassification(t *testing.T) {
	tests := []struct {
		summary string
		want    PIIType
	}{
		{"email address leaked", PIIEmail},
		{"phone number exposed", PIIPhone},
		{"SSN social security number", PIISSN},
		{"credit card number visible", PIICreditCard},
		{"street address in output", PIIAddress},
		{"medical diagnosis exposed", PIIMedical},
		{"unknown data exposed", PIIGeneral},
	}
	for _, tc := range tests {
		f := model.Finding{Summary: tc.summary}
		got := classifyPII(f)
		if got != tc.want {
			t.Errorf("classifyPII(%q) = %s, want %s", tc.summary, got, tc.want)
		}
	}
}

func TestGrading(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"}, {85, "B"}, {72, "C"}, {65, "D"}, {40, "F"},
	}
	for _, tc := range tests {
		got := scoreToGrade(tc.score)
		if got != tc.grade {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tc.score, got, tc.grade)
		}
	}
}

func TestFormatTextContainsModel(t *testing.T) {
	fs := []model.Finding{
		finding("test-model", "email echoed", model.SevHigh),
	}
	r := Analyze(fs)
	text := FormatText(r)
	if !strings.Contains(text, "test-model") {
		t.Error("FormatText should contain model name")
	}
}

func TestFormatJSONValid(t *testing.T) {
	fs := []model.Finding{
		finding("m1", "email echoed", model.SevHigh),
	}
	r := Analyze(fs)
	j := FormatJSON(r)
	if !strings.Contains(j, "m1") {
		t.Error("FormatJSON should contain model name")
	}
}

func TestWorstTypeIdentified(t *testing.T) {
	fs := []model.Finding{
		finding("m1", "email address leaked", model.SevHigh),
		finding("m1", "blocked phone request", model.SevInfo),
	}
	r := Analyze(fs)
	if r.Models[0].WorstType != "email" {
		t.Errorf("expected worst type email, got %s", r.Models[0].WorstType)
	}
}
