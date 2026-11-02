package reliability

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0, got %d", len(r.Models))
	}
}

func TestAnalyzeNoRefusals(t *testing.T) {
	findings := []model.Finding{
		{State: model.StateValidated, Severity: model.SevHigh, Model: model.ModelRef{Model: "m1"}, PromptHash: "p1"},
		{State: model.StateValidated, Severity: model.SevHigh, Model: model.ModelRef{Model: "m1"}, PromptHash: "p1"},
		{State: model.StateValidated, Severity: model.SevHigh, Model: model.ModelRef{Model: "m1"}, PromptHash: "p1"},
	}
	r := Analyze(findings)
	if r.Models[0].RefusalRate != 0 {
		t.Errorf("expected 0 refusal rate, got %f", r.Models[0].RefusalRate)
	}
	if r.Models[0].StateStability != 1.0 {
		t.Errorf("expected 1.0 state stability, got %f", r.Models[0].StateStability)
	}
	if r.Models[0].Score < 0.9 {
		t.Errorf("expected high score, got %f", r.Models[0].Score)
	}
}

func TestAnalyzeHighRefusal(t *testing.T) {
	findings := []model.Finding{
		{State: model.StateDeclined, Severity: model.SevHigh, Model: model.ModelRef{Model: "m1"}},
		{State: model.StateDeclined, Severity: model.SevHigh, Model: model.ModelRef{Model: "m1"}},
		{State: model.StateValidated, Severity: model.SevHigh, Model: model.ModelRef{Model: "m1"}},
	}
	r := Analyze(findings)
	if r.Models[0].RefusalRate < 0.6 {
		t.Errorf("expected high refusal rate, got %f", r.Models[0].RefusalRate)
	}
}

func TestAnalyzeMultiModel(t *testing.T) {
	findings := []model.Finding{
		{State: model.StateValidated, Model: model.ModelRef{Model: "m1"}},
		{State: model.StateRefuted, Model: model.ModelRef{Model: "m1"}},
		{State: model.StateValidated, Model: model.ModelRef{Model: "m2"}},
		{State: model.StateValidated, Model: model.ModelRef{Model: "m2"}},
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	m1 := r.Models[0]
	m2 := r.Models[1]
	if m1.Model == "m2" {
		m1, m2 = m2, m1
	}
	if m2.StateStability < m1.StateStability {
		t.Errorf("m2 should be more stable: m1=%.2f m2=%.2f", m1.StateStability, m2.StateStability)
	}
}

func TestDominance(t *testing.T) {
	counts := map[model.FindingState]int{
		model.StateValidated: 3,
		model.StateRefuted:   1,
	}
	d := dominance(counts, 4)
	if d != 0.75 {
		t.Errorf("expected 0.75, got %f", d)
	}
}

func TestFormatText(t *testing.T) {
	r := Analyze([]model.Finding{
		{State: model.StateValidated, Model: model.ModelRef{Model: "alpha"}},
	})
	text := FormatText(r)
	if !strings.Contains(text, "Model Reliability") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "alpha") {
		t.Error("missing model")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Report{Models: []ModelReliability{{Model: "m1", Score: 0.8}}, OverallScore: 0.8}
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}
