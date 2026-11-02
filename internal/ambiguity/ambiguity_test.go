package ambiguity

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

func TestNoAmbiguousFindings(t *testing.T) {
	findings := []model.Finding{
		mkFinding("f1", "gpt-4", "found sqli in login endpoint", model.SevHigh, model.StateValidated),
		mkFinding("f2", "gpt-4", "found xss in comment field", model.SevMedium, model.StateValidated),
	}
	r := Analyze(findings)
	if r.Models[0].AmbiguousCount != 0 {
		t.Errorf("expected 0 ambiguous, got %d", r.Models[0].AmbiguousCount)
	}
}

func TestAmbiguousWithClarification(t *testing.T) {
	findings := []model.Finding{
		mkFinding("f1", "gpt-4", "the endpoint is ambiguous, could you clarify which API version", model.SevMedium, model.StateTheorized),
	}
	r := Analyze(findings)
	m := r.Models[0]
	if m.AmbiguousCount != 1 {
		t.Errorf("expected 1 ambiguous, got %d", m.AmbiguousCount)
	}
	if m.ClarificationRate != 1.0 {
		t.Errorf("expected clarification rate 1.0, got %.2f", m.ClarificationRate)
	}
}

func TestAmbiguousWithGuess(t *testing.T) {
	findings := []model.Finding{
		mkFinding("f1", "gpt-4", "the target is ambiguous but i'll assume it's the main API", model.SevHigh, model.StateValidated),
	}
	r := Analyze(findings)
	m := r.Models[0]
	if m.AmbiguousCount != 1 {
		t.Errorf("expected 1 ambiguous, got %d", m.AmbiguousCount)
	}
	if m.GuessRate != 1.0 {
		t.Errorf("expected guess rate 1.0, got %.2f", m.GuessRate)
	}
	if m.GuessAccuracy != 1.0 {
		t.Errorf("expected guess accuracy 1.0, got %.2f", m.GuessAccuracy)
	}
}

func TestAmbiguousWithWrongGuess(t *testing.T) {
	findings := []model.Finding{
		mkFinding("f1", "gpt-4", "this is vague, assuming main endpoint", model.SevHigh, model.StateRefuted),
	}
	r := Analyze(findings)
	m := r.Models[0]
	if m.GuessAccuracy != 0.0 {
		t.Errorf("expected guess accuracy 0.0, got %.2f", m.GuessAccuracy)
	}
}

func TestDetectAmbiguityMarkers(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"the input is ambiguous", true},
		{"this is unclear and vague", true},
		{"could mean different things", true},
		{"open to interpretation by the model", true},
		{"clear finding of sqli", false},
		{"definite xss vulnerability", false},
	}
	for _, tt := range tests {
		got := detectAmbiguity(tt.text)
		if (got != "") != tt.want {
			t.Errorf("detectAmbiguity(%q) = %q, want match=%v", tt.text, got, tt.want)
		}
	}
}

func TestDetectClarification(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"could you clarify the endpoint", true},
		{"what do you mean by auth", true},
		{"please specify the version", true},
		{"found a vulnerability", false},
	}
	for _, tt := range tests {
		got := detectClarification(tt.text)
		if got != tt.want {
			t.Errorf("detectClarification(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}

func TestDetectGuess(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"i'll assume the main endpoint", true},
		{"assuming v2 of the api", true},
		{"my best guess is it's sqli", true},
		{"confirmed vulnerability", false},
	}
	for _, tt := range tests {
		got := detectGuess(tt.text)
		if got != tt.want {
			t.Errorf("detectGuess(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}

func TestMultipleModels(t *testing.T) {
	findings := []model.Finding{
		mkFinding("f1", "gpt-4", "this is ambiguous, could you clarify", model.SevHigh, model.StateTheorized),
		mkFinding("f2", "claude", "unclear target, assuming main api", model.SevHigh, model.StateValidated),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
}

func TestSensitivityScore(t *testing.T) {
	findings := []model.Finding{
		mkFinding("f1", "gpt-4", "ambiguous endpoint, could you clarify which version", model.SevHigh, model.StateTheorized),
		mkFinding("f2", "gpt-4", "clear finding of sqli", model.SevHigh, model.StateValidated),
	}
	r := Analyze(findings)
	m := r.Models[0]
	if m.SensitivityScore < 0 || m.SensitivityScore > 1 {
		t.Errorf("sensitivity out of range: %f", m.SensitivityScore)
	}
}

func TestGradeAssignment(t *testing.T) {
	ms := ModelScore{SensitivityScore: 0.9}
	if g := gradeModel(ms); g != "A" {
		t.Errorf("expected A, got %s", g)
	}
	ms2 := ModelScore{SensitivityScore: 0.1}
	if g := gradeModel(ms2); g != "F" {
		t.Errorf("expected F, got %s", g)
	}
}

func TestFormatText(t *testing.T) {
	findings := []model.Finding{
		mkFinding("f1", "gpt-4", "ambiguous input, need more context", model.SevHigh, model.StateTheorized),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Ambiguity") {
		t.Error("expected header in text output")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Error("expected model name in text output")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{
		mkFinding("f1", "gpt-4", "vague description, could you clarify", model.SevHigh, model.StateTheorized),
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
		mkFinding("f1", "model-a", "ambiguous, could you clarify", model.SevHigh, model.StateTheorized),
		mkFinding("f2", "model-b", "clear finding", model.SevHigh, model.StateValidated),
	}
	r := Analyze(findings)
	if r.BestModel == "" || r.WorstModel == "" {
		t.Error("expected best and worst model set")
	}
}

func TestUnknownModel(t *testing.T) {
	findings := []model.Finding{
		mkFinding("f1", "", "unclear issue", model.SevHigh, model.StateTheorized),
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown', got %q", r.Models[0].Model)
	}
}

func TestResponseStrategy(t *testing.T) {
	findings := []model.Finding{
		mkFinding("f1", "gpt-4", "ambiguous, could you clarify", model.SevHigh, model.StateTheorized),
		mkFinding("f2", "gpt-4", "vague but i'll assume prod api", model.SevHigh, model.StateValidated),
		mkFinding("f3", "gpt-4", "ambiguous, proceeding anyway", model.SevMedium, model.StateTheorized),
		mkFinding("f4", "gpt-4", "clear sqli vulnerability", model.SevHigh, model.StateValidated),
	}
	r := Analyze(findings)
	m := r.Models[0]
	strategies := map[string]int{}
	for _, a := range m.Analyses {
		strategies[a.ResponseStrategy]++
	}
	if strategies["clarification"] != 1 {
		t.Errorf("expected 1 clarification, got %d", strategies["clarification"])
	}
	if strategies["guess"] != 1 {
		t.Errorf("expected 1 guess, got %d", strategies["guess"])
	}
	if strategies["unambiguous"] != 1 {
		t.Errorf("expected 1 unambiguous, got %d", strategies["unambiguous"])
	}
}
