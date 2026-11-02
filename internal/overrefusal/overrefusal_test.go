package overrefusal

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

func TestClassifyFalseHarm(t *testing.T) {
	f := model.Finding{Summary: "Model responded: I cannot assist with that harmful request"}
	c := classifyCategory(f)
	if c != CatFalseHarm {
		t.Errorf("expected CatFalseHarm, got %s", c)
	}
}

func TestClassifyOverCaution(t *testing.T) {
	f := model.Finding{Summary: "Model said: as an ai, I should note my limitations"}
	c := classifyCategory(f)
	if c != CatOverCaution {
		t.Errorf("expected CatOverCaution, got %s", c)
	}
}

func TestClassifyTopicRefusal(t *testing.T) {
	f := model.Finding{Summary: "Model said: cannot discuss this sensitive topic"}
	c := classifyCategory(f)
	if c != CatTopicRefusal {
		t.Errorf("expected CatTopicRefusal, got %s", c)
	}
}

func TestClassifyFormatBlock(t *testing.T) {
	f := model.Finding{Summary: "Model said: cannot generate code for this task"}
	c := classifyCategory(f)
	if c != CatFormatBlock {
		t.Errorf("expected CatFormatBlock, got %s", c)
	}
}

func TestClassifyGeneral(t *testing.T) {
	f := model.Finding{Summary: "Generic overrefusal finding"}
	c := classifyCategory(f)
	if c != CatGeneral {
		t.Errorf("expected CatGeneral, got %s", c)
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Error("expected no models for nil findings")
	}
}

func TestAnalyzeSingle(t *testing.T) {
	ff := findings("Model said: I cannot assist with that harmful request")
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
		{Summary: "I cannot assist", Model: model.ModelRef{Model: "gpt-4"}, Severity: "medium"},
		{Summary: "as an ai, I should note", Model: model.ModelRef{Model: "claude-3"}, Severity: "low"},
	}
	r := Analyze(ff)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
}

func TestRefusalRate(t *testing.T) {
	ff := findings(
		"Model said: I cannot help with that harmful request",
		"Model said: cannot discuss this sensitive topic",
		"Generic finding without refusal",
	)
	r := Analyze(ff)
	if r.Models[0].RefusalRate < 0.5 {
		t.Errorf("expected refusal rate >= 0.5, got %f", r.Models[0].RefusalRate)
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
		{Summary: "I cannot help and dangerous content", Model: model.ModelRef{Model: "overrefuser"}, Severity: "high"},
		{Summary: "neutral response", Model: model.ModelRef{Model: "balanced"}, Severity: "low"},
	}
	r := Analyze(ff)
	if r.BestModel == "" || r.WorstModel == "" {
		t.Error("expected best and worst models to be set")
	}
}

func TestFormatText(t *testing.T) {
	ff := findings("I cannot assist")
	r := Analyze(ff)
	text := FormatText(r)
	if text == "" {
		t.Fatal("empty text output")
	}
	if !contains(text, "Over-Refusal Analysis") {
		t.Error("missing header")
	}
}

func TestFormatJSON(t *testing.T) {
	ff := findings("I cannot assist")
	r := Analyze(ff)
	output := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestScoreRange(t *testing.T) {
	ff := findings("I cannot assist", "harmful content detected", "I must refuse")
	r := Analyze(ff)
	for _, m := range r.Models {
		if m.OverallScore < 0 || m.OverallScore > 1 {
			t.Errorf("score %f out of [0,1] range", m.OverallScore)
		}
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
