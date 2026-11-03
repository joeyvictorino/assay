package negation

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

func TestNegationDetection(t *testing.T) {
	f := makeFinding("gpt-4", "The system cannot process requests that don't include authentication. It isn't possible without credentials.")
	r := Analyze([]model.Finding{f})
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Detail.NegationCount == 0 {
		t.Error("expected negation phrases detected")
	}
}

func TestDoubleNegDetection(t *testing.T) {
	f := makeFinding("claude", "It is not uncommon for models to not understand double negatives. The result is not invalid.")
	r := Analyze([]model.Finding{f})
	if r.Models[0].Detail.DoubleNegCount == 0 {
		t.Error("expected double negation detected")
	}
}

func TestContradictionDetection(t *testing.T) {
	f := makeFinding("claude", "The results are inconsistent and contradict the earlier analysis. This paradox is irreconcilable.")
	r := Analyze([]model.Finding{f})
	if r.Models[0].Detail.ContradictionCount == 0 {
		t.Error("expected contradiction detected")
	}
}

func TestMultipleModels(t *testing.T) {
	findings := []model.Finding{
		makeFinding("model-a", "The system cannot handle negation without proper scope"),
		makeFinding("model-b", "simple positive response"),
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
	f := makeFinding("test-model", "not without negation scope")
	r := Analyze([]model.Finding{f})
	text := FormatText(r)
	if !strings.Contains(text, "Negation") {
		t.Error("expected header")
	}
	if !strings.Contains(text, "test-model") {
		t.Error("expected model name")
	}
}

func TestFormatJSON(t *testing.T) {
	f := makeFinding("test-model", "negation scope")
	r := Analyze([]model.Finding{f})
	j := FormatJSON(r)
	if !strings.Contains(j, "negation_score") {
		t.Error("expected JSON field")
	}
}

func TestUnknownModel(t *testing.T) {
	f := model.Finding{Summary: "not applicable"}
	r := Analyze([]model.Finding{f})
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown model, got %s", r.Models[0].Model)
	}
}

func TestBaselineScore(t *testing.T) {
	f := makeFinding("weak", "simple response with no negation handling")
	r := Analyze([]model.Finding{f})
	if r.Models[0].NegationScore < 20 || r.Models[0].NegationScore > 90 {
		t.Errorf("expected reasonable baseline score, got %.1f", r.Models[0].NegationScore)
	}
}
