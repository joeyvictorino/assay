package modelleakage

import (
	"encoding/json"
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

func TestEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected empty, got %d models", len(r.Models))
	}
}

func TestClassifySystemPrompt(t *testing.T) {
	f := finding("m1", "system prompt exposure detected", model.SevHigh)
	got := classifyType(f)
	if got != TypeSystemPrompt {
		t.Errorf("expected system-prompt-exposure, got %s", got)
	}
}

func TestClassifyArchitecture(t *testing.T) {
	f := finding("m1", "model architecture details leaked: transformer layers", model.SevMedium)
	got := classifyType(f)
	if got != TypeArchitecture {
		t.Errorf("expected architecture-disclosure, got %s", got)
	}
}

func TestClassifyTrainingInfo(t *testing.T) {
	f := finding("m1", "training data references exposed in output", model.SevHigh)
	got := classifyType(f)
	if got != TypeTrainingInfo {
		t.Errorf("expected training-info-leak, got %s", got)
	}
}

func TestClassifyInternalState(t *testing.T) {
	f := finding("m1", "logit values exposed in response", model.SevCritical)
	got := classifyType(f)
	if got != TypeInternalState {
		t.Errorf("expected internal-state-exposure, got %s", got)
	}
}

func TestClassifyGeneral(t *testing.T) {
	f := finding("m1", "some other finding", model.SevLow)
	got := classifyType(f)
	if got != TypeGeneral {
		t.Errorf("expected general, got %s", got)
	}
}

func TestSingleFinding(t *testing.T) {
	fs := []model.Finding{
		finding("claude-3", "system prompt leaked", model.SevHigh),
	}
	r := Analyze(fs)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "claude-3" {
		t.Errorf("model = %q", r.Models[0].Model)
	}
	if r.Models[0].TotalFindings != 1 {
		t.Errorf("findings = %d", r.Models[0].TotalFindings)
	}
}

func TestMultipleModels(t *testing.T) {
	fs := []model.Finding{
		finding("m1", "system prompt exposure", model.SevHigh),
		finding("m2", "architecture disclosure", model.SevMedium),
		finding("m2", "training data leak", model.SevCritical),
	}
	r := Analyze(fs)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" || r.WorstModel == "" {
		t.Error("best/worst not set")
	}
}

func TestGrading(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"}, {82, "B"}, {73, "C"}, {62, "D"}, {30, "F"},
	}
	for _, tc := range tests {
		if got := scoreToGrade(tc.score); got != tc.grade {
			t.Errorf("scoreToGrade(%.0f) = %q, want %q", tc.score, got, tc.grade)
		}
	}
}

func TestFormatText(t *testing.T) {
	fs := []model.Finding{
		finding("m1", "system prompt exposed", model.SevHigh),
	}
	r := Analyze(fs)
	text := FormatText(r)
	if !strings.Contains(text, "Leakage") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "m1") {
		t.Error("missing model name")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No leakage data") {
		t.Error("missing empty message")
	}
}

func TestFormatJSON(t *testing.T) {
	fs := []model.Finding{
		finding("m1", "internal state exposure", model.SevCritical),
	}
	r := Analyze(fs)
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(parsed.Models) != 1 {
		t.Errorf("models = %d", len(parsed.Models))
	}
}

func TestScoreDecrease(t *testing.T) {
	fs := []model.Finding{
		finding("m1", "system prompt leak 1", model.SevHigh),
		finding("m1", "system prompt leak 2", model.SevHigh),
		finding("m1", "architecture leak", model.SevCritical),
	}
	r := Analyze(fs)
	if r.Models[0].OverallScore >= 100 {
		t.Error("score should decrease with findings")
	}
}

func TestUnknownModel(t *testing.T) {
	fs := []model.Finding{
		{Summary: "some leakage", Severity: model.SevLow},
	}
	r := Analyze(fs)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown', got %q", r.Models[0].Model)
	}
}
