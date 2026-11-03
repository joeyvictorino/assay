package abstraction

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(m, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: m},
		Lab:     "test-lab",
		Summary: summary,
	}
}

func TestCountPhrases(t *testing.T) {
	text := "at a high level the general idea is conceptually sound"
	got := countPhrases(text, highLevelPhrases)
	if got < 3 {
		t.Errorf("expected at least 3 high-level phrases, got %d", got)
	}
}

func TestCountPhrasesEmpty(t *testing.T) {
	got := countPhrases("", highLevelPhrases)
	if got != 0 {
		t.Errorf("expected 0 for empty text, got %d", got)
	}
}

func TestCountPhrasesCaseInsensitive(t *testing.T) {
	text := "AT A HIGH LEVEL the concept is clear"
	got := countPhrases(text, highLevelPhrases)
	if got < 1 {
		t.Errorf("expected at least 1 match, got %d", got)
	}
}

func TestScoreResponseHighAbstraction(t *testing.T) {
	resp := "At a high level, the overall architecture uses the key abstraction of layers. From a design perspective, the mental model separates the public contract from the private implementation. Zooming out, the scope of the domain model spans the application layer and the infrastructure layer."
	score, detail := scoreResponse(resp, "")
	if score < 50 {
		t.Errorf("expected score >= 50 for abstract response, got %.1f", score)
	}
	if detail.AbstractCount == 0 {
		t.Error("expected non-zero abstract count")
	}
}

func TestScoreResponseConcrete(t *testing.T) {
	resp := "For example, specifically the actual implementation is in this specific case here is the code with the exact steps. The concrete type gives the actual output, running this gives the precise value."
	score, detail := scoreResponse(resp, "")
	if score < 40 {
		t.Errorf("expected score >= 40 for concrete response, got %.1f", score)
	}
	if detail.ConcreteCount == 0 {
		t.Error("expected non-zero concrete count")
	}
}

func TestScoreResponseBalanced(t *testing.T) {
	resp := "At a high level, the overall architecture uses layered design. For example, specifically the api surface exposes methods. Zooming in, the implementation level handles the concrete type."
	score, detail := scoreResponse(resp, "")
	if detail.AbstractCount == 0 {
		t.Error("expected non-zero abstract count in balanced response")
	}
	if detail.ConcreteCount == 0 {
		t.Error("expected non-zero concrete count in balanced response")
	}
	if score < 45 {
		t.Errorf("expected higher score for balanced response, got %.1f", score)
	}
}

func TestScoreResponseEmpty(t *testing.T) {
	score, _ := scoreResponse("", "")
	if score < 0 || score > 100 {
		t.Errorf("score out of range: %.1f", score)
	}
}

func TestScoreResponseConfusion(t *testing.T) {
	resp := "The abstraction leaks here, mixing concerns and conflating the wrong level of detail. Too abstract in some places, premature abstraction."
	score, _ := scoreResponse(resp, "")
	if score > 60 {
		t.Errorf("expected lower score with confusion phrases, got %.1f", score)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"}, {85, "B"}, {75, "C"}, {65, "D"}, {45, "F"},
	}
	for _, tt := range tests {
		got := scoreToGrade(tt.score)
		if got != tt.grade {
			t.Errorf("scoreToGrade(%.1f) = %s, want %s", tt.score, got, tt.grade)
		}
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Errorf("expected 0 models, got %d", len(r.Models))
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "At a high level, the overall architecture is clean. For example, the api surface exposes a clear public contract."),
		finding("gpt-4", "The mental model uses layers from a design perspective. Specifically, the implementation level handles details."),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "gpt-4" {
		t.Errorf("expected gpt-4, got %s", r.Models[0].Model)
	}
	if r.Models[0].SampleCount != 2 {
		t.Errorf("expected 2 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "At a high level the overall architecture is clean. For example the concrete type shows the actual output."),
		finding("claude", "The general idea works in principle. The key abstraction separates concerns at the interface level."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" {
		t.Error("expected best model to be set")
	}
	if r.WorstModel == "" {
		t.Error("expected worst model to be set")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		{Lab: "lab1", Summary: "test response"},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown model, got %s", r.Models[0].Model)
	}
}

func TestAnalyzeSorted(t *testing.T) {
	findings := []model.Finding{
		finding("low", "a simple response"),
		finding("high", "At a high level the overall architecture uses the key abstraction. For example, the public contract and the private implementation show the interface level. Zooming in to scope details."),
	}
	r := Analyze(findings)
	if len(r.Models) < 2 {
		t.Fatal("expected 2 models")
	}
	if r.Models[0].AbstractionScore < r.Models[1].AbstractionScore {
		t.Error("expected sorted by score descending")
	}
}

func TestAnalyzeLabCount(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m"}, Lab: "lab1", Summary: "at a high level"},
		{Model: model.ModelRef{Model: "m"}, Lab: "lab2", Summary: "for example"},
		{Model: model.ModelRef{Model: "m"}, Lab: "lab1", Summary: "the general idea"},
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestFormatTextEmpty(t *testing.T) {
	out := FormatText(Report{})
	if !strings.Contains(out, "No abstraction data") {
		t.Error("expected empty message in output")
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("test-model", "At a high level the overall architecture is good. For example the api surface works."),
	})
	out := FormatText(r)
	if !strings.Contains(out, "test-model") {
		t.Error("expected model name in output")
	}
	if !strings.Contains(out, "Abstraction Quality") {
		t.Error("expected header in output")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "at a high level the general idea"),
	})
	out := FormatJSON(r)
	if !strings.Contains(out, "abstraction_score") {
		t.Error("expected abstraction_score in JSON")
	}
	if !strings.Contains(out, "m1") {
		t.Error("expected model name in JSON")
	}
}

func TestScoreResponseWithEvidence(t *testing.T) {
	score, _ := scoreResponse("basic response", "at a high level the overall architecture uses layers from a design perspective")
	if score < 40 {
		t.Errorf("expected evidence to boost score, got %.1f", score)
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		finding("a", "at a high level the key abstraction separates the public contract from private implementation"),
		finding("b", "just a plain answer"),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Errorf("expected positive mean score, got %.1f", r.MeanScore)
	}
}

func TestScoreBounds(t *testing.T) {
	texts := []string{
		"",
		"hello",
		strings.Repeat("at a high level for example the overall architecture the actual implementation ", 50),
		strings.Repeat("the abstraction leaks mixing concerns conflating wrong level of detail ", 20),
	}
	for _, txt := range texts {
		score, _ := scoreResponse(txt, "")
		if score < 0 || score > 100 {
			t.Errorf("score out of range for text len %d: %.1f", len(txt), score)
		}
	}
}

func TestConcernsCount(t *testing.T) {
	findings := []model.Finding{
		finding("m", "no relevant phrases at all just plain text"),
		finding("m", "another plain response with nothing useful"),
	}
	r := Analyze(findings)
	if r.Models[0].Concerns != 2 {
		t.Errorf("expected 2 concerns for responses with no abstraction/concrete phrases, got %d", r.Models[0].Concerns)
	}
}
