package proportionality

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(modelName, summary string, evidence ...string) model.Finding {
	var evs []model.Evidence
	for _, e := range evidence {
		evs = append(evs, model.Evidence{Marker: e})
	}
	return model.Finding{
		Model:    model.ModelRef{Model: modelName},
		Summary:  summary,
		Evidence: evs,
		Lab:      "test-lab",
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Error("expected empty report")
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("gpt-4", "Short answer.", "What is 2+2?"),
	})
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].ProportionalityScore <= 0 {
		t.Error("expected positive score")
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("gpt-4", "Concise.", "What is X?"),
		finding("claude", "Also concise.", "What is Y?"),
	})
	if len(r.Models) != 2 {
		t.Errorf("expected 2 models, got %d", len(r.Models))
	}
}

func TestQueryComplexitySimple(t *testing.T) {
	c := queryComplexity("What is 2+2?")
	if c > 0.6 {
		t.Errorf("simple query should have low complexity, got %.2f", c)
	}
}

func TestQueryComplexityComplex(t *testing.T) {
	c := queryComplexity("Explain the architecture and implementation details of the comprehensive distributed system design with trade-offs")
	if c < 0.6 {
		t.Errorf("complex query should have high complexity, got %.2f", c)
	}
}

func TestProportionalResponse(t *testing.T) {
	response := "The answer is 42. This is a simple factual response."
	r := Analyze([]model.Finding{finding("m1", response, "What is the answer?")})
	if r.Models[0].ProportionalityScore < 40 {
		t.Errorf("proportional response should score reasonably, got %.1f", r.Models[0].ProportionalityScore)
	}
}

func TestOverlyVerboseSimpleQuery(t *testing.T) {
	verbose := strings.Repeat("This is filler content with lots of unnecessary words. ", 50)
	r := Analyze([]model.Finding{finding("m1", verbose, "Is it true?")})
	baseline := Analyze([]model.Finding{finding("m1", "Yes, it is true.", "Is it true?")})
	if r.Models[0].ProportionalityScore >= baseline.Models[0].ProportionalityScore {
		t.Error("overly verbose answer to simple query should score lower than concise answer")
	}
}

func TestGradeMapping(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"}, {85, "B"}, {75, "C"}, {65, "D"}, {45, "F"},
	}
	for _, tt := range tests {
		if got := scoreToGrade(tt.score); got != tt.grade {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tt.score, got, tt.grade)
		}
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No proportionality data available") {
		t.Error("expected empty message")
	}
}

func TestFormatTextHeader(t *testing.T) {
	r := Analyze([]model.Finding{finding("test-model", "test", "query")})
	text := FormatText(r)
	if !strings.Contains(text, "Response Proportionality Analysis") {
		t.Error("missing title")
	}
}

func TestUnknownModelName(t *testing.T) {
	r := Analyze([]model.Finding{{Summary: "test", Lab: "lab1"}})
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown', got %s", r.Models[0].Model)
	}
}

func TestLabCounting(t *testing.T) {
	r := Analyze([]model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Summary: "a", Lab: "lab-1"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "b", Lab: "lab-2"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "c", Lab: "lab-1"},
	})
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestMeanScore(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "response one", "query one"),
		finding("m2", "response two", "query two"),
	})
	if r.MeanScore <= 0 {
		t.Error("expected positive mean score")
	}
}

func TestExpectedLength(t *testing.T) {
	low := expectedLength(0.1)
	high := expectedLength(1.0)
	if low >= high {
		t.Errorf("expected low complexity (%d) < high complexity (%d)", low, high)
	}
}
