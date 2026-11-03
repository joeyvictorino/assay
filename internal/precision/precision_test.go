package precision

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

func TestHighPrecision(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "The answer is specifically that the buffer overflow occurs at line 42. As shown in the stack trace, the result is a segfault."),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].PrecisionScore < 60 {
		t.Errorf("expected high precision score, got %.1f", r.Models[0].PrecisionScore)
	}
}

func TestLowPrecision(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-3", "By the way, it goes without saying that obviously, on a related note, to elaborate further, let me explain in more detail, needless to say as an aside incidentally speaking of which"),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].PrecisionScore > 50 {
		t.Errorf("expected low precision score for filler-heavy response, got %.1f", r.Models[0].PrecisionScore)
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
		makeFinding("model-a", "The answer is specifically the root cause. As shown in the logs."),
		makeFinding("model-b", "By the way, incidentally, on a related note, tangentially."),
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
		makeFinding("test-model", "The answer is specifically clear."),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Precision Analysis") {
		t.Error("expected header in output")
	}
	if !strings.Contains(text, "test-model") {
		t.Error("expected model name in output")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No precision data") {
		t.Error("expected empty message")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{
		makeFinding("test-model", "Specifically the answer is clear."),
	}
	r := Analyze(findings)
	j := FormatJSON(r)
	if !strings.Contains(j, "precision_score") {
		t.Error("expected precision_score in JSON")
	}
}

func TestCountPhrases(t *testing.T) {
	text := "The answer is specifically clear. The answer is correct."
	count := countPhrases(text, directAnswerPhrases)
	if count < 2 {
		t.Errorf("expected at least 2 direct answer phrases, got %d", count)
	}
}

func TestScoreResponse(t *testing.T) {
	score, detail := scoreResponse("The answer is specifically the buffer overflow at line 42.", "")
	if score < 50 {
		t.Errorf("expected decent score for precise response, got %.1f", score)
	}
	if detail.DirectAnswerCount == 0 {
		t.Error("expected direct answer count > 0")
	}
}
