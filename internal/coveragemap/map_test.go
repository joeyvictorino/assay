package coveragemap

import (
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestBuildMapEmpty(t *testing.T) {
	m := BuildMap(model.RunReport{}, nil)
	if len(m.Checkers) != 0 {
		t.Errorf("expected 0 checkers, got %d", len(m.Checkers))
	}
	if len(m.Gaps) != len(areas) {
		t.Errorf("expected %d gaps, got %d", len(areas), len(m.Gaps))
	}
}

func TestBuildMapWithReflection(t *testing.T) {
	rr := model.RunReport{
		Reflection: []model.ReflectionResult{
			{Lab: "lab1", Model: "m1", Reviewer: "injection-checker", Final: 8},
			{Lab: "lab2", Model: "m2", Reviewer: "bias-detector", Final: 6},
		},
	}
	m := BuildMap(rr, nil)
	if len(m.Checkers) != 2 {
		t.Errorf("expected 2 checkers, got %d", len(m.Checkers))
	}
}

func TestBuildMapWithFindings(t *testing.T) {
	findings := []model.Finding{
		{
			Model:    model.ModelRef{Provider: "leak-checker", Model: "gpt-4"},
			Summary:  "data leak detected via output",
			Severity: model.SevHigh,
		},
	}
	m := BuildMap(model.RunReport{}, findings)
	if len(m.Checkers) != 1 {
		t.Errorf("expected 1 checker, got %d", len(m.Checkers))
	}
	found := false
	for _, c := range m.Cells {
		if c.Checker == "leak-checker" && c.Area == "data_leakage" && c.Covered {
			found = true
		}
	}
	if !found {
		t.Error("expected leak-checker to cover data_leakage")
	}
}

func TestGapsAndOverlaps(t *testing.T) {
	rr := model.RunReport{
		Reflection: []model.ReflectionResult{
			{Lab: "l1", Model: "m1", Reviewer: "injection-a", Final: 7},
			{Lab: "l2", Model: "m2", Reviewer: "injection-b", Final: 6},
			{Lab: "l3", Model: "m3", Reviewer: "injection-c", Final: 5},
		},
	}
	m := BuildMap(rr, nil)
	hasOverlap := false
	for _, o := range m.Overlaps {
		if o == "prompt_injection" {
			hasOverlap = true
		}
	}
	if !hasOverlap {
		t.Error("expected prompt_injection to be an overlap area")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	m := CoverageMap{}
	out := FormatText(m)
	if out == "" {
		t.Error("expected non-empty output")
	}
}

func TestFormatTextWithData(t *testing.T) {
	m := BuildMap(model.RunReport{
		Reflection: []model.ReflectionResult{
			{Lab: "l1", Model: "m1", Reviewer: "tool-safety-checker", Final: 7},
		},
	}, nil)
	out := FormatText(m)
	if out == "" {
		t.Error("expected non-empty output")
	}
}

func TestAreasList(t *testing.T) {
	a := Areas()
	if len(a) != 10 {
		t.Errorf("expected 10 areas, got %d", len(a))
	}
}

func TestStrengthLabel(t *testing.T) {
	tests := []struct {
		count int
		want  string
	}{
		{0, "none"},
		{1, "weak"},
		{2, "moderate"},
		{5, "strong"},
		{10, "strong"},
	}
	for _, tt := range tests {
		got := strengthLabel(tt.count)
		if got != tt.want {
			t.Errorf("strengthLabel(%d) = %q, want %q", tt.count, got, tt.want)
		}
	}
}
