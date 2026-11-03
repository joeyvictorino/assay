package recall

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func makeFinding(modelName, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: modelName},
		Lab:     "test-lab",
		Summary: summary,
	}
}

func TestHighRecall(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "Comprehensively addressing all aspects of the vulnerability. Furthermore, also considering the network layer. Additionally, every component was thoroughly analyzed in its entirety."),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].RecallScore < 60 {
		t.Errorf("expected high recall score, got %.1f", r.Models[0].RecallScore)
	}
}

func TestLowRecall(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-3", "Skipping the network analysis. Omitting the authentication layer. Beyond the scope. Only briefly touching on the issue. Partially explored. Not in depth. Cursory look. I overlooked the key finding."),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].RecallScore > 50 {
		t.Errorf("expected low recall score for gap-heavy response, got %.1f", r.Models[0].RecallScore)
	}
}

func TestEmptyFindings(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Errorf("expected empty report, got %d models", len(r.Models))
	}
}

func TestMultipleModels(t *testing.T) {
	findings := []model.Finding{
		makeFinding("model-a", "Comprehensively covering each aspect. Furthermore additionally."),
		makeFinding("model-b", "Skipping important details. Only briefly mentioned. Partially done."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" {
		t.Error("expected best model to be set")
	}
}

func TestGrading(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"},
		{85, "B"},
		{75, "C"},
		{65, "D"},
		{45, "F"},
	}
	for _, tt := range tests {
		if g := scoreToGrade(tt.score); g != tt.grade {
			t.Errorf("scoreToGrade(%.1f) = %s, want %s", tt.score, g, tt.grade)
		}
	}
}

func TestFormatText(t *testing.T) {
	findings := []model.Finding{
		makeFinding("test-model", "Comprehensively covering all aspects."),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Recall Analysis") {
		t.Error("expected header in output")
	}
	if !strings.Contains(text, "test-model") {
		t.Error("expected model name in output")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No recall data") {
		t.Error("expected empty message")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{
		makeFinding("test-model", "Thoroughly analyzing all components."),
	}
	r := Analyze(findings)
	j := FormatJSON(r)
	if !strings.Contains(j, "recall_score") {
		t.Error("expected recall_score in JSON")
	}
}

func TestCountPhrases(t *testing.T) {
	text := "Comprehensively addressing all aspects. Furthermore, also considering the edge cases."
	count := countPhrases(text, completenessPhrases)
	if count < 1 {
		t.Errorf("expected at least 1 completeness phrase, got %d", count)
	}
}

func TestScoreResponse(t *testing.T) {
	score, detail := scoreResponse("Comprehensively addressing all aspects. Every component thoroughly analyzed.", "")
	if score < 50 {
		t.Errorf("expected decent score for comprehensive response, got %.1f", score)
	}
	if detail.CompletenessCount == 0 {
		t.Error("expected completeness count > 0")
	}
}
