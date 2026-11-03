package idempotency

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func mkFinding(modelName, lab, sev, class, state string) model.Finding {
	return model.Finding{
		Lab:      lab,
		Model:    model.ModelRef{Model: modelName},
		Severity: model.Severity(sev),
		Class:    model.Class(class),
		State:    model.FindingState(state),
		Location: model.Location{Method: "GET", PathTemplate: "/api/test"},
	}
}

func TestEmptyFindings(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
}

func TestSingleFindingPerfectScore(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "high", "sqli", "validated"),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].IdempotencyScore != 1.0 {
		t.Fatalf("expected score 1.0, got %.2f", r.Models[0].IdempotencyScore)
	}
}

func TestConsistentPair(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "high", "sqli", "validated"),
		mkFinding("gpt-4", "lab1", "high", "sqli", "validated"),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.ConsistentPairs != 1 {
		t.Fatalf("expected 1 consistent pair, got %d", r.Models[0].Detail.ConsistentPairs)
	}
	if r.Models[0].Detail.InconsistentPairs != 0 {
		t.Fatal("expected 0 inconsistent pairs")
	}
}

func TestSeverityFlip(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "high", "sqli", "validated"),
		mkFinding("gpt-4", "lab1", "low", "sqli", "validated"),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.SeverityFlips != 1 {
		t.Fatalf("expected 1 severity flip, got %d", r.Models[0].Detail.SeverityFlips)
	}
}

func TestClassFlip(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "high", "sqli", "validated"),
		mkFinding("gpt-4", "lab1", "high", "xss-reflected", "validated"),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.ClassFlips != 1 {
		t.Fatalf("expected 1 class flip, got %d", r.Models[0].Detail.ClassFlips)
	}
}

func TestStateFlip(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "high", "sqli", "validated"),
		mkFinding("gpt-4", "lab1", "high", "sqli", "refuted"),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.StateFlips != 1 {
		t.Fatalf("expected 1 state flip, got %d", r.Models[0].Detail.StateFlips)
	}
}

func TestMultipleModels(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "high", "sqli", "validated"),
		mkFinding("gpt-4", "lab1", "high", "sqli", "validated"),
		mkFinding("claude", "lab1", "high", "sqli", "validated"),
		mkFinding("claude", "lab1", "low", "sqli", "validated"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.MostConsistent != "gpt-4" {
		t.Fatalf("expected gpt-4 most consistent, got %s", r.MostConsistent)
	}
}

func TestGradeAssignment(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{0.96, "A"},
		{0.88, "B"},
		{0.75, "C"},
		{0.55, "D"},
		{0.30, "F"},
	}
	for _, tt := range tests {
		got := grade(tt.score)
		if got != tt.want {
			t.Errorf("grade(%.2f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestFormatText(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "Idempotency") {
		t.Fatal("expected header in text output")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze(nil)
	j := FormatJSON(r)
	if !strings.Contains(j, "models") {
		t.Fatal("expected models key in JSON output")
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m1", "lab1", "high", "sqli", "validated"),
		mkFinding("m1", "lab1", "high", "sqli", "validated"),
		mkFinding("m2", "lab1", "high", "sqli", "validated"),
		mkFinding("m2", "lab1", "high", "sqli", "validated"),
	}
	r := Analyze(findings)
	if r.MeanScore < 0.5 {
		t.Fatalf("expected mean score >= 0.5, got %.2f", r.MeanScore)
	}
}

func TestLabCount(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "high", "sqli", "validated"),
		mkFinding("gpt-4", "lab2", "high", "xss-reflected", "validated"),
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Fatalf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}
