package promptrisk

import (
	"encoding/json"
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
	if len(r.Prompts) != 0 {
		t.Fatalf("prompts = %d", len(r.Prompts))
	}
}

func TestSingleCriticalPrompt(t *testing.T) {
	ff := []model.Finding{
		finding("p1", "m1", model.StateValidated, model.SevCritical),
		finding("p1", "m2", model.StateValidated, model.SevCritical),
		finding("p1", "m3", model.StateValidated, model.SevCritical),
	}
	r := Analyze(ff)
	if len(r.Prompts) != 1 {
		t.Fatalf("prompts = %d", len(r.Prompts))
	}
	p := r.Prompts[0]
	if p.RiskLevel != RiskCritical {
		t.Errorf("level = %s, want critical", p.RiskLevel)
	}
	if p.RiskScore < 75 {
		t.Errorf("score = %.2f, want >= 75", p.RiskScore)
	}
}

func TestLowRiskPrompt(t *testing.T) {
	ff := []model.Finding{
		finding("p1", "m1", model.StateRefuted, model.SevLow),
	}
	r := Analyze(ff)
	p := r.Prompts[0]
	if p.RiskLevel != RiskMedium {
		t.Errorf("level = %s, want medium (low sev refuted still has base risk)", p.RiskLevel)
	}
}

func TestMultiplePromptsSorted(t *testing.T) {
	ff := []model.Finding{
		finding("safe", "m1", model.StateRefuted, model.SevLow),
		finding("dangerous", "m1", model.StateValidated, model.SevCritical),
	}
	r := Analyze(ff)
	if r.Prompts[0].Hash != "dangerous" {
		t.Errorf("first = %q, want dangerous", r.Prompts[0].Hash)
	}
}

func TestDistribution(t *testing.T) {
	ff := []model.Finding{
		finding("p1", "m1", model.StateValidated, model.SevCritical),
		finding("p1", "m2", model.StateValidated, model.SevCritical),
		finding("p1", "m3", model.StateValidated, model.SevCritical),
		finding("p2", "m1", model.StateRefuted, model.SevLow),
	}
	r := Analyze(ff)
	if r.Distribution["critical"]+r.Distribution["high"]+r.Distribution["medium"]+r.Distribution["low"] != 2 {
		t.Error("distribution total wrong")
	}
}

func TestNoHashFallback(t *testing.T) {
	ff := []model.Finding{
		finding("", "m1", model.StateValidated, model.SevMedium),
	}
	r := Analyze(ff)
	if r.Prompts[0].Hash != "(no-hash)" {
		t.Errorf("hash = %q", r.Prompts[0].Hash)
	}
}

func TestValidationRate(t *testing.T) {
	ff := []model.Finding{
		finding("p1", "m1", model.StateValidated, model.SevMedium),
		finding("p1", "m1", model.StateRefuted, model.SevMedium),
	}
	r := Analyze(ff)
	if r.Prompts[0].ValidRate != 50 {
		t.Errorf("valid rate = %.2f", r.Prompts[0].ValidRate)
	}
}

func TestRefusalRate(t *testing.T) {
	ff := []model.Finding{
		finding("p1", "m1", model.StateDeclined, model.SevMedium),
		finding("p1", "m1", model.StateValidated, model.SevMedium),
	}
	r := Analyze(ff)
	if r.Prompts[0].RefuseRate != 50 {
		t.Errorf("refuse rate = %.2f", r.Prompts[0].RefuseRate)
	}
}

func TestModelCount(t *testing.T) {
	ff := []model.Finding{
		finding("p1", "m1", model.StateValidated, model.SevHigh),
		finding("p1", "m2", model.StateValidated, model.SevHigh),
		finding("p1", "m3", model.StateValidated, model.SevHigh),
	}
	r := Analyze(ff)
	if r.Prompts[0].Models != 3 {
		t.Errorf("models = %d", r.Prompts[0].Models)
	}
}

func TestScoreToLevel(t *testing.T) {
	tests := []struct {
		score float64
		want  RiskLevel
	}{
		{90, RiskCritical}, {75, RiskCritical},
		{60, RiskHigh}, {50, RiskHigh},
		{40, RiskMedium}, {25, RiskMedium},
		{15, RiskLow}, {0, RiskLow},
	}
	for _, tt := range tests {
		if got := scoreToLevel(tt.score); got != tt.want {
			t.Errorf("scoreToLevel(%.0f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestAvgRisk(t *testing.T) {
	ff := []model.Finding{
		finding("p1", "m1", model.StateValidated, model.SevCritical),
		finding("p2", "m1", model.StateRefuted, model.SevLow),
	}
	r := Analyze(ff)
	if r.AvgRisk <= 0 {
		t.Errorf("avg risk = %.2f", r.AvgRisk)
	}
}

func TestFormatTextNotEmpty(t *testing.T) {
	ff := []model.Finding{
		finding("p1", "m1", model.StateValidated, model.SevHigh),
	}
	r := Analyze(ff)
	text := FormatText(r)
	if !strings.Contains(text, "Risk Classification") {
		t.Error("missing header")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No prompts") {
		t.Error("missing empty message")
	}
}

func TestFormatJSONValid(t *testing.T) {
	ff := []model.Finding{
		finding("p1", "m1", model.StateValidated, model.SevHigh),
	}
	r := Analyze(ff)
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestCritHighCount(t *testing.T) {
	ff := []model.Finding{
		finding("p1", "m1", model.StateValidated, model.SevCritical),
		finding("p1", "m1", model.StateValidated, model.SevHigh),
		finding("p1", "m1", model.StateValidated, model.SevMedium),
	}
	r := Analyze(ff)
	if r.Prompts[0].CritHigh != 2 {
		t.Errorf("crit/high = %d, want 2", r.Prompts[0].CritHigh)
	}
}
