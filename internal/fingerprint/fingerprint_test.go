package fingerprint

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(m string, state model.FindingState, sev model.Severity, class model.Class, summary string) model.Finding {
	return model.Finding{
		Model:    model.ModelRef{Model: m},
		State:    state,
		Severity: sev,
		Class:    class,
		Summary:  summary,
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Fingerprints) != 0 {
		t.Fatalf("fingerprints = %d", len(r.Fingerprints))
	}
}

func TestSingleModel(t *testing.T) {
	ff := []model.Finding{
		finding("m1", model.StateValidated, model.SevHigh, "injection", "found injection"),
		finding("m1", model.StateValidated, model.SevMedium, "ssrf", "found ssrf issue"),
	}
	r := Analyze(ff)
	if len(r.Fingerprints) != 1 {
		t.Fatalf("fingerprints = %d", len(r.Fingerprints))
	}
	fp := r.Fingerprints[0]
	if fp.Model != "m1" {
		t.Errorf("model = %q", fp.Model)
	}
	if fp.Count != 2 {
		t.Errorf("count = %d", fp.Count)
	}
	if fp.Style.ValidRate != 1.0 {
		t.Errorf("valid rate = %.2f", fp.Style.ValidRate)
	}
}

func TestMultipleModelsComparison(t *testing.T) {
	ff := []model.Finding{
		finding("m1", model.StateValidated, model.SevHigh, "injection", "a"),
		finding("m2", model.StateValidated, model.SevHigh, "injection", "b"),
	}
	r := Analyze(ff)
	if len(r.Fingerprints) != 2 {
		t.Fatalf("fingerprints = %d", len(r.Fingerprints))
	}
	if len(r.Comparisons) != 1 {
		t.Fatalf("comparisons = %d", len(r.Comparisons))
	}
	c := r.Comparisons[0]
	if c.Similarity != 1.0 {
		t.Errorf("identical distributions should have similarity 1.0, got %.2f", c.Similarity)
	}
}

func TestThreeModelsComparisons(t *testing.T) {
	ff := []model.Finding{
		finding("a", model.StateValidated, model.SevHigh, "c1", "x"),
		finding("b", model.StateValidated, model.SevMedium, "c2", "y"),
		finding("c", model.StateValidated, model.SevLow, "c3", "z"),
	}
	r := Analyze(ff)
	if len(r.Comparisons) != 3 {
		t.Errorf("comparisons = %d, want 3", len(r.Comparisons))
	}
}

func TestAvgSummaryLen(t *testing.T) {
	ff := []model.Finding{
		finding("m1", model.StateValidated, model.SevHigh, "c1", "12345"),
		finding("m1", model.StateValidated, model.SevHigh, "c1", "12345678901234567890"),
	}
	r := Analyze(ff)
	want := 12.5
	if r.Fingerprints[0].Style.AvgSummaryLen != want {
		t.Errorf("avg summary len = %.2f, want %.2f", r.Fingerprints[0].Style.AvgSummaryLen, want)
	}
}

func TestRefusalRate(t *testing.T) {
	ff := []model.Finding{
		finding("m1", model.StateDeclined, model.SevMedium, "c1", ""),
		finding("m1", model.StateValidated, model.SevMedium, "c1", "ok"),
	}
	r := Analyze(ff)
	if r.Fingerprints[0].Style.RefuseRate != 0.5 {
		t.Errorf("refuse rate = %.2f", r.Fingerprints[0].Style.RefuseRate)
	}
}

func TestShannonEntropy(t *testing.T) {
	uniform := map[model.Severity]int{
		model.SevCritical: 25,
		model.SevHigh:     25,
		model.SevMedium:   25,
		model.SevLow:      25,
	}
	e := shannonEntropy(uniform)
	if math.Abs(e-2.0) > 0.01 {
		t.Errorf("uniform entropy = %.2f, want 2.0", e)
	}

	single := map[model.Severity]int{model.SevHigh: 100}
	e2 := shannonEntropy(single)
	if e2 != 0 {
		t.Errorf("single-value entropy = %.2f, want 0", e2)
	}
}

func TestShannonEntropyEmpty(t *testing.T) {
	e := shannonEntropy(map[string]int{})
	if e != 0 {
		t.Errorf("empty entropy = %.2f", e)
	}
}

func TestCosineSimilarityIdentical(t *testing.T) {
	a := map[string]float64{"high": 0.5, "low": 0.5}
	if s := cosineSimilarity(a, a); math.Abs(s-1.0) > 0.001 {
		t.Errorf("identical similarity = %.2f", s)
	}
}

func TestCosineSimilarityOrthogonal(t *testing.T) {
	a := map[string]float64{"high": 1.0}
	b := map[string]float64{"low": 1.0}
	if s := cosineSimilarity(a, b); s != 0 {
		t.Errorf("orthogonal similarity = %.2f", s)
	}
}

func TestCosineSimilarityEmpty(t *testing.T) {
	if s := cosineSimilarity(nil, nil); s != 0 {
		t.Errorf("empty similarity = %.2f", s)
	}
}

func TestFormatTextNotEmpty(t *testing.T) {
	ff := []model.Finding{
		finding("m1", model.StateValidated, model.SevHigh, "c1", "test"),
	}
	r := Analyze(ff)
	text := FormatText(r)
	if !strings.Contains(text, "Fingerprint") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "m1") {
		t.Error("missing model")
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
		finding("m1", model.StateValidated, model.SevHigh, "c1", "test"),
	}
	r := Analyze(ff)
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestSortedByModelName(t *testing.T) {
	ff := []model.Finding{
		finding("z-model", model.StateValidated, model.SevHigh, "c1", "x"),
		finding("a-model", model.StateValidated, model.SevMedium, "c2", "y"),
	}
	r := Analyze(ff)
	if r.Fingerprints[0].Model != "a-model" {
		t.Errorf("first model = %q, want a-model", r.Fingerprints[0].Model)
	}
}

func TestSeverityDistribution(t *testing.T) {
	ff := []model.Finding{
		finding("m1", model.StateValidated, model.SevHigh, "c1", "a"),
		finding("m1", model.StateValidated, model.SevHigh, "c1", "b"),
		finding("m1", model.StateValidated, model.SevLow, "c1", "c"),
	}
	r := Analyze(ff)
	sevDist := r.Fingerprints[0].Style.SevDist
	if sevDist["high"] != 0.67 {
		t.Errorf("high dist = %.2f, want 0.67", sevDist["high"])
	}
}
