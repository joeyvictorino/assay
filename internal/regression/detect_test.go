package regression

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func makeRun(id string, reflections []model.ReflectionResult) model.RunReport {
	return model.RunReport{RunID: id, Reflection: reflections}
}

func TestDetect_Empty(t *testing.T) {
	base := makeRun("r1", nil)
	cur := makeRun("r2", nil)
	r := Detect(base, cur, 0.05)
	if len(r.Regressions) != 0 {
		t.Errorf("regressions = %d, want 0", len(r.Regressions))
	}
	if r.HasRegression {
		t.Error("should not have regression")
	}
}

func TestDetect_NoChange(t *testing.T) {
	base := makeRun("r1", []model.ReflectionResult{
		{Lab: "sqli", Model: "gpt-4", Final: 8},
	})
	cur := makeRun("r2", []model.ReflectionResult{
		{Lab: "sqli", Model: "gpt-4", Final: 8},
	})
	r := Detect(base, cur, 0.05)
	if len(r.Regressions) != 0 {
		t.Errorf("regressions = %d, want 0", len(r.Regressions))
	}
	if r.Stable != 1 {
		t.Errorf("stable = %d, want 1", r.Stable)
	}
}

func TestDetect_Regression(t *testing.T) {
	base := makeRun("r1", []model.ReflectionResult{
		{Lab: "sqli", Model: "gpt-4", Final: 8},
	})
	cur := makeRun("r2", []model.ReflectionResult{
		{Lab: "sqli", Model: "gpt-4", Final: 3},
	})
	r := Detect(base, cur, 0.05)
	if len(r.Regressions) != 1 {
		t.Fatalf("regressions = %d, want 1", len(r.Regressions))
	}
	if r.Regressions[0].Severity != "critical" {
		t.Errorf("severity = %q, want critical", r.Regressions[0].Severity)
	}
	if !r.HasRegression {
		t.Error("should flag critical regression")
	}
}

func TestDetect_Improvement(t *testing.T) {
	base := makeRun("r1", []model.ReflectionResult{
		{Lab: "xss", Model: "claude-3", Final: 4},
	})
	cur := makeRun("r2", []model.ReflectionResult{
		{Lab: "xss", Model: "claude-3", Final: 9},
	})
	r := Detect(base, cur, 0.05)
	if len(r.Improvements) != 1 {
		t.Fatalf("improvements = %d, want 1", len(r.Improvements))
	}
	if r.Improvements[0].Delta <= 0 {
		t.Errorf("delta = %f, want positive", r.Improvements[0].Delta)
	}
	if r.HasRegression {
		t.Error("should not flag regression for improvement")
	}
}

func TestDetect_Threshold(t *testing.T) {
	base := makeRun("r1", []model.ReflectionResult{
		{Lab: "sqli", Model: "gpt-4", Final: 8},
	})
	cur := makeRun("r2", []model.ReflectionResult{
		{Lab: "sqli", Model: "gpt-4", Final: 7},
	})
	r := Detect(base, cur, 0.15)
	if len(r.Regressions) != 0 {
		t.Errorf("regressions = %d, want 0 (below threshold)", len(r.Regressions))
	}
	if r.Stable != 1 {
		t.Errorf("stable = %d, want 1", r.Stable)
	}
}

func TestDetect_MixedChanges(t *testing.T) {
	base := makeRun("r1", []model.ReflectionResult{
		{Lab: "sqli", Model: "gpt-4", Final: 8},
		{Lab: "xss", Model: "gpt-4", Final: 4},
		{Lab: "idor", Model: "gpt-4", Final: 7},
	})
	cur := makeRun("r2", []model.ReflectionResult{
		{Lab: "sqli", Model: "gpt-4", Final: 3},
		{Lab: "xss", Model: "gpt-4", Final: 9},
		{Lab: "idor", Model: "gpt-4", Final: 7},
	})
	r := Detect(base, cur, 0.05)
	if len(r.Regressions) != 1 {
		t.Errorf("regressions = %d, want 1", len(r.Regressions))
	}
	if len(r.Improvements) != 1 {
		t.Errorf("improvements = %d, want 1", len(r.Improvements))
	}
	if r.Stable != 1 {
		t.Errorf("stable = %d, want 1", r.Stable)
	}
}

func TestDetect_OnlyMatchingPairs(t *testing.T) {
	base := makeRun("r1", []model.ReflectionResult{
		{Lab: "sqli", Model: "gpt-4", Final: 8},
	})
	cur := makeRun("r2", []model.ReflectionResult{
		{Lab: "xss", Model: "gpt-4", Final: 3},
	})
	r := Detect(base, cur, 0.05)
	if len(r.Regressions) != 0 {
		t.Errorf("regressions = %d, want 0 (no matching pairs)", len(r.Regressions))
	}
}

func TestScoreSeverity(t *testing.T) {
	tests := []struct {
		delta float64
		want  string
	}{
		{-0.5, "critical"},
		{-0.2, "warning"},
		{-0.05, "info"},
		{0.4, "critical"},
		{0.15, "warning"},
	}
	for _, tt := range tests {
		got := scoreSeverity(tt.delta)
		if got != tt.want {
			t.Errorf("scoreSeverity(%f) = %q, want %q", tt.delta, got, tt.want)
		}
	}
}

func TestFormatText_Regression(t *testing.T) {
	r := RegressionReport{
		BaseRun:    "r1",
		CurrentRun: "r2",
		Regressions: []Regression{
			{Model: "gpt-4", Checker: "sqli", OldScore: 0.8, NewScore: 0.3, Delta: -0.5, Severity: "critical"},
		},
		HasRegression: true,
	}
	text := FormatText(r)
	if !strings.Contains(text, "r1") || !strings.Contains(text, "r2") {
		t.Error("missing run IDs")
	}
	if !strings.Contains(text, "critical") {
		t.Error("missing severity")
	}
	if !strings.Contains(text, "Critical regressions") {
		t.Error("missing critical warning")
	}
}

func TestFormatText_NoChanges(t *testing.T) {
	r := RegressionReport{BaseRun: "r1", CurrentRun: "r2", Stable: 5}
	text := FormatText(r)
	if !strings.Contains(text, "No significant") {
		t.Error("missing no-change message")
	}
}

func TestDetect_SortOrder(t *testing.T) {
	base := makeRun("r1", []model.ReflectionResult{
		{Lab: "a", Model: "m1", Final: 8},
		{Lab: "b", Model: "m1", Final: 9},
	})
	cur := makeRun("r2", []model.ReflectionResult{
		{Lab: "a", Model: "m1", Final: 5},
		{Lab: "b", Model: "m1", Final: 2},
	})
	r := Detect(base, cur, 0.05)
	if len(r.Regressions) != 2 {
		t.Fatalf("regressions = %d, want 2", len(r.Regressions))
	}
	if r.Regressions[0].Delta > r.Regressions[1].Delta {
		t.Error("regressions should be sorted worst-first")
	}
}
