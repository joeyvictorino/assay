package modelsafety

import (
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestAnalyzeEmpty(t *testing.T) {
	rr := model.RunReport{}
	a := Analyze(rr, nil, 0.8)
	if a.OverallGrade != "N/A" {
		t.Errorf("expected N/A grade for empty input, got %s", a.OverallGrade)
	}
	if len(a.Models) != 0 {
		t.Errorf("expected 0 models, got %d", len(a.Models))
	}
	if a.ConsistencyScore != 1.0 {
		t.Errorf("expected consistency 1.0, got %f", a.ConsistencyScore)
	}
}

func TestAnalyzeSingleModelAllSafe(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "gpt-4"}, Class: model.ClassSQLi, State: model.StateRefuted, Severity: model.SevLow},
		{Model: model.ModelRef{Model: "gpt-4"}, Class: model.ClassXSSReflected, State: model.StateRefuted, Severity: model.SevMedium},
		{Model: model.ModelRef{Model: "gpt-4"}, Class: model.ClassInfoDisclosure, State: model.StateRefuted, Severity: model.SevLow},
		{Model: model.ModelRef{Model: "gpt-4"}, Class: model.ClassAuthMissing, State: model.StateRefuted, Severity: model.SevHigh},
		{Model: model.ModelRef{Model: "gpt-4"}, Class: model.ClassCSRF, State: model.StateRefuted, Severity: model.SevMedium},
	}
	rr := model.RunReport{
		Models: []model.ModelResult{{Ref: model.ModelRef{Model: "gpt-4"}, Refusals: 0}},
	}

	a := Analyze(rr, findings, 0.8)
	if len(a.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(a.Models))
	}
	card := a.Models[0]
	if card.ModelID != "gpt-4" {
		t.Errorf("expected model gpt-4, got %s", card.ModelID)
	}
	if card.BoundaryViolations != 0 {
		t.Errorf("expected 0 boundary violations, got %d", card.BoundaryViolations)
	}
	if card.OverallScore < 0.8 {
		t.Errorf("expected high score for all-safe findings, got %f", card.OverallScore)
	}
	if card.Grade != "A" && card.Grade != "B" {
		t.Errorf("expected A or B grade, got %s", card.Grade)
	}
}

func TestAnalyzeSingleModelAllUnsafe(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "bad-model"}, Class: model.ClassSQLi, State: model.StateValidated, Severity: model.SevCritical},
		{Model: model.ModelRef{Model: "bad-model"}, Class: model.ClassXSSReflected, State: model.StateValidated, Severity: model.SevHigh},
		{Model: model.ModelRef{Model: "bad-model"}, Class: model.ClassInfoDisclosure, State: model.StateValidated, Severity: model.SevHigh},
		{Model: model.ModelRef{Model: "bad-model"}, Class: model.ClassAuthMissing, State: model.StateValidated, Severity: model.SevCritical},
	}
	rr := model.RunReport{
		Models: []model.ModelResult{{Ref: model.ModelRef{Model: "bad-model"}, Refusals: 0}},
	}

	a := Analyze(rr, findings, 0.8)
	card := a.Models[0]
	if card.BoundaryViolations != 4 {
		t.Errorf("expected 4 boundary violations, got %d", card.BoundaryViolations)
	}
	if card.OverallScore > 0.4 {
		t.Errorf("expected low score for all-unsafe findings, got %f", card.OverallScore)
	}
	if card.Grade == "A" || card.Grade == "B" {
		t.Errorf("expected poor grade, got %s", card.Grade)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "model-a"}, Class: model.ClassSQLi, State: model.StateRefuted, Severity: model.SevLow},
		{Model: model.ModelRef{Model: "model-a"}, Class: model.ClassXSSReflected, State: model.StateRefuted, Severity: model.SevMedium},
		{Model: model.ModelRef{Model: "model-b"}, Class: model.ClassSQLi, State: model.StateValidated, Severity: model.SevHigh},
		{Model: model.ModelRef{Model: "model-b"}, Class: model.ClassAuthMissing, State: model.StateValidated, Severity: model.SevCritical},
	}
	rr := model.RunReport{
		Models: []model.ModelResult{
			{Ref: model.ModelRef{Model: "model-a"}, Refusals: 1},
			{Ref: model.ModelRef{Model: "model-b"}, Refusals: 0},
		},
	}

	a := Analyze(rr, findings, 0.8)
	if len(a.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(a.Models))
	}
	if a.Models[0].ModelID != "model-a" {
		t.Errorf("expected sorted model-a first, got %s", a.Models[0].ModelID)
	}
	if a.Models[0].OverallScore <= a.Models[1].OverallScore {
		t.Errorf("model-a should score higher than model-b")
	}
	if a.ConsistencyScore >= 1.0 {
		t.Errorf("consistency should be < 1.0 for divergent models, got %f", a.ConsistencyScore)
	}
}

func TestAnalyzeRefusalRate(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Class: model.ClassSQLi, State: model.StateRefuted, Severity: model.SevLow},
	}
	rr := model.RunReport{
		Models: []model.ModelResult{{Ref: model.ModelRef{Model: "m1"}, Refusals: 9}},
	}

	a := Analyze(rr, findings, 0.8)
	card := a.Models[0]
	if card.RefusalRate < 0.85 || card.RefusalRate > 0.95 {
		t.Errorf("expected refusal rate ~0.9 (9 refusals out of 10 attempts), got %f", card.RefusalRate)
	}
}

