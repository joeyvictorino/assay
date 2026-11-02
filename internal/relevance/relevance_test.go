package relevance

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

func TestHighRelevance(t *testing.T) {
	findings := []model.Finding{
		mkFinding("focused-model", "SQL injection in login endpoint", "SQL injection found in the login endpoint via unsanitized user input"),
	}
	r := Analyze(findings)
	if r.Models[0].RelevanceScore < 50 {
		t.Errorf("high-relevance response should score well, got %.1f", r.Models[0].RelevanceScore)
	}
}

func TestLowRelevance(t *testing.T) {
	findings := []model.Finding{
		mkFinding("drifty-model", "SQL injection in login", "The weather today is sunny with clear skies and temperatures around seventy degrees fahrenheit"),
	}
	r := Analyze(findings)
	if r.Models[0].RelevanceScore > 70 {
		t.Errorf("off-topic response should score low, got %.1f", r.Models[0].RelevanceScore)
	}
}

func TestTopicDrift(t *testing.T) {
	findings := []model.Finding{
		mkFinding("drifter", "XSS vulnerability analysis", "The system uses various cloud computing paradigms and microservice architectures to process data"),
	}
	r := Analyze(findings)
	if r.Models[0].DriftCount < 1 {
		t.Errorf("expected drift warning for off-topic response, got %d", r.Models[0].DriftCount)
	}
}

func TestMultipleModels(t *testing.T) {
	findings := []model.Finding{
		mkFinding("good-model", "CSRF protection missing", "CSRF protection is missing on the form submission endpoint"),
		mkFinding("bad-model", "CSRF protection missing", "Machine learning algorithms optimize gradient descent for neural network training"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.MostRelevant == r.LeastRelevant {
		t.Error("most and least relevant should differ")
	}
}

func TestKeywordOverlapFunc(t *testing.T) {
	labKW := extractKeywords("SQL injection vulnerability")
	sumKW := extractKeywords("SQL injection found in endpoint")
	overlap := keywordOverlap(labKW, sumKW)
	if overlap < 30 {
		t.Errorf("overlapping keywords should score > 30, got %.1f", overlap)
	}
}

func TestExtractKeywordsStopWords(t *testing.T) {
	kw := extractKeywords("the is a an of in for to")
	if len(kw) != 0 {
		t.Errorf("expected 0 keywords from stop words, got %d", len(kw))
	}
}

func TestAnswerDirectnessPenalty(t *testing.T) {
	hedgy := "It depends on various factors. Generally speaking, there are many considerations. On the other hand, it varies."
	score := answerDirectness(hedgy)
	if score > 70 {
		t.Errorf("hedging response should score low on directness, got %.1f", score)
	}
}

func TestAnswerDirectnessClean(t *testing.T) {
	direct := "SQL injection vulnerability in the login endpoint allows authentication bypass."
	score := answerDirectness(direct)
	if score < 90 {
		t.Errorf("direct response should score high, got %.1f", score)
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
	if !strings.Contains(text, "Relevance") {
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
	if !strings.Contains(out, `"relevance_score"`) {
		t.Error("expected relevance_score in JSON output")
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
