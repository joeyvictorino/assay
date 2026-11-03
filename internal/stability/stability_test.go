package stability

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
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	results := Analyze(nil, 3)
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestAnalyzeBelowMinRuns(t *testing.T) {
	findings := []model.Finding{
		makeFinding("p1", "gpt-4", "sqli", model.StateValidated, model.SevHigh),
		makeFinding("p1", "gpt-4", "sqli", model.StateValidated, model.SevHigh),
	}
	results := Analyze(findings, 3)
	if len(results) != 0 {
		t.Fatalf("expected 0 results (below min runs), got %d", len(results))
	}
}

func TestAnalyzeConsistentRuns(t *testing.T) {
	findings := []model.Finding{
		makeFinding("p1", "gpt-4", "sqli", model.StateValidated, model.SevHigh),
		makeFinding("p1", "gpt-4", "sqli", model.StateValidated, model.SevHigh),
		makeFinding("p1", "gpt-4", "sqli", model.StateValidated, model.SevHigh),
	}
	results := Analyze(findings, 3)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Consistency != 1.0 {
		t.Errorf("expected consistency=1.0, got %.2f", r.Consistency)
	}
	if r.FlipCount != 0 {
		t.Errorf("expected 0 flips, got %d", r.FlipCount)
	}
	if r.Grade != "A" {
		t.Errorf("expected grade A, got %s", r.Grade)
	}
}

func TestAnalyzeFlippingRuns(t *testing.T) {
	findings := []model.Finding{
		makeFinding("p1", "gpt-4", "sqli", model.StateValidated, model.SevHigh),
		makeFinding("p1", "gpt-4", "sqli", model.StateRefuted, model.SevLow),
		makeFinding("p1", "gpt-4", "sqli", model.StateValidated, model.SevHigh),
		makeFinding("p1", "gpt-4", "sqli", model.StateRefuted, model.SevLow),
	}
	results := Analyze(findings, 3)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.FlipCount != 3 {
		t.Errorf("expected 3 flips, got %d", r.FlipCount)
	}
	if r.Consistency != 0.5 {
		t.Errorf("expected consistency=0.5, got %.2f", r.Consistency)
	}
	if r.Grade == "A" || r.Grade == "B" {
		t.Errorf("expected grade C or worse for flipping, got %s", r.Grade)
	}
}

func TestAnalyzeScoreVariance(t *testing.T) {
	findings := []model.Finding{
		makeFinding("p1", "gpt-4", "sqli", model.StateValidated, model.SevCritical),
		makeFinding("p1", "gpt-4", "sqli", model.StateValidated, model.SevLow),
		makeFinding("p1", "gpt-4", "sqli", model.StateValidated, model.SevHigh),
	}
	results := Analyze(findings, 3)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ScoreVariance <= 0 {
		t.Error("expected non-zero variance for mixed severities")
	}
}

func TestAnalyzeMultipleGroups(t *testing.T) {
	findings := []model.Finding{
		makeFinding("p1", "gpt-4", "sqli", model.StateValidated, model.SevHigh),
		makeFinding("p1", "gpt-4", "sqli", model.StateValidated, model.SevHigh),
		makeFinding("p1", "gpt-4", "sqli", model.StateValidated, model.SevHigh),
		makeFinding("p2", "claude-3", "xss-reflected", model.StateRefuted, model.SevLow),
		makeFinding("p2", "claude-3", "xss-reflected", model.StateRefuted, model.SevLow),
		makeFinding("p2", "claude-3", "xss-reflected", model.StateRefuted, model.SevLow),
	}
	results := Analyze(findings, 3)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestGrading(t *testing.T) {
	tests := []struct {
		consistency float64
		variance    float64
		flips       int
		runs        int
		wantGrade   string
	}{
		{1.0, 0.0, 0, 5, "A"},
		{0.9, 0.03, 0, 10, "B"},
		{0.75, 0.08, 2, 10, "C"},
		{0.6, 0.15, 4, 10, "D"},
		{0.5, 0.20, 5, 10, "F"},
	}
	for _, tt := range tests {
		got := gradeFromMetrics(tt.consistency, tt.variance, tt.flips, tt.runs)
		if got != tt.wantGrade {
			t.Errorf("gradeFromMetrics(%.2f, %.2f, %d, %d) = %s, want %s",
				tt.consistency, tt.variance, tt.flips, tt.runs, got, tt.wantGrade)
		}
	}
}

func TestFormatTextNotEmpty(t *testing.T) {
	findings := []model.Finding{
		makeFinding("p1", "gpt-4", "sqli", model.StateValidated, model.SevHigh),
		makeFinding("p1", "gpt-4", "sqli", model.StateValidated, model.SevHigh),
		makeFinding("p1", "gpt-4", "sqli", model.StateRefuted, model.SevLow),
	}
	results := Analyze(findings, 3)
	text := FormatText(results)
	if !strings.Contains(text, "Prompt Stability") {
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
		makeFinding("p1", "gpt-4", "sqli", model.StateValidated, model.SevHigh),
		makeFinding("p1", "gpt-4", "sqli", model.StateValidated, model.SevHigh),
	}
	results := Analyze(findings, 3)
	out := FormatJSON(results)
	var parsed []Result
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(parsed) != 1 {
		t.Errorf("expected 1 result in JSON, got %d", len(parsed))
	}
}

func TestCountFlips(t *testing.T) {
	tests := []struct {
		verdicts []bool
		want     int
	}{
		{[]bool{true, true, true}, 0},
		{[]bool{true, false, true}, 2},
		{[]bool{false, false, false}, 0},
		{[]bool{true, false}, 1},
	}
	for _, tt := range tests {
		got := countFlips(tt.verdicts)
		if got != tt.want {
			t.Errorf("countFlips(%v) = %d, want %d", tt.verdicts, got, tt.want)
		}
	}
}

func TestScoreVariance(t *testing.T) {
	if v := scoreVariance([]float64{0.5, 0.5, 0.5}); v != 0 {
		t.Errorf("expected 0 variance for identical scores, got %f", v)
	}
	if v := scoreVariance(nil); v != 0 {
		t.Errorf("expected 0 variance for empty, got %f", v)
	}
	if v := scoreVariance([]float64{0.0, 1.0}); v <= 0 {
		t.Errorf("expected positive variance for [0, 1], got %f", v)
	}
}

func TestSortByGrade(t *testing.T) {
	findings := []model.Finding{
		makeFinding("stable", "gpt-4", "sqli", model.StateValidated, model.SevHigh),
		makeFinding("stable", "gpt-4", "sqli", model.StateValidated, model.SevHigh),
		makeFinding("stable", "gpt-4", "sqli", model.StateValidated, model.SevHigh),
		makeFinding("unstable", "claude-3", "xss-reflected", model.StateValidated, model.SevHigh),
		makeFinding("unstable", "claude-3", "xss-reflected", model.StateRefuted, model.SevLow),
		makeFinding("unstable", "claude-3", "xss-reflected", model.StateValidated, model.SevCritical),
	}
	results := Analyze(findings, 3)
	if len(results) < 2 {
		t.Skip("need at least 2 groups")
	}
	g0 := gradeRank(results[0].Grade)
	g1 := gradeRank(results[1].Grade)
	if g0 < g1 {
		t.Errorf("expected worst grade first, got %s before %s", results[0].Grade, results[1].Grade)
	}
}
