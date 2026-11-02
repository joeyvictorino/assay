package faithfulness

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func mkFinding(modelName, lab, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: modelName},
		Lab:     lab,
		Summary: summary,
	}
}

func TestEmptyFindings(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
}

func TestHighFaithfulness(t *testing.T) {
	findings := []model.Finding{
		mkFinding("faithful-model", "SQL injection in login endpoint via user input",
			"SQL injection vulnerability found in the login endpoint through unsanitized user input"),
	}
	r := Analyze(findings)
	if r.Models[0].FaithfulnessScore < 50 {
		t.Errorf("faithful response should score well, got %.1f", r.Models[0].FaithfulnessScore)
	}
}

func TestLowFaithfulness(t *testing.T) {
	findings := []model.Finding{
		mkFinding("hallucinator", "SQL injection in login",
			"The application probably uses quantum computing algorithms that might process blockchain transactions"),
	}
	r := Analyze(findings)
	if r.Models[0].FaithfulnessScore > 60 {
		t.Errorf("hallucinated response should score low, got %.1f", r.Models[0].FaithfulnessScore)
	}
}

func TestHallucinationDetection(t *testing.T) {
	findings := []model.Finding{
		mkFinding("fabricator", "XSS vulnerability in search",
			"The quantum entanglement processor leverages distributed ledger technology for neural pathway optimization"),
	}
	r := Analyze(findings)
	if r.Models[0].HallucinationCount < 1 {
		t.Errorf("expected hallucination warning, got %d", r.Models[0].HallucinationCount)
	}
}

func TestMultipleModels(t *testing.T) {
	findings := []model.Finding{
		mkFinding("good-model", "CSRF protection missing on form",
			"CSRF protection is missing on the form submission endpoint"),
		mkFinding("bad-model", "CSRF protection missing on form",
			"Machine learning algorithms optimize gradient descent for neural network training"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.MostFaithful == r.LeastFaithful {
		t.Error("most and least faithful should differ")
	}
}

func TestClaimCoverageFunc(t *testing.T) {
	score := claimCoverage("SQL injection vulnerability", "SQL injection found in endpoint")
	if score < 30 {
		t.Errorf("overlapping claims should score > 30, got %.1f", score)
	}
}

func TestExtractKeywordsStopWords(t *testing.T) {
	kw := extractKeywords("the is a an of in for to")
	if len(kw) != 0 {
		t.Errorf("expected 0 keywords from stop words, got %d", len(kw))
	}
}

func TestSourceGroundingSpeculative(t *testing.T) {
	speculative := "I think probably maybe this could be an issue. Perhaps it seems like it might be a vulnerability."
	score := sourceGrounding(speculative)
	if score > 60 {
		t.Errorf("speculative response should score low on grounding, got %.1f", score)
	}
}

func TestSourceGroundingDirect(t *testing.T) {
	direct := "SQL injection vulnerability in the login endpoint allows authentication bypass."
	score := sourceGrounding(direct)
	if score < 90 {
		t.Errorf("direct response should score high on grounding, got %.1f", score)
	}
}

func TestGradeAssignment(t *testing.T) {
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
	for _, tc := range tests {
		got := scoreToGrade(tc.score)
		if got != tc.grade {
			t.Errorf("scoreToGrade(%.0f) = %q, want %q", tc.score, got, tc.grade)
		}
	}
}

func TestFormatText(t *testing.T) {
	findings := []model.Finding{
		mkFinding("test-model", "test lab", "test summary"),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Faithfulness") {
		t.Error("expected header in text output")
	}
	if !strings.Contains(text, "test-model") {
		t.Error("expected model name in text output")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{
		mkFinding("test-model", "test lab", "test summary"),
	}
	r := Analyze(findings)
	out := FormatJSON(r)
	if !strings.Contains(out, `"faithfulness_score"`) {
		t.Error("expected faithfulness_score in JSON output")
	}
}

func TestEmptyModelName(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{}, Lab: "test", Summary: "test"},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown' for empty model, got %q", r.Models[0].Model)
	}
}
