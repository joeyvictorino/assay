package robustness

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(hash string, class model.Class, sev model.Severity, modelName string) model.Finding {
	return model.Finding{
		ID:         "f-" + hash,
		PromptHash: hash,
		Class:      class,
		Severity:   sev,
		Model:      model.ModelRef{Model: modelName},
		State:      model.StateValidated,
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
	if r.MeanScore != 0 {
		t.Fatalf("expected 0 mean, got %.1f", r.MeanScore)
	}
}

func TestAnalyzeSingleVariant(t *testing.T) {
	f := []model.Finding{
		finding("baseline-1", "sqli", model.SevHigh, "model-a"),
	}
	r := Analyze(f)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].OverallScore < 90 {
		t.Errorf("single variant should have high robustness, got %.1f", r.Models[0].OverallScore)
	}
	if r.Models[0].StdDev != 0 {
		t.Errorf("single variant should have 0 std dev, got %.2f", r.Models[0].StdDev)
	}
}

func TestAnalyzeConsistentVariants(t *testing.T) {
	f := []model.Finding{
		finding("baseline-1", "sqli", model.SevHigh, "model-a"),
		finding("rephrase-1", "sqli", model.SevHigh, "model-a"),
		finding("typo-1", "sqli", model.SevHigh, "model-a"),
	}
	r := Analyze(f)
	m := r.Models[0]
	if m.StdDev != 0 {
		t.Errorf("consistent variants should have 0 std dev, got %.2f", m.StdDev)
	}
	if m.MaxDelta != 0 {
		t.Errorf("consistent variants should have 0 max delta, got %d", m.MaxDelta)
	}
	if m.OverallScore < 90 {
		t.Errorf("consistent variants should score high, got %.1f", m.OverallScore)
	}
}

func TestAnalyzeInconsistentVariants(t *testing.T) {
	f := []model.Finding{
		finding("baseline-1", "sqli", model.SevHigh, "model-a"),
		finding("rephrase-1", "sqli", model.SevHigh, "model-a"),
		finding("rephrase-1", "xss-reflected", model.SevHigh, "model-a"),
		finding("rephrase-1", "idor", model.SevCritical, "model-a"),
	}
	r := Analyze(f)
	m := r.Models[0]
	if m.StdDev == 0 {
		t.Error("inconsistent variants should have non-zero std dev")
	}
	if m.MaxDelta == 0 {
		t.Error("inconsistent variants should have non-zero max delta")
	}
}

func TestAnalyzeMultiModel(t *testing.T) {
	f := []model.Finding{
		finding("baseline-1", "sqli", model.SevHigh, "model-a"),
		finding("rephrase-1", "sqli", model.SevHigh, "model-a"),
		finding("baseline-1", "sqli", model.SevHigh, "model-b"),
		finding("rephrase-1", "sqli", model.SevHigh, "model-b"),
		finding("rephrase-1b", "xss-reflected", model.SevHigh, "model-b"),
		finding("rephrase-1c", "idor", model.SevCritical, "model-b"),
	}
	r := Analyze(f)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.MostRobust != "model-a" {
		t.Errorf("expected model-a as most robust, got %s", r.MostRobust)
	}
	if r.LeastRobust != "model-b" {
		t.Errorf("expected model-b as least robust, got %s", r.LeastRobust)
	}
}

func TestClassifyPerturbation(t *testing.T) {
	tests := []struct {
		hash string
		want PerturbationType
	}{
		{"baseline-prompt-1", PerturbBaseline},
		{"rephrase-v2", PerturbRephrase},
		{"para-variant", PerturbRephrase},
		{"typo-test", PerturbTypo},
		{"misspell-1", PerturbTypo},
		{"adv-suffix-1", PerturbSuffix},
		{"suffix-inject", PerturbSuffix},
		{"translate-es", PerturbTranslation},
		{"lang-fr", PerturbTranslation},
		{"unknown-hash", PerturbBaseline},
	}
	for _, tt := range tests {
		f := model.Finding{PromptHash: tt.hash}
		got := classifyPerturbation(f)
		if got != tt.want {
			t.Errorf("classifyPerturbation(%q) = %s, want %s", tt.hash, got, tt.want)
		}
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{100, "A"}, {90, "A"}, {89, "B"}, {80, "B"},
		{79, "C"}, {70, "C"}, {69, "D"}, {60, "D"},
		{59, "F"}, {0, "F"},
	}
	for _, tt := range tests {
		got := scoreToGrade(tt.score)
		if got != tt.want {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestScoreFloor(t *testing.T) {
	var findings []model.Finding
	for i := 0; i < 50; i++ {
		findings = append(findings, finding("variant-"+string(rune('a'+i%26)), "sqli", model.SevCritical, "model-x"))
	}
	r := Analyze(findings)
	for _, m := range r.Models {
		if m.OverallScore < 0 {
			t.Errorf("score %.1f below zero", m.OverallScore)
		}
	}
}

func TestUnknownModel(t *testing.T) {
	f := []model.Finding{
		{ID: "f1", Class: "sqli", Severity: model.SevLow, State: model.StateValidated, PromptHash: "base"},
	}
	r := Analyze(f)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown', got %q", r.Models[0].Model)
	}
}

func TestFormatTextOutput(t *testing.T) {
	f := []model.Finding{
		finding("baseline-1", "sqli", model.SevHigh, "model-a"),
		finding("rephrase-1", "sqli", model.SevHigh, "model-a"),
	}
	r := Analyze(f)
	text := FormatText(r)
	if !strings.Contains(text, "Prompt Robustness") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "model-a") {
		t.Error("missing model name")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No robustness data") {
		t.Error("expected empty message")
	}
}

func TestFormatJSON(t *testing.T) {
	f := []model.Finding{
		finding("baseline-1", "sqli", model.SevHigh, "model-a"),
	}
	r := Analyze(f)
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(parsed.Models) != 1 {
		t.Errorf("expected 1 model, got %d", len(parsed.Models))
	}
}

func TestVariantClasses(t *testing.T) {
	f := []model.Finding{
		finding("baseline-1", "sqli", model.SevHigh, "model-a"),
		{ID: "f2", PromptHash: "baseline-1", Class: "xss-reflected", Severity: model.SevMedium,
			Model: model.ModelRef{Model: "model-a"}, State: model.StateValidated},
	}
	r := Analyze(f)
	v := r.Models[0].Variants[0]
	if len(v.Classes) != 2 {
		t.Errorf("expected 2 classes, got %d", len(v.Classes))
	}
}
