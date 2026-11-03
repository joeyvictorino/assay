package selfcorrect

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func mkFinding(id, mdl, summary string, sev model.Severity, state model.FindingState) model.Finding {
	return model.Finding{
		ID:       id,
		Model:    model.ModelRef{Model: mdl},
		Summary:  summary,
		Severity: sev,
		State:    state,
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
}

func TestAnalyzeNoCorrections(t *testing.T) {
	findings := []model.Finding{
		mkFinding("f1", "gpt-4", "found sqli vulnerability", model.SevHigh, model.StateValidated),
		mkFinding("f2", "gpt-4", "found xss issue", model.SevMedium, model.StateValidated),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].CorrectionCount != 0 {
		t.Errorf("expected 0 corrections, got %d", r.Models[0].CorrectionCount)
	}
}

func TestAnalyzeWithCorrections(t *testing.T) {
	findings := []model.Finding{
		mkFinding("f1", "gpt-4", "found sqli issue", model.SevHigh, model.StateTheorized),
		mkFinding("f2", "gpt-4", "actually, let me reconsider the sqli finding", model.SevMedium, model.StateValidated),
	}
	r := Analyze(findings)
	if r.Models[0].CorrectionCount != 1 {
		t.Errorf("expected 1 correction, got %d", r.Models[0].CorrectionCount)
	}
	if r.Models[0].CorrectionRate <= 0 {
		t.Error("expected positive correction rate")
	}
}

func TestDetectCorrectionMarkers(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"actually this is wrong", true},
		{"let me reconsider the evidence", true},
		{"correction: the severity should be high", true},
		{"upon reflection the finding is valid", true},
		{"normal finding summary", false},
		{"found a vulnerability in the auth module", false},
	}
	for _, tt := range tests {
		got := detectCorrection(tt.text)
		if (got != "") != tt.want {
			t.Errorf("detectCorrection(%q) = %q, want match=%v", tt.text, got, tt.want)
		}
	}
}

func TestDetectOvercorrection(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"actually, my first answer was correct after all", true},
		{"i was right the first time about the sqli", true},
		{"going back to my original assessment", true},
		{"the finding is confirmed", false},
	}
	for _, tt := range tests {
		got := detectOvercorrection(tt.text)
		if got != tt.want {
			t.Errorf("detectOvercorrection(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}

func TestMultipleModels(t *testing.T) {
	findings := []model.Finding{
		mkFinding("f1", "gpt-4", "found sqli", model.SevHigh, model.StateValidated),
		mkFinding("f2", "gpt-4", "actually, revising finding", model.SevMedium, model.StateValidated),
		mkFinding("f3", "claude", "found xss", model.SevHigh, model.StateValidated),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
}

func TestGradeModel(t *testing.T) {
	ms := ModelScore{
		ImprovementRate:    0.9,
		OvercorrectionRate: 0.0,
		AccuracyAfter:      0.9,
	}
	g := gradeModel(ms)
	if g != "A" {
		t.Errorf("expected grade A, got %s", g)
	}

	ms2 := ModelScore{
		ImprovementRate:    0.1,
		OvercorrectionRate: 0.8,
		AccuracyAfter:      0.1,
	}
	g2 := gradeModel(ms2)
	if g2 == "A" {
		t.Error("bad model should not get grade A")
	}
}

func TestImprovementRate(t *testing.T) {
	findings := []model.Finding{
		mkFinding("f1", "gpt-4", "initial finding", model.SevHigh, model.StateTheorized),
		mkFinding("f2", "gpt-4", "actually, confirmed after validation", model.SevHigh, model.StateValidated),
		mkFinding("f3", "gpt-4", "wait, this is also valid", model.SevMedium, model.StateValidated),
	}
	r := Analyze(findings)
	m := r.Models[0]
	if m.ImprovementRate <= 0 {
		t.Error("expected positive improvement rate")
	}
}

func TestOvercorrectionRate(t *testing.T) {
	findings := []model.Finding{
		mkFinding("f1", "gpt-4", "found an issue", model.SevHigh, model.StateTheorized),
		mkFinding("f2", "gpt-4", "actually, my first answer was correct about the vuln", model.SevHigh, model.StateValidated),
	}
	r := Analyze(findings)
	m := r.Models[0]
	if m.OvercorrectionRate <= 0 {
		t.Error("expected positive overcorrection rate")
	}
}

func TestCorrectionEvents(t *testing.T) {
	findings := []model.Finding{
		mkFinding("f1", "gpt-4", "initial", model.SevHigh, model.StateTheorized),
		mkFinding("f2", "gpt-4", "sorry, let me correct this", model.SevMedium, model.StateValidated),
	}
	r := Analyze(findings)
	if len(r.Models[0].Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(r.Models[0].Events))
	}
	evt := r.Models[0].Events[0]
	if evt.Marker != "sorry," {
		t.Errorf("expected marker 'sorry,', got %q", evt.Marker)
	}
}

func TestFormatText(t *testing.T) {
	findings := []model.Finding{
		mkFinding("f1", "gpt-4", "actually, revising", model.SevHigh, model.StateValidated),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Self-Correction") {
		t.Error("expected header in text output")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Error("expected model name in text output")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{
		mkFinding("f1", "gpt-4", "actually, this is correct", model.SevHigh, model.StateValidated),
	}
	r := Analyze(findings)
	j := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(j), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(parsed.Models) == 0 {
		t.Error("expected models in JSON output")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No findings") {
		t.Error("expected empty message")
	}
}

func TestBestWorstModel(t *testing.T) {
	findings := []model.Finding{
		mkFinding("f1", "good-model", "actually, confirmed", model.SevHigh, model.StateValidated),
		mkFinding("f2", "bad-model", "actually, wrong again", model.SevLow, model.StateRefuted),
	}
	r := Analyze(findings)
	if r.BestModel == "" || r.WorstModel == "" {
		t.Error("expected best and worst model to be set")
	}
}

func TestUnknownModel(t *testing.T) {
	findings := []model.Finding{
		mkFinding("f1", "", "found something", model.SevHigh, model.StateValidated),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown', got %q", r.Models[0].Model)
	}
}

func TestAccuracyScores(t *testing.T) {
	findings := []model.Finding{
		mkFinding("f1", "gpt-4", "found issue", model.SevHigh, model.StateValidated),
		mkFinding("f2", "gpt-4", "actually, another issue", model.SevMedium, model.StateValidated),
		mkFinding("f3", "gpt-4", "third finding", model.SevLow, model.StateRefuted),
	}
	r := Analyze(findings)
	m := r.Models[0]
	if m.AccuracyAfter < 0 || m.AccuracyAfter > 1 {
		t.Errorf("accuracy after out of range: %f", m.AccuracyAfter)
	}
}
