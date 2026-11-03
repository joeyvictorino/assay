package responseformat

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
)

func mkFinding(modelName, summary string) model.Finding {
	return model.Finding{
		ID:        "test-1",
		Summary:   summary,
		Model:     model.ModelRef{Model: modelName},
		FirstSeen: time.Now(),
		LastSeen:  time.Now(),
	}
}

func TestSingleModel(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "Output is well-formed JSON"),
		mkFinding("gpt-4", "Response conforms to schema"),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "gpt-4" {
		t.Errorf("expected model gpt-4, got %s", r.Models[0].Model)
	}
}

func TestMalformedOutput(t *testing.T) {
	findings := []model.Finding{
		mkFinding("test-model", "Response is malformed JSON"),
	}
	r := Analyze(findings)
	sv := r.Models[0].Dimensions[0]
	if sv.Name != "structural_validity" {
		t.Errorf("expected structural_validity dimension, got %s", sv.Name)
	}
	if sv.Score >= 80 {
		t.Errorf("expected reduced score for malformed output, got %.1f", sv.Score)
	}
}

func TestMissingField(t *testing.T) {
	findings := []model.Finding{
		mkFinding("test-model", "Response has missing field 'name'"),
	}
	r := Analyze(findings)
	sc := r.Models[0].Dimensions[1]
	if sc.Name != "schema_conformance" {
		t.Errorf("expected schema_conformance dimension, got %s", sc.Name)
	}
	if sc.Score >= 80 {
		t.Errorf("expected reduced score for missing field, got %.1f", sc.Score)
	}
}

func TestEncodingError(t *testing.T) {
	findings := []model.Finding{
		mkFinding("test-model", "Response has encoding error in unicode characters"),
	}
	r := Analyze(findings)
	ec := r.Models[0].Dimensions[2]
	if ec.Name != "encoding_correctness" {
		t.Errorf("expected encoding_correctness dimension, got %s", ec.Name)
	}
	if ec.Score >= 80 {
		t.Errorf("expected reduced score for encoding error, got %.1f", ec.Score)
	}
}

func TestMultipleModels(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "Well-formed output"),
		mkFinding("claude", "Malformed JSON response with parse error"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" || r.WorstModel == "" {
		t.Error("expected non-empty best/worst models")
	}
}

func TestEmptyFindings(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Errorf("expected 0 models for nil findings, got %d", len(r.Models))
	}
}

func TestFormatText(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "Good format"),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if text == "" {
		t.Error("expected non-empty text output")
	}
}

func TestJSONRoundTrip(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "Valid response"),
	}
	r := Analyze(findings)
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var r2 Report
	if err := json.Unmarshal(data, &r2); err != nil {
		t.Fatal(err)
	}
	if len(r2.Models) != 1 {
		t.Errorf("expected 1 model after round-trip, got %d", len(r2.Models))
	}
}
