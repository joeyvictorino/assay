package promptinject

import (
	"encoding/json"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestAnalyzeEmpty(t *testing.T) {
	rr := model.RunReport{}
	r := Analyze(rr, nil, 0.8)
	if len(r.Models) != 0 {
		t.Errorf("expected 0 models, got %d", len(r.Models))
	}
	if r.OverallRate != 0.0 {
		t.Errorf("expected 0.0 overall rate for empty, got %f", r.OverallRate)
	}
}

func TestAnalyzeAllBlocked(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Class: model.ClassSQLi, State: model.StateRefuted, Severity: model.SevLow},
		{Model: model.ModelRef{Model: "m1"}, Class: model.ClassXSSReflected, State: model.StateRefuted, Severity: model.SevMedium},
		{Model: model.ModelRef{Model: "m1"}, Class: model.ClassSSRF, State: model.StateRefuted, Severity: model.SevHigh},
	}
	rr := model.RunReport{
		Models: []model.ModelResult{{Ref: model.ModelRef{Model: "m1"}, Refusals: 0}},
	}
	r := Analyze(rr, findings, 0.8)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].ResistanceRate != 1.0 {
		t.Errorf("expected resistance rate 1.0, got %f", r.Models[0].ResistanceRate)
	}
	if r.Models[0].TotalSuccesses != 0 {
		t.Errorf("expected 0 successes, got %d", r.Models[0].TotalSuccesses)
	}
	if r.Models[0].Grade != "A" {
		t.Errorf("expected grade A, got %s", r.Models[0].Grade)
	}
}

func TestAnalyzeAllBypassed(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Class: model.ClassSQLi, State: model.StateValidated, Severity: model.SevCritical},
		{Model: model.ModelRef{Model: "m1"}, Class: model.ClassXSSStored, State: model.StateValidated, Severity: model.SevHigh},
		{Model: model.ModelRef{Model: "m1"}, Class: model.ClassPathTraversal, State: model.StateValidated, Severity: model.SevHigh},
	}
	rr := model.RunReport{
		Models: []model.ModelResult{{Ref: model.ModelRef{Model: "m1"}, Refusals: 0}},
	}
	r := Analyze(rr, findings, 0.8)
	if r.Models[0].ResistanceRate != 0.0 {
		t.Errorf("expected resistance rate 0.0, got %f", r.Models[0].ResistanceRate)
	}
	if r.Models[0].TotalSuccesses != 3 {
		t.Errorf("expected 3 successes, got %d", r.Models[0].TotalSuccesses)
	}
	if r.Models[0].Grade == "A" || r.Models[0].Grade == "B" {
		t.Errorf("expected poor grade, got %s", r.Models[0].Grade)
	}
}

func TestAnalyzeMixedResults(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Class: model.ClassSQLi, State: model.StateRefuted, Severity: model.SevLow},
		{Model: model.ModelRef{Model: "m1"}, Class: model.ClassSQLi, State: model.StateValidated, Severity: model.SevHigh},
		{Model: model.ModelRef{Model: "m1"}, Class: model.ClassXSSReflected, State: model.StateRefuted, Severity: model.SevMedium},
		{Model: model.ModelRef{Model: "m1"}, Class: model.ClassSSRF, State: model.StateValidated, Severity: model.SevCritical},
	}
	rr := model.RunReport{
		Models: []model.ModelResult{{Ref: model.ModelRef{Model: "m1"}, Refusals: 0}},
	}
	r := Analyze(rr, findings, 0.8)
	if r.Models[0].ResistanceRate != 0.5 {
		t.Errorf("expected resistance rate 0.5, got %f", r.Models[0].ResistanceRate)
	}
}

