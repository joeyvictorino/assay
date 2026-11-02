package modelprofile

import (
	"strings"
	"testing"
)

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze("test-model", nil)
	if r.Summary.TotalSamples != 0 {
		t.Fatalf("expected 0 samples, got %d", r.Summary.TotalSamples)
	}
	if r.Summary.Dimensions != 0 {
		t.Fatalf("expected 0 dimensions, got %d", r.Summary.Dimensions)
	}
}

func TestSingleDimension(t *testing.T) {
	results := []EvalResult{
		{Dimension: "accuracy", Score: 0.9},
		{Dimension: "accuracy", Score: 0.85},
	}
	r := Analyze("gpt-4", results)
	if r.Summary.Dimensions != 1 {
		t.Fatalf("expected 1 dimension, got %d", r.Summary.Dimensions)
	}
	if r.Scores[0].Dimension != "accuracy" {
		t.Fatal("expected accuracy dimension")
	}
}

func TestStrengthDetection(t *testing.T) {
	results := []EvalResult{
		{Dimension: "safety", Score: 0.95},
		{Dimension: "safety", Score: 0.92},
	}
	r := Analyze("model", results)
	if len(r.Strengths) == 0 {
		t.Fatal("expected strength for high-scoring dimension")
	}
}

func TestWeaknessDetection(t *testing.T) {
	results := []EvalResult{
		{Dimension: "math", Score: 0.3},
		{Dimension: "math", Score: 0.4},
	}
	r := Analyze("model", results)
	if len(r.Weaknesses) == 0 {
		t.Fatal("expected weakness for low-scoring dimension")
	}
}

func TestMultipleDimensions(t *testing.T) {
	results := []EvalResult{
		{Dimension: "safety", Score: 0.95},
		{Dimension: "accuracy", Score: 0.85},
		{Dimension: "speed", Score: 0.4},
	}
	r := Analyze("model", results)
	if r.Summary.Dimensions != 3 {
		t.Fatalf("expected 3 dimensions, got %d", r.Summary.Dimensions)
	}
	if r.Summary.Strengths == 0 {
		t.Fatal("expected at least one strength")
	}
	if r.Summary.Weaknesses == 0 {
		t.Fatal("expected at least one weakness")
	}
}

func TestGrading(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{0.95, "A"}, {0.85, "B"}, {0.75, "C"}, {0.65, "D"}, {0.45, "F"},
	}
	for _, tt := range tests {
		got := floatGrade(tt.score)
		if got != tt.grade {
			t.Errorf("floatGrade(%f) = %s, want %s", tt.score, got, tt.grade)
		}
	}
}

func TestFormatTextContainsHeader(t *testing.T) {
	r := Analyze("model", nil)
	out := FormatText(r)
	if !strings.Contains(out, "Model Behavioral Profile") {
		t.Fatal("expected header")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze("model", []EvalResult{{Dimension: "test", Score: 0.5}})
	out := FormatJSON(r)
	if !strings.Contains(out, "\"model_name\"") {
		t.Fatal("expected model_name in JSON")
	}
}

func TestOverallScore(t *testing.T) {
	results := []EvalResult{
		{Dimension: "a", Score: 0.8},
		{Dimension: "b", Score: 0.6},
	}
	r := Analyze("model", results)
	if r.Summary.OverallScore < 0.69 || r.Summary.OverallScore > 0.71 {
		t.Fatalf("expected overall ~0.7, got %f", r.Summary.OverallScore)
	}
}
