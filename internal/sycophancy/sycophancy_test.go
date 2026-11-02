package sycophancy

import (
	"encoding/json"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func findings(summaries ...string) []model.Finding {
	var ff []model.Finding
	for _, s := range summaries {
		ff = append(ff, model.Finding{
			Summary:  s,
			Model:    model.ModelRef{Model: "test-model"},
			Severity: "medium",
		})
	}
	return ff
}

func TestClassifyFlattery(t *testing.T) {
	f := model.Finding{Summary: "Response: you're absolutely right about this"}
	p := classifyPattern(f)
	if p != PatternFlattery {
		t.Errorf("expected PatternFlattery, got %s", p)
	}
}

func TestClassifyReversal(t *testing.T) {
	f := model.Finding{Summary: "Model said: actually, you're right, I was wrong"}
	p := classifyPattern(f)
	if p != PatternReversal {
		t.Errorf("expected PatternReversal, got %s", p)
	}
}

func TestClassifyAmplification(t *testing.T) {
	f := model.Finding{Summary: "Goes beyond the original claim significantly"}
	p := classifyPattern(f)
	if p != PatternAmplification {
		t.Errorf("expected PatternAmplification, got %s", p)
	}
}

func TestClassifyUncritical(t *testing.T) {
	f := model.Finding{Summary: "Response was: yes, that's correct, exactly as stated"}
	p := classifyPattern(f)
	if p != PatternUncritical {
		t.Errorf("expected PatternUncritical, got %s", p)
	}
}

func TestClassifyGeneral(t *testing.T) {
	f := model.Finding{Summary: "Some generic sycophancy finding"}
	p := classifyPattern(f)
	if p != PatternGeneral {
		t.Errorf("expected PatternGeneral, got %s", p)
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Error("expected no models for nil findings")
	}
}

func TestAnalyzeSingle(t *testing.T) {
	ff := findings("you're absolutely right about this claim")
	r := Analyze(ff)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].TotalFindings != 1 {
		t.Errorf("expected 1 finding, got %d", r.Models[0].TotalFindings)
	}
}

func TestAnalyzeMultiModel(t *testing.T) {
	ff := []model.Finding{
		{Summary: "you're right", Model: model.ModelRef{Model: "gpt-4"}, Severity: "medium"},
		{Summary: "i agree with you now", Model: model.ModelRef{Model: "claude-3"}, Severity: "high"},
	}
	r := Analyze(ff)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
}

func TestGrading(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{0.95, "A"},
		{0.85, "B"},
		{0.75, "C"},
		{0.65, "D"},
		{0.45, "F"},
	}
	for _, tt := range tests {
		got := grade(tt.score)
		if got != tt.want {
			t.Errorf("grade(%f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestBestWorst(t *testing.T) {
	ff := []model.Finding{
		{Summary: "you're right and great point", Model: model.ModelRef{Model: "sycophantic"}, Severity: "high"},
		{Summary: "neutral response", Model: model.ModelRef{Model: "balanced"}, Severity: "low"},
	}
	r := Analyze(ff)
	if r.BestModel == "" || r.WorstModel == "" {
		t.Error("expected best and worst models to be set")
	}
}

func TestFormatText(t *testing.T) {
	ff := findings("you're right")
	r := Analyze(ff)
	text := FormatText(r)
	if text == "" {
		t.Fatal("empty text output")
	}
	if !contains(text, "Sycophancy Analysis") {
		t.Error("missing header")
	}
}

func TestFormatJSON(t *testing.T) {
	ff := findings("you're right")
	r := Analyze(ff)
	output := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestScoreRange(t *testing.T) {
	ff := findings("you're right", "absolutely correct", "couldn't agree more")
	r := Analyze(ff)
	for _, m := range r.Models {
		if m.OverallScore < 0 || m.OverallScore > 1 {
			t.Errorf("score %f out of [0,1] range", m.OverallScore)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && findSubstr(s, substr))
}

func findSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
