package promptrobust

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(id string, class model.Class, sev model.Severity, modelName, prompt, summary string) model.Finding {
	return model.Finding{
		ID:         id,
		Class:      class,
		Severity:   sev,
		Model:      model.ModelRef{Model: modelName},
		PromptHash: prompt,
		Summary:    summary,
		State:      model.StateValidated,
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
	if r.MeanScore != 0 {
		t.Fatalf("expected 0 mean, got %.1f", r.MeanScore)
	}
}

func TestAnalyzeNoFindings(t *testing.T) {
	r := Analyze([]model.Finding{})
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
}

func TestAnalyzeSinglePrompt(t *testing.T) {
	f := []model.Finding{
		finding("f1", "sqli", model.SevHigh, "model-a", "p1", "sql injection"),
		finding("f2", "sqli", model.SevHigh, "model-a", "p1", "another sqli"),
	}
	r := Analyze(f)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	m := r.Models[0]
	if m.UniquePrompts != 1 {
		t.Errorf("expected 1 unique prompt, got %d", m.UniquePrompts)
	}
	if m.ConsistencyScore < 90 {
		t.Errorf("single prompt should have high consistency, got %.1f", m.ConsistencyScore)
	}
}

func TestAnalyzeMultiPromptConsistent(t *testing.T) {
	f := []model.Finding{
		finding("f1", "sqli", model.SevHigh, "model-a", "p1", "sqli 1"),
		finding("f2", "sqli", model.SevHigh, "model-a", "p2", "sqli 2"),
		finding("f3", "sqli", model.SevHigh, "model-a", "p3", "sqli 3"),
	}
	r := Analyze(f)
	m := r.Models[0]
	if m.ConsistencyScore < 90 {
		t.Errorf("equal findings per prompt should have high consistency, got %.1f", m.ConsistencyScore)
	}
}

func TestAnalyzeMultiPromptInconsistent(t *testing.T) {
	var f []model.Finding
	for i := 0; i < 10; i++ {
		f = append(f, finding("f", "sqli", model.SevHigh, "model-a", "p1", "sqli"))
	}
	f = append(f, finding("g", "xss-reflected", model.SevLow, "model-a", "p2", "xss"))
	r := Analyze(f)
	m := r.Models[0]
	if m.ConsistencyScore >= 100 {
		t.Errorf("unequal findings per prompt should reduce consistency, got %.1f", m.ConsistencyScore)
	}
}

func TestAnalyzeMultiModel(t *testing.T) {
	f := []model.Finding{
		finding("f1", "sqli", model.SevHigh, "model-a", "p1", "sqli"),
		finding("f2", "sqli", model.SevHigh, "model-a", "p2", "sqli"),
		finding("f3", "sqli", model.SevCritical, "model-b", "p1", "sqli"),
		finding("f4", "xss-reflected", model.SevLow, "model-b", "p2", "xss"),
	}
	r := Analyze(f)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.MostRobust == "" {
		t.Error("expected most robust to be set")
	}
	if r.LeastRobust == "" {
		t.Error("expected least robust to be set")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	f := []model.Finding{
		{ID: "f1", Class: "sqli", Severity: model.SevLow, State: model.StateValidated, PromptHash: "p1"},
	}
	r := Analyze(f)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown' model, got %q", r.Models[0].Model)
	}
}

func TestSeverityVariance(t *testing.T) {
	uniform := []model.Finding{
		finding("f1", "sqli", model.SevHigh, "m", "p1", "a"),
		finding("f2", "sqli", model.SevHigh, "m", "p2", "b"),
	}
	mixed := []model.Finding{
		finding("f1", "sqli", model.SevCritical, "m", "p1", "a"),
		finding("f2", "sqli", model.SevInfo, "m", "p2", "b"),
	}
	uVar := computeSeverityVariance(uniform)
	mVar := computeSeverityVariance(mixed)
	if mVar <= uVar {
		t.Errorf("mixed severity should have higher variance: %.2f <= %.2f", mVar, uVar)
	}
}

func TestClassVariance(t *testing.T) {
	uniform := []model.Finding{
		finding("f1", "sqli", model.SevHigh, "m", "p1", "a"),
		finding("f2", "sqli", model.SevHigh, "m", "p2", "b"),
	}
	mixed := []model.Finding{
		finding("f1", "sqli", model.SevHigh, "m", "p1", "a"),
		finding("f2", "xss-reflected", model.SevHigh, "m", "p2", "b"),
		finding("f3", "xss-reflected", model.SevHigh, "m", "p2", "c"),
		finding("f4", "xss-reflected", model.SevHigh, "m", "p2", "d"),
	}
	uVar := computeClassVariance(uniform)
	mVar := computeClassVariance(mixed)
	if uVar != 0 {
		t.Errorf("single class should have 0 variance, got %.2f", uVar)
	}
	if mVar <= 0 {
		t.Errorf("multiple classes with unequal counts should have positive variance, got %.2f", mVar)
	}
}

func TestScoreFloor(t *testing.T) {
	var f []model.Finding
	for i := 0; i < 50; i++ {
		sev := model.SevCritical
		if i%2 == 0 {
			sev = model.SevInfo
		}
		f = append(f, finding("f", model.Class("sqli"), sev, "m", "p1", "test"))
	}
	r := Analyze(f)
	if r.Models[0].OverallScore < 0 {
		t.Errorf("score should not go below 0, got %.1f", r.Models[0].OverallScore)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{100, "A"}, {90, "A"}, {89, "B"}, {80, "B"},
		{79, "C"}, {70, "C"}, {69, "D"}, {60, "D"},
		{59, "F"}, {0, "F"},
	}
	for _, tt := range tests {
		got := scoreToGrade(tt.score)
		if got != tt.want {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestFormatTextOutput(t *testing.T) {
	f := []model.Finding{
		finding("f1", "sqli", model.SevHigh, "model-a", "p1", "sqli"),
	}
	r := Analyze(f)
	text := FormatText(r)
	if !strings.Contains(text, "Prompt Robustness") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "model-a") {
		t.Error("missing model name")
	}
	if !strings.Contains(text, "Most robust") {
		t.Error("missing most robust line")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No robustness data") {
		t.Error("expected empty message")
	}
}

func TestFormatJSON(t *testing.T) {
	f := []model.Finding{
		finding("f1", "sqli", model.SevHigh, "model-a", "p1", "sqli"),
	}
	r := Analyze(f)
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(parsed.Models) != 1 {
		t.Errorf("expected 1 model, got %d", len(parsed.Models))
	}
}

func TestTotalPrompts(t *testing.T) {
	f := []model.Finding{
		finding("f1", "sqli", model.SevHigh, "model-a", "p1", "a"),
		finding("f2", "sqli", model.SevHigh, "model-a", "p2", "b"),
		finding("f3", "sqli", model.SevHigh, "model-b", "p1", "c"),
		finding("f4", "sqli", model.SevHigh, "model-b", "p3", "d"),
	}
	r := Analyze(f)
	if r.TotalPrompts != 3 {
		t.Errorf("expected 3 total prompts (p1, p2, p3), got %d", r.TotalPrompts)
	}
}

func TestNoHashPrompt(t *testing.T) {
	f := []model.Finding{
		{ID: "f1", Class: "sqli", Severity: model.SevHigh, Model: model.ModelRef{Model: "m"}, State: model.StateValidated},
	}
	r := Analyze(f)
	if r.Models[0].UniquePrompts != 1 {
		t.Errorf("expected 1 unique prompt (no-hash), got %d", r.Models[0].UniquePrompts)
	}
	if r.Models[0].PromptGroups[0].PromptHash != "no-hash" {
		t.Errorf("expected 'no-hash' group, got %q", r.Models[0].PromptGroups[0].PromptHash)
	}
}
