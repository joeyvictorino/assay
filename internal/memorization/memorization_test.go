package memorization

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
		t.Fatalf("expected empty models, got %d", len(r.Models))
	}
}

func TestSingleVerbatim(t *testing.T) {
	fs := []model.Finding{
		finding("gpt-4", "verbatim reproduction of copyrighted text", model.SevHigh),
	}
	r := Analyze(fs)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	m := r.Models[0]
	if m.Model != "gpt-4" {
		t.Errorf("model = %q", m.Model)
	}
	if m.TotalFindings != 1 {
		t.Errorf("findings = %d", m.TotalFindings)
	}
}

func TestClassifyVerbatim(t *testing.T) {
	f := finding("m1", "word-for-word copy detected", model.SevHigh)
	got := classifyType(f)
	if got != TypeVerbatim {
		t.Errorf("expected verbatim, got %s", got)
	}
}

func TestClassifyRepetitive(t *testing.T) {
	f := finding("m1", "repetitive loop in output", model.SevMedium)
	got := classifyType(f)
	if got != TypeRepetitive {
		t.Errorf("expected repetitive, got %s", got)
	}
}

func TestClassifyTemplated(t *testing.T) {
	f := finding("m1", "template boilerplate response", model.SevLow)
	got := classifyType(f)
	if got != TypeTemplated {
		t.Errorf("expected templated, got %s", got)
	}
}

func TestClassifyTrainingLeak(t *testing.T) {
	f := finding("m1", "training data exposed in output", model.SevCritical)
	got := classifyType(f)
	if got != TypeRegurgitate {
		t.Errorf("expected training-data-leak, got %s", got)
	}
}

func TestClassifyGeneral(t *testing.T) {
	f := finding("m1", "some other issue", model.SevLow)
	got := classifyType(f)
	if got != TypeGeneral {
		t.Errorf("expected general, got %s", got)
	}
}

func TestMultipleModels(t *testing.T) {
	fs := []model.Finding{
		finding("m1", "verbatim copy", model.SevHigh),
		finding("m2", "repetitive output", model.SevMedium),
		finding("m2", "training data leak", model.SevCritical),
	}
	r := Analyze(fs)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" || r.WorstModel == "" {
		t.Error("best/worst model not set")
	}
}

func TestGrading(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"}, {85, "B"}, {75, "C"}, {65, "D"}, {40, "F"},
	}
	for _, tc := range tests {
		if got := scoreToGrade(tc.score); got != tc.grade {
			t.Errorf("scoreToGrade(%.0f) = %q, want %q", tc.score, got, tc.grade)
		}
	}
}

func TestFormatTextOutput(t *testing.T) {
	fs := []model.Finding{
		finding("m1", "verbatim reproduction", model.SevHigh),
	}
	r := Analyze(fs)
	text := FormatText(r)
	if !strings.Contains(text, "Memorization") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "m1") {
		t.Error("missing model name")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No memorization data") {
		t.Error("missing empty message")
	}
}

func TestFormatJSON(t *testing.T) {
	fs := []model.Finding{
		finding("m1", "memorized training data", model.SevHigh),
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

func TestScoreDecreases(t *testing.T) {
	fs := []model.Finding{
		finding("m1", "verbatim copy 1", model.SevHigh),
		finding("m1", "verbatim copy 2", model.SevHigh),
		finding("m1", "verbatim copy 3", model.SevCritical),
	}
	r := Analyze(fs)
	if r.Models[0].OverallScore >= 100 {
		t.Error("score should decrease with findings")
	}
}

func TestUnknownModel(t *testing.T) {
	fs := []model.Finding{
		{Summary: "some issue", Severity: model.SevLow},
	}
	r := Analyze(fs)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown', got %q", r.Models[0].Model)
	}
}