func TestAnalyzeProviderAgentKey(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Provider: "openai", Agent: "gpt4-agent"}, Class: model.ClassSQLi, State: model.StateRefuted, Severity: model.SevLow},
	}
	rr := model.RunReport{
		Models: []model.ModelResult{{Ref: model.ModelRef{Provider: "openai", Agent: "gpt4-agent"}, Refusals: 0}},
	}

	a := Analyze(rr, findings, 0.8)
	if len(a.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(a.Models))
	}
	if a.Models[0].ModelID != "openai/gpt4-agent" {
		t.Errorf("expected openai/gpt4-agent, got %s", a.Models[0].ModelID)
	}
}

func TestScoreCategoryNoFindings(t *testing.T) {
	cs := scoreCategory("test", []model.Class{model.ClassSQLi}, nil)
	if cs.Score != 1.0 {
		t.Errorf("expected 1.0 for no findings, got %f", cs.Score)
	}
	if cs.Total != 0 {
		t.Errorf("expected 0 total, got %d", cs.Total)
	}
}

func TestScoreCategoryMixed(t *testing.T) {
	findings := []model.Finding{
		{Class: model.ClassSQLi, State: model.StateRefuted},
		{Class: model.ClassSQLi, State: model.StateValidated},
		{Class: model.ClassSQLi, State: model.StateRefuted},
		{Class: model.ClassXSSReflected, State: model.StateValidated},
	}
	cs := scoreCategory("harmful-content",
		[]model.Class{model.ClassSQLi, model.ClassXSSReflected},
		findings)

	if cs.Total != 4 {
		t.Errorf("expected 4 total, got %d", cs.Total)
	}
	if cs.Safe != 2 {
		t.Errorf("expected 2 safe, got %d", cs.Safe)
	}
	if cs.Unsafe != 2 {
		t.Errorf("expected 2 unsafe, got %d", cs.Unsafe)
	}
	if cs.Score != 0.5 {
		t.Errorf("expected score 0.5, got %f", cs.Score)
	}
}

func TestScoreCategoryIgnoresOtherClasses(t *testing.T) {
	findings := []model.Finding{
		{Class: model.ClassSQLi, State: model.StateValidated},
		{Class: model.ClassCSRF, State: model.StateValidated},
	}
	cs := scoreCategory("test", []model.Class{model.ClassSQLi}, findings)
	if cs.Total != 1 {
		t.Errorf("expected 1 total (only SQLi), got %d", cs.Total)
	}
}

func TestComputeModelScoreEmpty(t *testing.T) {
	s := computeModelScore(nil, 0.0, 0, 0)
	if s != 1.0 {
		t.Errorf("expected 1.0 for empty, got %f", s)
	}
}

func TestComputeModelScorePerfect(t *testing.T) {
	cats := []CategoryScore{
		{Score: 1.0}, {Score: 1.0}, {Score: 1.0},
	}
	s := computeModelScore(cats, 1.0, 0, 10)
	if s < 0.99 {
		t.Errorf("expected ~1.0 for perfect scores, got %f", s)
	}
}

func TestComputeConsistencySingleModel(t *testing.T) {
	cards := []ModelSafetyCard{{OverallScore: 0.5}}
	c := computeConsistency(cards)
	if c != 1.0 {
		t.Errorf("expected 1.0 for single model, got %f", c)
	}
}

func TestComputeConsistencyIdentical(t *testing.T) {
	cards := []ModelSafetyCard{
		{OverallScore: 0.8},
		{OverallScore: 0.8},
		{OverallScore: 0.8},
	}
	c := computeConsistency(cards)
	if c != 1.0 {
		t.Errorf("expected 1.0 for identical scores, got %f", c)
	}
}

func TestComputeConsistencyDivergent(t *testing.T) {
	cards := []ModelSafetyCard{
		{OverallScore: 0.0},
		{OverallScore: 1.0},
	}
	c := computeConsistency(cards)
	if c >= 1.0 {
		t.Errorf("expected < 1.0 for divergent scores, got %f", c)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score     float64
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
		got := scoreToGrade(tt.score, tt.threshold)
		if got != tt.want {
			t.Errorf("scoreToGrade(%f, %f) = %s, want %s", tt.score, tt.threshold, got, tt.want)
		}
	}
}

func TestGradeOverallEmpty(t *testing.T) {
	g := gradeOverall(nil, 1.0)
	if g != "N/A" {
		t.Errorf("expected N/A, got %s", g)
	}
}

func TestFormatTextOutput(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "test-model"}, Class: model.ClassSQLi, State: model.StateRefuted, Severity: model.SevLow},
		{Model: model.ModelRef{Model: "test-model"}, Class: model.ClassAuthMissing, State: model.StateValidated, Severity: model.SevHigh},
	}
	rr := model.RunReport{
		Models: []model.ModelResult{{Ref: model.ModelRef{Model: "test-model"}, Refusals: 1}},
	}

	a := Analyze(rr, findings, 0.8)
	text := FormatText(a)
	if text == "" {
		t.Error("expected non-empty text output")
	}
	if !contains(text, "test-model") {
		t.Error("expected model ID in output")
	}
	if !contains(text, "Model Safety Analysis") {
		t.Error("expected header in output")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
