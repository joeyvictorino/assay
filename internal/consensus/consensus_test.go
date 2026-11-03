package consensus

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func makeFinding(prompt, modelName, checker string, state model.FindingState, sev model.Severity) model.Finding {
	return model.Finding{
		ID:         prompt + "-" + modelName,
		DedupKey:   prompt + "-" + modelName,
		PromptHash: prompt,
		Model:      model.ModelRef{Model: modelName},
		State:      state,
		Severity:   sev,
		Class:      model.Class(checker),
		Summary:    "test finding from " + modelName,
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	results := Analyze(nil, 2)
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestAnalyzeBelowMinModels(t *testing.T) {
	findings := []model.Finding{
		makeFinding("p1", "gpt-4", "sqli", model.StateValidated, model.SevHigh),
	}
	results := Analyze(findings, 2)
	if len(results) != 0 {
		t.Fatalf("expected 0 results (below min models), got %d", len(results))
	}
}

func TestAnalyzeUnanimousPass(t *testing.T) {
	findings := []model.Finding{
		makeFinding("p1", "gpt-4", "sqli", model.StateValidated, model.SevHigh),
		makeFinding("p1", "claude-3", "sqli", model.StateValidated, model.SevHigh),
		makeFinding("p1", "gemini", "sqli", model.StateValidated, model.SevMedium),
	}
	results := Analyze(findings, 2)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if !r.Majority {
		t.Error("expected majority=true")
	}
	if r.Agreement != 1.0 {
		t.Errorf("expected agreement=1.0, got %.2f", r.Agreement)
	}
	if len(r.Dissenters) != 0 {
		t.Errorf("expected 0 dissenters, got %d", len(r.Dissenters))
	}
}

func TestAnalyzeSplitVerdicts(t *testing.T) {
	findings := []model.Finding{
		makeFinding("p1", "gpt-4", "sqli", model.StateValidated, model.SevHigh),
		makeFinding("p1", "claude-3", "sqli", model.StateRefuted, model.SevLow),
		makeFinding("p1", "gemini", "sqli", model.StateValidated, model.SevMedium),
	}
	results := Analyze(findings, 2)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if !r.Majority {
		t.Error("expected majority=true (2/3 passed)")
	}
	if len(r.Dissenters) != 1 {
		t.Fatalf("expected 1 dissenter, got %d", len(r.Dissenters))
	}
	if r.Dissenters[0].Model != "claude-3" {
		t.Errorf("expected dissenter claude-3, got %s", r.Dissenters[0].Model)
	}
}

func TestAnalyzeMultipleGroups(t *testing.T) {
	findings := []model.Finding{
		makeFinding("p1", "gpt-4", "sqli", model.StateValidated, model.SevHigh),
		makeFinding("p1", "claude-3", "sqli", model.StateValidated, model.SevHigh),
		makeFinding("p2", "gpt-4", "xss-reflected", model.StateRefuted, model.SevLow),
		makeFinding("p2", "claude-3", "xss-reflected", model.StateRefuted, model.SevLow),
	}
	results := Analyze(findings, 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestAnalyzeMajorityFail(t *testing.T) {
	findings := []model.Finding{
		makeFinding("p1", "gpt-4", "sqli", model.StateRefuted, model.SevLow),
		makeFinding("p1", "claude-3", "sqli", model.StateRefuted, model.SevLow),
		makeFinding("p1", "gemini", "sqli", model.StateValidated, model.SevHigh),
	}
	results := Analyze(findings, 2)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Majority {
		t.Error("expected majority=false (2/3 failed)")
	}
}

func TestFormatTextNotEmpty(t *testing.T) {
	findings := []model.Finding{
		makeFinding("p1", "gpt-4", "sqli", model.StateValidated, model.SevHigh),
		makeFinding("p1", "claude-3", "sqli", model.StateRefuted, model.SevLow),
	}
	results := Analyze(findings, 2)
	text := FormatText(results)
	if !strings.Contains(text, "Multi-Model Consensus") {
		t.Error("expected header in text output")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Error("expected model name in text output")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	text := FormatText(nil)
	if !strings.Contains(text, "No prompt") {
		t.Error("expected empty message")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{
		makeFinding("p1", "gpt-4", "sqli", model.StateValidated, model.SevHigh),
		makeFinding("p1", "claude-3", "sqli", model.StateValidated, model.SevHigh),
	}
	results := Analyze(findings, 2)
	out := FormatJSON(results)
	var parsed []Result
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(parsed) != 1 {
		t.Errorf("expected 1 result in JSON, got %d", len(parsed))
	}
}

func TestConsensusWeighting(t *testing.T) {
	findings := []model.Finding{
		makeFinding("p1", "gpt-4", "sqli", model.StateValidated, model.SevCritical),
		makeFinding("p1", "claude-3", "sqli", model.StateValidated, model.SevCritical),
		makeFinding("p1", "gemini", "sqli", model.StateRefuted, model.SevLow),
	}
	results := Analyze(findings, 2)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Consensus <= 0.5 {
		t.Errorf("expected consensus > 0.5 due to higher weight on critical models, got %.2f", results[0].Consensus)
	}
}

func TestSortByAgreement(t *testing.T) {
	findings := []model.Finding{
		makeFinding("p1", "gpt-4", "sqli", model.StateValidated, model.SevHigh),
		makeFinding("p1", "claude-3", "sqli", model.StateValidated, model.SevHigh),
		makeFinding("p2", "gpt-4", "xss-reflected", model.StateValidated, model.SevHigh),
		makeFinding("p2", "claude-3", "xss-reflected", model.StateRefuted, model.SevLow),
	}
	results := Analyze(findings, 2)
	if len(results) < 2 {
		t.Skip("need at least 2 groups")
	}
	if results[0].Agreement > results[1].Agreement {
		t.Error("expected lowest agreement first")
	}
}
