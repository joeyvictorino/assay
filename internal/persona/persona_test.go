package persona

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Profiles) != 0 {
		t.Fatalf("expected 0 profiles, got %d", len(r.Profiles))
	}
	if r.OverallConsistency != 0 {
		t.Fatalf("expected 0 consistency, got %f", r.OverallConsistency)
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		{Class: model.ClassSQLi, Severity: model.SevHigh, Model: model.ModelRef{Model: "m1"}, State: model.StateValidated},
		{Class: model.ClassXSSReflected, Severity: model.SevMedium, Model: model.ModelRef{Model: "m1"}, State: model.StateValidated},
		{Class: model.ClassSQLi, Severity: model.SevCritical, Model: model.ModelRef{Model: "m1"}, State: model.StateDeclined},
	}
	r := Analyze(findings)
	if len(r.Profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(r.Profiles))
	}
	if r.Profiles[0].Model != "m1" {
		t.Errorf("expected model m1, got %s", r.Profiles[0].Model)
	}
	if r.Profiles[0].TotalRuns != 3 {
		t.Errorf("expected 3 runs, got %d", r.Profiles[0].TotalRuns)
	}
	if r.OverallConsistency != 1.0 {
		t.Errorf("expected 1.0 consistency for single model, got %f", r.OverallConsistency)
	}
	if r.Profiles[0].RefusalPct < 30 || r.Profiles[0].RefusalPct > 35 {
		t.Errorf("expected ~33%% refusal, got %.1f%%", r.Profiles[0].RefusalPct)
	}
}

func TestAnalyzeTwoModels(t *testing.T) {
	findings := []model.Finding{
		{Class: model.ClassSQLi, Severity: model.SevHigh, Model: model.ModelRef{Model: "m1"}, State: model.StateValidated},
		{Class: model.ClassXSSReflected, Severity: model.SevMedium, Model: model.ModelRef{Model: "m1"}, State: model.StateValidated},
		{Class: model.ClassSQLi, Severity: model.SevHigh, Model: model.ModelRef{Model: "m2"}, State: model.StateValidated},
		{Class: model.ClassIDOR, Severity: model.SevLow, Model: model.ModelRef{Model: "m2"}, State: model.StateValidated},
	}
	r := Analyze(findings)
	if len(r.Profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(r.Profiles))
	}
	if len(r.Drifts) != 1 {
		t.Fatalf("expected 1 drift comparison, got %d", len(r.Drifts))
	}
	d := r.Drifts[0]
	if d.PersonaScore < 0 || d.PersonaScore > 1 {
		t.Errorf("persona score out of range: %f", d.PersonaScore)
	}
	if r.OverallConsistency < 0 || r.OverallConsistency > 1 {
		t.Errorf("overall consistency out of range: %f", r.OverallConsistency)
	}
}

func TestJaccard(t *testing.T) {
	a := map[string]bool{"x": true, "y": true}
	b := map[string]bool{"y": true, "z": true}
	j := jaccard(a, b)
	if j < 0.33 || j > 0.34 {
		t.Errorf("expected ~0.33, got %f", j)
	}

	j2 := jaccard(a, a)
	if j2 != 1.0 {
		t.Errorf("expected 1.0 for identical sets, got %f", j2)
	}

	j3 := jaccard(nil, nil)
	if j3 != 1.0 {
		t.Errorf("expected 1.0 for empty sets, got %f", j3)
	}
}

func TestCosine(t *testing.T) {
	a := []float64{1, 0, 0}
	b := []float64{0, 1, 0}
	c := cosine(a, b)
	if c != 0 {
		t.Errorf("expected 0 for orthogonal, got %f", c)
	}
	c2 := cosine(a, a)
	if c2 != 1.0 {
		t.Errorf("expected 1.0 for identical, got %f", c2)
	}
}

func TestFormatText(t *testing.T) {
	findings := []model.Finding{
		{Class: model.ClassSQLi, Severity: model.SevHigh, Model: model.ModelRef{Model: "alpha"}, State: model.StateValidated},
		{Class: model.ClassSQLi, Severity: model.SevHigh, Model: model.ModelRef{Model: "beta"}, State: model.StateValidated},
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Model Persona Consistency") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "alpha") {
		t.Error("missing model alpha")
	}
	if !strings.Contains(text, "beta") {
		t.Error("missing model beta")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Report{
		Profiles:           []PersonaProfile{{Model: "m1", TotalRuns: 5}},
		OverallConsistency: 0.85,
	}
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.Profiles[0].Model != "m1" {
		t.Error("model mismatch")
	}
}