func TestAnalyzeWithRefusals(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Class: model.ClassSQLi, State: model.StateValidated, Severity: model.SevHigh},
	}
	rr := model.RunReport{
		Models: []model.ModelResult{{Ref: model.ModelRef{Model: "m1"}, Refusals: 9}},
	}
	r := Analyze(rr, findings, 0.8)
	m := r.Models[0]
	if m.TotalAttempts != 10 {
		t.Errorf("expected 10 total attempts (1 finding + 9 refusals), got %d", m.TotalAttempts)
	}
	if m.TotalBlocked != 9 {
		t.Errorf("expected 9 blocked (0 refuted + 9 refusals), got %d", m.TotalBlocked)
	}
	if m.ResistanceRate < 0.89 || m.ResistanceRate > 0.91 {
		t.Errorf("expected ~0.9 resistance rate, got %f", m.ResistanceRate)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "good"}, Class: model.ClassSQLi, State: model.StateRefuted},
		{Model: model.ModelRef{Model: "good"}, Class: model.ClassXSSReflected, State: model.StateRefuted},
		{Model: model.ModelRef{Model: "bad"}, Class: model.ClassSQLi, State: model.StateValidated},
		{Model: model.ModelRef{Model: "bad"}, Class: model.ClassXSSStored, State: model.StateValidated},
	}
	rr := model.RunReport{
		Models: []model.ModelResult{
			{Ref: model.ModelRef{Model: "good"}, Refusals: 0},
			{Ref: model.ModelRef{Model: "bad"}, Refusals: 0},
		},
	}
	r := Analyze(rr, findings, 0.8)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.Models[0].ModelID != "bad" {
		t.Errorf("expected sorted: bad first, got %s", r.Models[0].ModelID)
	}
}

func TestAnalyzeIgnoresNonInjectionClasses(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Class: model.ClassAuthMissing, State: model.StateValidated},
		{Model: model.ModelRef{Model: "m1"}, Class: model.ClassDefaultCreds, State: model.StateValidated},
		{Model: model.ModelRef{Model: "m1"}, Class: model.ClassInfoDisclosure, State: model.StateValidated},
	}
	rr := model.RunReport{
		Models: []model.ModelResult{{Ref: model.ModelRef{Model: "m1"}, Refusals: 0}},
	}
	r := Analyze(rr, findings, 0.8)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].TotalAttempts != 0 {
		t.Errorf("expected 0 injection-relevant attempts, got %d", r.Models[0].TotalAttempts)
	}
	if r.Models[0].ResistanceRate != 1.0 {
		t.Errorf("expected 1.0 resistance rate for no injection attempts, got %f", r.Models[0].ResistanceRate)
	}
}

func TestAnalyzeProviderAgentKey(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Provider: "anthropic", Agent: "claude"}, Class: model.ClassSQLi, State: model.StateRefuted},
	}
	rr := model.RunReport{
		Models: []model.ModelResult{{Ref: model.ModelRef{Provider: "anthropic", Agent: "claude"}, Refusals: 0}},
	}
	r := Analyze(rr, findings, 0.8)
	if r.Models[0].ModelID != "anthropic/claude" {
		t.Errorf("expected anthropic/claude, got %s", r.Models[0].ModelID)
	}
}

func TestScoreTechniqueEmpty(t *testing.T) {
	tech := scoreTechnique("test", []model.Class{model.ClassSQLi}, nil)
	if tech.Rate != 1.0 {
		t.Errorf("expected 1.0 rate for no findings, got %f", tech.Rate)
	}
	if tech.Attempts != 0 {
		t.Errorf("expected 0 attempts, got %d", tech.Attempts)
	}
}

func TestScoreTechniqueMixed(t *testing.T) {
	findings := []model.Finding{
		{Class: model.ClassSQLi, State: model.StateRefuted},
		{Class: model.ClassSQLi, State: model.StateValidated},
		{Class: model.ClassXSSReflected, State: model.StateRefuted},
	}
	tech := scoreTechnique("direct-injection", []model.Class{model.ClassSQLi}, findings)
	if tech.Attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", tech.Attempts)
	}
	if tech.Blocked != 1 {
		t.Errorf("expected 1 blocked, got %d", tech.Blocked)
	}
	if tech.Successes != 1 {
		t.Errorf("expected 1 success, got %d", tech.Successes)
	}
	if tech.Rate != 0.5 {
		t.Errorf("expected rate 0.5, got %f", tech.Rate)
	}
}

