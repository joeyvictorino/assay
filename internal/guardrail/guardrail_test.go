package guardrail

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestClassifyChecker_Injection(t *testing.T) {
	if got := ClassifyChecker("prompt-injection-test"); got != "prompt_injection" {
		t.Errorf("got %q, want prompt_injection", got)
	}
}

func TestClassifyChecker_Leak(t *testing.T) {
	if got := ClassifyChecker("data-leakage-check"); got != "data_leakage" {
		t.Errorf("got %q, want data_leakage", got)
	}
}

func TestClassifyChecker_ClassBased(t *testing.T) {
	if got := ClassifyChecker("sqli"); got != "prompt_injection" {
		t.Errorf("got %q, want prompt_injection", got)
	}
}

func TestClassifyChecker_Unknown(t *testing.T) {
	if got := ClassifyChecker("zzz-mystery"); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestAnalyze_Empty(t *testing.T) {
	rr := model.RunReport{RunID: "r1"}
	r := Analyze(rr, nil)
	if len(r.Categories) != 0 {
		t.Errorf("expected 0 categories, got %d", len(r.Categories))
	}
	if r.OverallPass != 0 {
		t.Errorf("expected 0 overall pass, got %f", r.OverallPass)
	}
}

func TestAnalyze_WithReflection(t *testing.T) {
	rr := model.RunReport{
		RunID: "r1",
		Reflection: []model.ReflectionResult{
			{Lab: "prompt-injection-alpha", Passed: true, Final: 8},
			{Lab: "prompt-injection-beta", Passed: false, Final: 3, Revisions: 2},
			{Lab: "data-leakage-check", Passed: true, Final: 9},
		},
	}
	r := Analyze(rr, nil)
	if len(r.Categories) < 2 {
		t.Fatalf("expected at least 2 categories, got %d", len(r.Categories))
	}
	found := false
	for _, cat := range r.Categories {
		if cat.Name == "prompt_injection" {
			found = true
			if cat.TotalTests != 2 {
				t.Errorf("injection tests = %d, want 2", cat.TotalTests)
			}
			if cat.Passed != 1 {
				t.Errorf("injection passed = %d, want 1", cat.Passed)
			}
		}
	}
	if !found {
		t.Error("missing prompt_injection category")
	}
}

func TestAnalyze_WithFindings(t *testing.T) {
	rr := model.RunReport{RunID: "r1"}
	findings := []model.Finding{
		{Lab: "info-disclosure-test", State: model.StateValidated, Severity: model.SevHigh},
		{Lab: "info-disclosure-test", State: model.StateRefuted, Severity: model.SevLow},
		{Lab: "tool-misuse-check", State: model.StateTheorized, Severity: model.SevMedium},
	}
	r := Analyze(rr, findings)
	for _, cat := range r.Categories {
		if cat.Name == "data_leakage" {
			if cat.Passed != 0 {
				t.Errorf("data_leakage should fail (has validated finding)")
			}
		}
		if cat.Name == "tool_misuse" {
			if cat.Passed != 1 {
				t.Errorf("tool_misuse should pass (no validated findings)")
			}
		}
	}
}

func TestAnalyze_PassRate(t *testing.T) {
	rr := model.RunReport{
		RunID: "r1",
		Reflection: []model.ReflectionResult{
			{Lab: "injection-a", Passed: true, Final: 8},
			{Lab: "injection-b", Passed: true, Final: 9},
			{Lab: "leak-a", Passed: false, Final: 2},
			{Lab: "leak-b", Passed: false, Final: 1},
		},
	}
	r := Analyze(rr, nil)
	if r.OverallPass != 0.5 {
		t.Errorf("overall pass = %f, want 0.5", r.OverallPass)
	}
}

func TestAnalyze_WeakestStrongest(t *testing.T) {
	rr := model.RunReport{
		RunID: "r1",
		Reflection: []model.ReflectionResult{
			{Lab: "injection-a", Passed: true, Final: 8},
			{Lab: "leak-a", Passed: false, Final: 2},
		},
	}
	r := Analyze(rr, nil)
	if r.StrongestArea != "prompt_injection" {
		t.Errorf("strongest = %q, want prompt_injection", r.StrongestArea)
	}
	if r.WeakestArea != "data_leakage" {
		t.Errorf("weakest = %q, want data_leakage", r.WeakestArea)
	}
}

func TestFormatText(t *testing.T) {
	r := GuardrailReport{
		OverallPass:   0.75,
		StrongestArea: "prompt_injection",
		WeakestArea:   "data_leakage",
		Categories: []GuardrailCategory{
			{
				Name: "prompt_injection", PassRate: 1.0, TotalTests: 1, Passed: 1,
				Checkers: []CheckerResult{{Checker: "inj-a", Passed: true, Score: 0.8}},
			},
			{
				Name: "data_leakage", PassRate: 0.0, TotalTests: 1, Failed: 1,
				Checkers: []CheckerResult{{Checker: "leak-a", Passed: false, Score: 0.2, Details: "score 2/10"}},
			},
		},
	}
	text := FormatText(r)
	if !strings.Contains(text, "75%") {
		t.Error("missing overall pass rate")
	}
	if !strings.Contains(text, "prompt_injection") {
		t.Error("missing prompt_injection category")
	}
	if !strings.Contains(text, "PASS") {
		t.Error("missing PASS status")
	}
	if !strings.Contains(text, "FAIL") {
		t.Error("missing FAIL status")
	}
}

func TestAnalyze_CategoryOrder(t *testing.T) {
	rr := model.RunReport{
		RunID: "r1",
		Reflection: []model.ReflectionResult{
			{Lab: "bias-test", Passed: true, Final: 7},
			{Lab: "injection-test", Passed: true, Final: 9},
		},
	}
	r := Analyze(rr, nil)
	if len(r.Categories) < 2 {
		t.Fatalf("expected 2 categories, got %d", len(r.Categories))
	}
	if r.Categories[0].Name != "prompt_injection" {
		t.Errorf("first category = %q, want prompt_injection", r.Categories[0].Name)
	}
}
