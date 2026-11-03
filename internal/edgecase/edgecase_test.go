package edgecase

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func makeFinding(modelName, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: modelName},
		Summary: summary,
	}
}

func TestEmptyFindings(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Error("expected empty report for nil findings")
	}
}

func TestBoundaryDetection(t *testing.T) {
	f := makeFinding("gpt-4", "edge case at boundary condition with maximum value and off-by-one error")
	r := Analyze([]model.Finding{f})
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Detail.BoundaryCount == 0 {
		t.Error("expected boundary phrases detected")
	}
	if r.Models[0].EdgeCaseScore < 30 {
		t.Error("expected score above 30 for boundary-rich text")
	}
}

func TestNullDetection(t *testing.T) {
	f := makeFinding("claude", "null input returns empty response with undefined behavior for missing value")
	r := Analyze([]model.Finding{f})
	if r.Models[0].Detail.NullCount == 0 {
		t.Error("expected null phrases detected")
	}
}

func TestOverflowDetection(t *testing.T) {
	f := makeFinding("claude", "integer overflow causes wraparound when value exceeds size limit")
	r := Analyze([]model.Finding{f})
	if r.Models[0].Detail.OverflowCount == 0 {
		t.Error("expected overflow phrases detected")
	}
}

func TestMultipleModels(t *testing.T) {
	findings := []model.Finding{
		makeFinding("model-a", "boundary condition edge case with null handling"),
		makeFinding("model-b", "simple response"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" || r.WorstModel == "" {
		t.Error("expected best and worst model set")
	}
}

func TestGrading(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"}, {85, "B"}, {75, "C"}, {65, "D"}, {45, "F"},
	}
	for _, tt := range tests {
		if g := scoreToGrade(tt.score); g != tt.grade {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tt.score, g, tt.grade)
		}
	}
}

func TestFormatText(t *testing.T) {
	f := makeFinding("test-model", "edge case boundary limit")
	r := Analyze([]model.Finding{f})
	text := FormatText(r)
	if !strings.Contains(text, "Edge Case") {
		t.Error("expected header")
	}
	if !strings.Contains(text, "test-model") {
		t.Error("expected model name")
	}
}

func TestFormatJSON(t *testing.T) {
	f := makeFinding("test-model", "edge case boundary")
	r := Analyze([]model.Finding{f})
	j := FormatJSON(r)
	if !strings.Contains(j, "edge_case_score") {
		t.Error("expected JSON field")
	}
}

func TestUnknownModel(t *testing.T) {
	f := model.Finding{Summary: "boundary edge case"}
	r := Analyze([]model.Finding{f})
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown model, got %s", r.Models[0].Model)
	}
}

func TestConcernsCount(t *testing.T) {
	f := makeFinding("weak", "ok")
	r := Analyze([]model.Finding{f})
	if r.Models[0].Concerns == 0 {
		t.Error("expected concern for low-scoring response")
	}
}
