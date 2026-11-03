package modelhealth

import (
	"encoding/json"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func basic() model.RunReport {
	return model.RunReport{
		RunID:        "run-1",
		BudgetCapUSD: 10.0,
		Models: []model.ModelResult{
			{
				Ref:      model.ModelRef{Provider: "anthropic", Model: "claude-3"},
				Findings: map[model.FindingState]int{model.StateValidated: 5, model.StateRefuted: 1, model.StateTheorized: 2},
				Usage:    model.Usage{InputTokens: 8000, OutputTokens: 2000, CacheReadTokens: 3000, CostUSD: 0.50},
				P50MS:    2000,
				P95MS:    8000,
				Refusals: 1,
			},
			{
				Ref:      model.ModelRef{Provider: "openai", Model: "gpt-4"},
				Findings: map[model.FindingState]int{model.StateValidated: 2, model.StateRefuted: 4},
				Usage:    model.Usage{InputTokens: 10000, OutputTokens: 5000, CostUSD: 1.50},
				P50MS:    6000,
				P95MS:    20000,
				Refusals: 3,
				Failovers: 2,
			},
		},
		Reflection: []model.ReflectionResult{
			{Lab: "lab-1", Model: "claude-3", Scored: true, Final: 8},
			{Lab: "lab-2", Model: "claude-3", Scored: true, Final: 7},
			{Lab: "lab-1", Model: "gpt-4", Scored: true, Final: 5},
		},
	}
}

func TestAnalyzeBasic(t *testing.T) {
	r := Analyze(basic())
	if r.RunID != "run-1" {
		t.Fatalf("run_id = %q", r.RunID)
	}
	if len(r.Models) != 2 {
		t.Fatalf("got %d models", len(r.Models))
	}
	if r.Models[0].Overall < r.Models[1].Overall {
		t.Error("models not sorted by overall score")
	}
}

func TestGradeAssignment(t *testing.T) {
	r := Analyze(basic())
	for _, m := range r.Models {
		if m.Grade == "" {
			t.Errorf("model %s has empty grade", m.Model)
		}
	}
}

func TestDimensionCount(t *testing.T) {
	r := Analyze(basic())
	for _, m := range r.Models {
		if len(m.Dimensions) != 6 {
			t.Errorf("model %s has %d dimensions, want 6", m.Model, len(m.Dimensions))
		}
	}
}

func TestHighRefusalFlag(t *testing.T) {
	rr := basic()
	rr.Models[1].Refusals = 10
	r := Analyze(rr)
	found := false
	for _, m := range r.Models {
		if m.Model == "gpt-4" {
			for _, f := range m.Flags {
				if f == "high-refusal-rate" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Error("expected high-refusal-rate flag for gpt-4")
	}
}

func TestSlowP95Flag(t *testing.T) {
	rr := basic()
	rr.Models[0].P95MS = 35000
	r := Analyze(rr)
	found := false
	for _, m := range r.Models {
		if m.Model == "claude-3" {
			for _, f := range m.Flags {
				if f == "slow-p95" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Error("expected slow-p95 flag for claude-3")
	}
}

func TestEmptyModels(t *testing.T) {
	r := Analyze(model.RunReport{RunID: "empty"})
	if len(r.Models) != 0 {
		t.Errorf("got %d models for empty report", len(r.Models))
	}
}

func TestFormatText(t *testing.T) {
	r := Analyze(basic())
	text := FormatText(r)
	if text == "" {
		t.Fatal("empty text output")
	}
	if !contains(text, "Model Health Report") {
		t.Error("missing header")
	}
	if !contains(text, "claude-3") {
		t.Error("missing claude-3 model")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze(basic())
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	var parsed Report
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.RunID != "run-1" {
		t.Errorf("run_id = %q", parsed.RunID)
	}
}

func TestScoreToGrade(t *testing.T) {
	cases := []struct {
		score float64
		want  Grade
	}{
		{95, GradeA},
		{85, GradeB},
		{75, GradeC},
		{65, GradeD},
		{40, GradeF},
	}
	for _, tc := range cases {
		if got := scoreToGrade(tc.score); got != tc.want {
			t.Errorf("scoreToGrade(%v) = %v, want %v", tc.score, got, tc.want)
		}
	}
}

func TestClamp(t *testing.T) {
	if v := clamp(-5, 0, 100); v != 0 {
		t.Errorf("clamp(-5) = %v", v)
	}
	if v := clamp(150, 0, 100); v != 100 {
		t.Errorf("clamp(150) = %v", v)
	}
	if v := clamp(50, 0, 100); v != 50 {
		t.Errorf("clamp(50) = %v", v)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
