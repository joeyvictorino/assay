package outputformat

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil, nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
	if r.OverallScore != 0 {
		t.Fatalf("expected 0 score, got %f", r.OverallScore)
	}
}

func TestAnalyzeCompliant(t *testing.T) {
	findings := []model.Finding{
		{
			ID:       "f1",
			Class:    model.ClassSQLi,
			Severity: model.SevHigh,
			State:    model.StateValidated,
			Summary:  "SQL injection in login",
			Location: model.Location{Method: "POST", PathTemplate: "/login"},
			Model:    model.ModelRef{Model: "m1"},
		},
	}
	r := Analyze(findings, nil)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Score != 1.0 {
		t.Errorf("expected 1.0 compliance, got %f", r.Models[0].Score)
	}
	if r.OverallScore != 1.0 {
		t.Errorf("expected 1.0 overall, got %f", r.OverallScore)
	}
}

func TestAnalyzeMissingFields(t *testing.T) {
	findings := []model.Finding{
		{
			Class:    model.ClassSQLi,
			Severity: model.SevHigh,
			State:    model.StateValidated,
			Model:    model.ModelRef{Model: "m1"},
		},
	}
	r := Analyze(findings, nil)
	if r.Models[0].Score >= 1.0 {
		t.Errorf("expected <1.0 compliance for missing fields, got %f", r.Models[0].Score)
	}
	if len(r.Models[0].Issues) == 0 {
		t.Error("expected issues for missing fields")
	}
}

func TestAnalyzeCustomFields(t *testing.T) {
	findings := []model.Finding{
		{
			ID:    "f1",
			CWE:   "CWE-89",
			Model: model.ModelRef{Model: "m1"},
		},
	}
	r := Analyze(findings, []string{"id", "cwe"})
	if r.Models[0].Score != 1.0 {
		t.Errorf("expected 1.0 for custom fields, got %f", r.Models[0].Score)
	}
}

func TestAnalyzeMultiModel(t *testing.T) {
	findings := []model.Finding{
		{ID: "f1", Class: model.ClassSQLi, Severity: model.SevHigh, State: model.StateValidated, Summary: "s", Location: model.Location{Method: "GET"}, Model: model.ModelRef{Model: "m1"}},
		{Class: model.ClassXSSReflected, Model: model.ModelRef{Model: "m2"}},
	}
	r := Analyze(findings, nil)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	m1 := r.Models[0]
	m2 := r.Models[1]
	if m1.Model == "m2" {
		m1, m2 = m2, m1
	}
	if m1.Score <= m2.Score {
		t.Errorf("m1 should be more compliant than m2: %f vs %f", m1.Score, m2.Score)
	}
}

func TestHasField(t *testing.T) {
	f := model.Finding{
		ID:       "x",
		Class:    model.ClassSQLi,
		CWE:      "CWE-89",
		Evidence: []model.Evidence{{Kind: "http-exchange"}},
	}
	if !hasField(f, "id") {
		t.Error("expected id present")
	}
	if !hasField(f, "cwe") {
		t.Error("expected cwe present")
	}
	if !hasField(f, "evidence") {
		t.Error("expected evidence present")
	}
	if hasField(f, "summary") {
		t.Error("expected summary absent")
	}
	if hasField(f, "unknown_field") {
		t.Error("expected unknown field absent")
	}
}

func TestFormatText(t *testing.T) {
	r := Analyze([]model.Finding{
		{ID: "f1", Class: model.ClassSQLi, Severity: model.SevHigh, State: model.StateValidated, Summary: "s", Location: model.Location{Method: "GET"}, Model: model.ModelRef{Model: "alpha"}},
	}, nil)
	text := FormatText(r)
	if !strings.Contains(text, "Output Format Compliance") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "alpha") {
		t.Error("missing model name")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Report{
		Models:       []ModelCompliance{{Model: "m1", Total: 3, Score: 0.9}},
		OverallScore: 0.9,
	}
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.Models[0].Model != "m1" {
		t.Error("model mismatch")
	}
}
