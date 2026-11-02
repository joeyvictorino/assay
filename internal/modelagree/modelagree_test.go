package modelagree

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(hash, mdl string, state model.FindingState, sev model.Severity) model.Finding {
	return model.Finding{
		PromptHash: hash,
		Model:      model.ModelRef{Model: mdl},
		State:      state,
		Severity:   sev,
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Pairs) != 0 {
		t.Fatalf("pairs = %d", len(r.Pairs))
	}
	if r.Overall != 0 {
		t.Errorf("overall = %.2f", r.Overall)
	}
}

func TestTwoModelsFullAgreement(t *testing.T) {
	ff := []model.Finding{
		finding("p1", "m1", model.StateValidated, model.SevHigh),
		finding("p1", "m2", model.StateValidated, model.SevHigh),
		finding("p2", "m1", model.StateRefuted, model.SevLow),
		finding("p2", "m2", model.StateRefuted, model.SevLow),
	}
	r := Analyze(ff)
	if len(r.Pairs) != 1 {
		t.Fatalf("pairs = %d", len(r.Pairs))
	}
	if r.Pairs[0].Agreement != 100 {
		t.Errorf("agreement = %.2f", r.Pairs[0].Agreement)
	}
}

func TestTwoModelsDisagreement(t *testing.T) {
	ff := []model.Finding{
		finding("p1", "m1", model.StateValidated, model.SevHigh),
		finding("p1", "m2", model.StateRefuted, model.SevLow),
	}
	r := Analyze(ff)
	if r.Pairs[0].Agreement != 0 {
		t.Errorf("agreement = %.2f, want 0", r.Pairs[0].Agreement)
	}
}

func TestThreeModels(t *testing.T) {
	ff := []model.Finding{
		finding("p1", "a", model.StateValidated, model.SevHigh),
		finding("p1", "b", model.StateValidated, model.SevHigh),
		finding("p1", "c", model.StateValidated, model.SevHigh),
	}
	r := Analyze(ff)
	if len(r.Pairs) != 3 {
		t.Errorf("pairs = %d, want 3", len(r.Pairs))
	}
}

func TestNoSharedPrompts(t *testing.T) {
	ff := []model.Finding{
		finding("p1", "m1", model.StateValidated, model.SevHigh),
		finding("p2", "m2", model.StateValidated, model.SevHigh),
	}
	r := Analyze(ff)
	if len(r.Pairs) != 0 {
		t.Errorf("pairs = %d, want 0", len(r.Pairs))
	}
}

func TestEmptyHashSkipped(t *testing.T) {
	ff := []model.Finding{
		finding("", "m1", model.StateValidated, model.SevHigh),
		finding("", "m2", model.StateValidated, model.SevHigh),
	}
	r := Analyze(ff)
	if len(r.Pairs) != 0 {
		t.Errorf("pairs = %d, want 0 (empty hashes skipped)", len(r.Pairs))
	}
}

func TestPearsonPerfect(t *testing.T) {
	x := []float64{1, 2, 3, 4}
	y := []float64{1, 2, 3, 4}
	if p := pearson(x, y); math.Abs(p-1.0) > 0.001 {
		t.Errorf("pearson = %.2f, want 1.0", p)
	}
}

func TestPearsonInverse(t *testing.T) {
	x := []float64{1, 2, 3, 4}
	y := []float64{4, 3, 2, 1}
	if p := pearson(x, y); math.Abs(p+1.0) > 0.001 {
		t.Errorf("pearson = %.2f, want -1.0", p)
	}
}

func TestPearsonConstant(t *testing.T) {
	x := []float64{3, 3, 3}
	y := []float64{1, 2, 3}
	if p := pearson(x, y); p != 0 {
		t.Errorf("pearson = %.2f, want 0", p)
	}
}

func TestPearsonTooShort(t *testing.T) {
	if p := pearson([]float64{1}, []float64{2}); p != 0 {
		t.Errorf("pearson single = %.2f", p)
	}
}

func TestSummariesSorted(t *testing.T) {
	ff := []model.Finding{
		finding("p1", "good", model.StateValidated, model.SevHigh),
		finding("p1", "bad", model.StateRefuted, model.SevLow),
		finding("p2", "good", model.StateValidated, model.SevHigh),
		finding("p2", "bad", model.StateValidated, model.SevHigh),
	}
	r := Analyze(ff)
	if len(r.Summaries) != 2 {
		t.Fatalf("summaries = %d", len(r.Summaries))
	}
}

func TestFormatTextNotEmpty(t *testing.T) {
	ff := []model.Finding{
		finding("p1", "m1", model.StateValidated, model.SevHigh),
		finding("p1", "m2", model.StateValidated, model.SevHigh),
	}
	r := Analyze(ff)
	text := FormatText(r)
	if !strings.Contains(text, "Agreement") {
		t.Error("missing header")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No shared prompts") {
		t.Error("missing empty message")
	}
}

func TestFormatJSONValid(t *testing.T) {
	ff := []model.Finding{
		finding("p1", "m1", model.StateValidated, model.SevHigh),
		finding("p1", "m2", model.StateValidated, model.SevHigh),
	}
	r := Analyze(ff)
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestOverall(t *testing.T) {
	ff := []model.Finding{
		finding("p1", "a", model.StateValidated, model.SevHigh),
		finding("p1", "b", model.StateValidated, model.SevHigh),
	}
	r := Analyze(ff)
	if r.Overall != 100 {
		t.Errorf("overall = %.2f, want 100", r.Overall)
	}
}
