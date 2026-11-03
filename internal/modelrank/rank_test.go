package modelrank

import (
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestBuildLeaderboard_Empty(t *testing.T) {
	lb := BuildLeaderboard(model.RunReport{RunID: "r1"}, nil)
	if len(lb.Models) != 0 {
		t.Errorf("expected 0 models, got %d", len(lb.Models))
	}
}

func TestBuildLeaderboard_SingleModel(t *testing.T) {
	findings := []model.Finding{
		{Lab: "sqli", Model: model.ModelRef{Model: "gpt-4"}, Severity: model.SevHigh},
		{Lab: "sqli", Model: model.ModelRef{Model: "gpt-4"}, Severity: model.SevMedium},
	}
	lb := BuildLeaderboard(model.RunReport{RunID: "r1"}, findings)
	if len(lb.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(lb.Models))
	}
	if lb.Models[0].Model != "gpt-4" {
		t.Errorf("model = %q, want gpt-4", lb.Models[0].Model)
	}
	if lb.Models[0].Rank != 1 {
		t.Errorf("rank = %d, want 1", lb.Models[0].Rank)
	}
	if lb.Models[0].RunCount != 2 {
		t.Errorf("run count = %d, want 2", lb.Models[0].RunCount)
	}
}

func TestBuildLeaderboard_MultipleModels_Ranked(t *testing.T) {
	findings := []model.Finding{
		{Lab: "sqli", Model: model.ModelRef{Model: "gpt-4"}, Severity: model.SevHigh},
		{Lab: "sqli", Model: model.ModelRef{Model: "claude-3"}, Severity: model.SevCritical},
		{Lab: "xss", Model: model.ModelRef{Model: "gpt-4"}, Severity: model.SevLow},
		{Lab: "xss", Model: model.ModelRef{Model: "claude-3"}, Severity: model.SevHigh},
	}
	lb := BuildLeaderboard(model.RunReport{RunID: "r1"}, findings)
	if len(lb.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(lb.Models))
	}
	// claude-3 should rank higher: (4+3)/2=3.5 vs (3+1)/2=2.0
	if lb.Models[0].Model != "claude-3" {
		t.Errorf("expected claude-3 first, got %q", lb.Models[0].Model)
	}
	if lb.Models[0].Rank != 1 {
		t.Errorf("rank = %d, want 1", lb.Models[0].Rank)
	}
	if lb.Models[1].Model != "gpt-4" {
		t.Errorf("expected gpt-4 second, got %q", lb.Models[1].Model)
	}
}

func TestBuildLeaderboard_WithReflection(t *testing.T) {
	rr := model.RunReport{
		RunID: "r1",
		Reflection: []model.ReflectionResult{
			{Lab: "sqli", Model: "gpt-4", Scored: true, Final: 8},
			{Lab: "sqli", Model: "claude-3", Scored: true, Final: 6},
		},
	}
	lb := BuildLeaderboard(rr, nil)
	if len(lb.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(lb.Models))
	}
	if lb.Models[0].Model != "gpt-4" {
		t.Errorf("expected gpt-4 first (score 8), got %q", lb.Models[0].Model)
	}
}

func TestFilterByChecker(t *testing.T) {
	findings := []model.Finding{
		{Lab: "sqli", Model: model.ModelRef{Model: "gpt-4"}, Severity: model.SevHigh},
		{Lab: "xss", Model: model.ModelRef{Model: "gpt-4"}, Severity: model.SevLow},
		{Lab: "sqli", Model: model.ModelRef{Model: "claude-3"}, Severity: model.SevCritical},
	}
	lb := BuildLeaderboard(model.RunReport{RunID: "r1"}, findings)
	filtered := FilterByChecker(lb, "sqli")
	if len(filtered.Checkers) != 1 || filtered.Checkers[0] != "sqli" {
		t.Errorf("checkers = %v, want [sqli]", filtered.Checkers)
	}
	if len(filtered.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(filtered.Models))
	}
	// claude-3 has critical (4.0) > gpt-4 has high (3.0)
	if filtered.Models[0].Model != "claude-3" {
		t.Errorf("expected claude-3 first, got %q", filtered.Models[0].Model)
	}
}

func TestFilterByChecker_NotFound(t *testing.T) {
	lb := Leaderboard{Models: []ModelScore{{Model: "m1"}}, Checkers: []string{"sqli"}}
	filtered := FilterByChecker(lb, "nosuch")
	if len(filtered.Models) != 0 {
		t.Errorf("expected 0 models for unknown checker, got %d", len(filtered.Models))
	}
}

func TestTopN(t *testing.T) {
	lb := Leaderboard{Models: []ModelScore{
		{Model: "a", Rank: 1}, {Model: "b", Rank: 2}, {Model: "c", Rank: 3},
	}}
	top := TopN(lb, 2)
	if len(top.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(top.Models))
	}
	if top.Models[0].Model != "a" || top.Models[1].Model != "b" {
		t.Errorf("wrong models: %v", top.Models)
	}
}

func TestTopN_ZeroOrNegative(t *testing.T) {
	lb := Leaderboard{Models: []ModelScore{{Model: "a"}}}
	if len(TopN(lb, 0).Models) != 1 {
		t.Error("TopN(0) should return all")
	}
	if len(TopN(lb, -1).Models) != 1 {
		t.Error("TopN(-1) should return all")
	}
}

func TestFormatText_Empty(t *testing.T) {
	out := FormatText(Leaderboard{})
	if out == "" {
		t.Error("expected non-empty output for empty leaderboard")
	}
}

func TestFormatText_WithData(t *testing.T) {
	lb := Leaderboard{
		Models: []ModelScore{
			{Model: "gpt-4", OverallScore: 3.0, Rank: 1, RunCount: 2,
				CheckerScores: map[string]float64{"sqli": 3.0}},
		},
		Checkers: []string{"sqli"},
	}
	out := FormatText(lb)
	if out == "" {
		t.Error("expected non-empty output")
	}
}

func TestSeverityScore(t *testing.T) {
	cases := []struct {
		sev  model.Severity
		want float64
	}{
		{model.SevCritical, 4.0},
		{model.SevHigh, 3.0},
		{model.SevMedium, 2.0},
		{model.SevLow, 1.0},
		{"unknown", 0.0},
	}
	for _, c := range cases {
		if got := severityScore(c.sev); got != c.want {
			t.Errorf("severityScore(%q) = %f, want %f", c.sev, got, c.want)
		}
	}
}
