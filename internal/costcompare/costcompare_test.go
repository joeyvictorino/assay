package costcompare

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(m string, state model.FindingState, maxTokens int) model.Finding {
	return model.Finding{
		Model:     model.ModelRef{Model: m},
		State:     state,
		Severity:  model.SevMedium,
		MaxTokens: maxTokens,
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("models = %d", len(r.Models))
	}
}

func TestSingleModel(t *testing.T) {
	ff := []model.Finding{
		finding("m1", model.StateValidated, 1000),
		finding("m1", model.StateValidated, 2000),
	}
	r := Analyze(ff)
	if len(r.Models) != 1 {
		t.Fatalf("models = %d", len(r.Models))
	}
	m := r.Models[0]
	if m.FindingCount != 2 {
		t.Errorf("findings = %d", m.FindingCount)
	}
	if m.ValidCount != 2 {
		t.Errorf("validated = %d", m.ValidCount)
	}
	if m.TotalTokens != 3000 {
		t.Errorf("tokens = %d", m.TotalTokens)
	}
	if m.CostPerFind != 1500 {
		t.Errorf("cost/find = %.2f", m.CostPerFind)
	}
}

func TestMultipleModelsSorted(t *testing.T) {
	ff := []model.Finding{
		finding("expensive", model.StateValidated, 10000),
		finding("expensive", model.StateRefuted, 10000),
		finding("cheap", model.StateValidated, 500),
		finding("cheap", model.StateValidated, 500),
	}
	r := Analyze(ff)
	if len(r.Models) != 2 {
		t.Fatalf("models = %d", len(r.Models))
	}
	if r.Models[0].Model != "cheap" {
		t.Errorf("first model = %q, want cheap (highest efficiency)", r.Models[0].Model)
	}
}

func TestComparisons(t *testing.T) {
	ff := []model.Finding{
		finding("good", model.StateValidated, 1000),
		finding("bad", model.StateValidated, 5000),
	}
	r := Analyze(ff)
	if len(r.Comparisons) < 1 {
		t.Fatal("no comparisons")
	}
	if r.Comparisons[0].Better != "good" {
		t.Errorf("better = %q", r.Comparisons[0].Better)
	}
}

func TestNoTokensZeroCost(t *testing.T) {
	ff := []model.Finding{
		finding("m1", model.StateValidated, 0),
	}
	r := Analyze(ff)
	if r.Models[0].CostPerFind != 0 {
		t.Errorf("cost = %.2f, want 0 with no tokens", r.Models[0].CostPerFind)
	}
}

func TestNoValidatedZeroCostPerValid(t *testing.T) {
	ff := []model.Finding{
		finding("m1", model.StateRefuted, 1000),
	}
	r := Analyze(ff)
	if r.Models[0].CostPerValid != 0 {
		t.Errorf("cost/valid = %.2f, want 0", r.Models[0].CostPerValid)
	}
}

func TestEfficiencyZeroTokens(t *testing.T) {
	ff := []model.Finding{
		finding("m1", model.StateValidated, 0),
	}
	r := Analyze(ff)
	if r.Models[0].Efficiency != 0 {
		t.Errorf("efficiency = %.2f, want 0", r.Models[0].Efficiency)
	}
}

func TestFormatTextNotEmpty(t *testing.T) {
	ff := []model.Finding{
		finding("m1", model.StateValidated, 1000),
	}
	r := Analyze(ff)
	text := FormatText(r)
	if !strings.Contains(text, "Cost Comparison") {
		t.Error("missing header")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No model data") {
		t.Error("missing empty message")
	}
}

func TestFormatJSONValid(t *testing.T) {
	ff := []model.Finding{
		finding("m1", model.StateValidated, 1000),
	}
	r := Analyze(ff)
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestSingleModelNoComparison(t *testing.T) {
	ff := []model.Finding{
		finding("m1", model.StateValidated, 1000),
	}
	r := Analyze(ff)
	if len(r.Comparisons) != 0 {
		t.Errorf("comparisons = %d, want 0", len(r.Comparisons))
	}
}

func TestCostPerValidated(t *testing.T) {
	ff := []model.Finding{
		finding("m1", model.StateValidated, 1000),
		finding("m1", model.StateRefuted, 2000),
	}
	r := Analyze(ff)
	if r.Models[0].CostPerValid != 3000 {
		t.Errorf("cost/valid = %.2f, want 3000", r.Models[0].CostPerValid)
	}
}