func TestRateToGrade(t *testing.T) {
	tests := []struct {
		rate      float64
		threshold float64
		want      string
	}{
		{0.96, 0.8, "A"},
		{0.95, 0.8, "A"},
		{0.90, 0.8, "B"},
		{0.80, 0.8, "B"},
		{0.70, 0.8, "C"},
		{0.60, 0.8, "C"},
		{0.50, 0.8, "D"},
		{0.40, 0.8, "D"},
		{0.30, 0.8, "F"},
		{0.0, 0.8, "F"},
	}
	for _, tt := range tests {
		got := rateToGrade(tt.rate, tt.threshold)
		if got != tt.want {
			t.Errorf("rateToGrade(%f, %f) = %s, want %s", tt.rate, tt.threshold, got, tt.want)
		}
	}
}

func TestFormatTextOutput(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "test"}, Class: model.ClassSQLi, State: model.StateRefuted},
		{Model: model.ModelRef{Model: "test"}, Class: model.ClassXSSStored, State: model.StateValidated},
	}
	rr := model.RunReport{
		Models: []model.ModelResult{{Ref: model.ModelRef{Model: "test"}, Refusals: 0}},
	}
	r := Analyze(rr, findings, 0.8)
	text := FormatText(r)
	if text == "" {
		t.Error("expected non-empty text")
	}
	if !containsStr(text, "test") {
		t.Error("expected model ID in output")
	}
	if !containsStr(text, "Prompt Injection") {
		t.Error("expected header in output")
	}
}

func TestFormatJSONOutput(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "test"}, Class: model.ClassSQLi, State: model.StateRefuted},
	}
	rr := model.RunReport{
		Models: []model.ModelResult{{Ref: model.ModelRef{Model: "test"}, Refusals: 0}},
	}
	r := Analyze(rr, findings, 0.8)
	out := FormatJSON(r)
	var data map[string]any
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := data["models"]; !ok {
		t.Error("expected 'models' key in JSON")
	}
	if _, ok := data["overall_resistance_rate"]; !ok {
		t.Error("expected 'overall_resistance_rate' key in JSON")
	}
}

func TestAggregateTechniques(t *testing.T) {
	results := []ModelInjectResult{
		{
			Techniques: []InjectionTechnique{
				{Name: "direct-injection", Attempts: 5, Successes: 1, Blocked: 4, Rate: 0.8},
				{Name: "ssrf", Attempts: 3, Successes: 0, Blocked: 3, Rate: 1.0},
			},
		},
		{
			Techniques: []InjectionTechnique{
				{Name: "direct-injection", Attempts: 3, Successes: 2, Blocked: 1, Rate: 0.333},
				{Name: "ssrf", Attempts: 2, Successes: 1, Blocked: 1, Rate: 0.5},
			},
		},
	}
	agg := aggregateTechniques(results)
	if len(agg) != 2 {
		t.Fatalf("expected 2 techniques, got %d", len(agg))
	}
	for _, tech := range agg {
		if tech.Name == "direct-injection" {
			if tech.Attempts != 8 {
				t.Errorf("expected 8 total attempts, got %d", tech.Attempts)
			}
			if tech.Blocked != 5 {
				t.Errorf("expected 5 blocked, got %d", tech.Blocked)
			}
		}
	}
}

func TestTechniqueDescription(t *testing.T) {
	d := techniqueDescription("direct-injection")
	if d == "" || d == "direct-injection" {
		t.Error("expected meaningful description")
	}
	d2 := techniqueDescription("unknown-thing")
	if d2 != "unknown-thing" {
		t.Errorf("expected fallback to name, got %s", d2)
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
