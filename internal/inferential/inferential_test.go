package inferential

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
	text := "therefore it follows that consequently thus the data shows"
	got := countPhrases(text, deductivePhrases)
	if got < 3 {
		t.Errorf("expected at least 3 deductive phrases, got %d", got)
	}
}

func TestCountPhrasesEmpty(t *testing.T) {
	got := countPhrases("", deductivePhrases)
	if got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestCountPhrasesCaseInsensitive(t *testing.T) {
	text := "THEREFORE it FOLLOWS THAT this is true"
	got := countPhrases(text, deductivePhrases)
	if got < 1 {
		t.Errorf("expected at least 1 match, got %d", got)
	}
}

func TestScoreResponseDeductive(t *testing.T) {
	resp := "Therefore, it follows that the result must be correct. Hence, we can conclude that logically the solution holds. Given that the premises are true, consequently the theorem is valid. Thus by definition necessarily it is certain that this proves the point."
	score, detail := scoreResponse(resp, "")
	if score < 50 {
		t.Errorf("expected score >= 50 for deductive response, got %.1f", score)
	}
	if detail.InferenceCount == 0 {
		t.Error("expected non-zero inference count")
	}
}

func TestScoreResponseInductive(t *testing.T) {
	resp := "Based on the evidence, the data suggests a pattern. The pattern indicates that observations show in most cases this trend. Empirically, from the sample the evidence points to a general rule. Statistically extrapolating from the data we can infer the answer."
	score, detail := scoreResponse(resp, "")
	if score < 50 {
		t.Errorf("expected score >= 50 for inductive response, got %.1f", score)
	}
	if detail.InferenceCount == 0 {
		t.Error("expected non-zero inference count for inductive")
	}
}

func TestScoreResponseAbductive(t *testing.T) {
	resp := "The best explanation is that most likely this would explain the observation. The simplest explanation involves a plausible cause. Probably because the evidence is consistent with the hypothesis, one possible explanation accounts for the symptoms."
	score, detail := scoreResponse(resp, "")
	if score < 45 {
		t.Errorf("expected score >= 45 for abductive response, got %.1f", score)
	}
	if detail.InferenceCount == 0 {
		t.Error("expected non-zero inference count for abductive")
	}
}

func TestScoreResponseFallacious(t *testing.T) {
	resp := "Everyone knows this is true. Obviously it goes without saying that common sense dictates the answer. You can't deny that clearly this proves the point. No one would disagree."
	score, detail := scoreResponse(resp, "")
	if detail.FallacyCount == 0 {
		t.Error("expected non-zero fallacy count")
	}
	if score > 50 {
		t.Errorf("expected lower score with fallacies, got %.1f", score)
	}
}

func TestScoreResponseEmpty(t *testing.T) {
	score, _ := scoreResponse("", "")
	if score < 0 || score > 100 {
		t.Errorf("score out of range: %.1f", score)
	}
}

func TestScoreResponseHedgy(t *testing.T) {
	resp := "It might be the case. Perhaps, possibly, one might think it could be argued that there's a chance. It seems possible, arguably it's debatable whether some might say this is true."
	_, detail := scoreResponse(resp, "")
	if detail.HedgeCount == 0 {
		t.Error("expected non-zero hedge count")
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
		finding("gpt-4", "Therefore the data suggests this result. Based on the evidence, we can conclude the answer."),
		finding("gpt-4", "It follows that the pattern indicates a trend. Hence, supported by research the findings demonstrate validity."),
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
		finding("gpt-4", "Therefore it follows that the data suggests a clear conclusion. As evidenced by the research."),
		finding("claude", "Everyone knows this is true obviously."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" {
		t.Error("expected best model")
	}
	if r.WorstModel == "" {
		t.Error("expected worst model")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		{Lab: "lab1", Summary: "test response"},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown, got %s", r.Models[0].Model)
	}
}

func TestAnalyzeSorted(t *testing.T) {
	findings := []model.Finding{
		finding("low", "a simple response"),
		finding("high", "Therefore it follows that consequently thus hence logically we can conclude. Based on the evidence the data suggests supported by research. As evidenced by documented in confirmed by."),
	}
	r := Analyze(findings)
	if len(r.Models) < 2 {
		t.Fatal("expected 2 models")
	}
	if r.Models[0].InferentialScore < r.Models[1].InferentialScore {
		t.Error("expected sorted descending")
	}
}

func TestAnalyzeLabCount(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m"}, Lab: "lab1", Summary: "therefore"},
		{Model: model.ModelRef{Model: "m"}, Lab: "lab2", Summary: "thus"},
		{Model: model.ModelRef{Model: "m"}, Lab: "lab1", Summary: "hence"},
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestFormatTextEmpty(t *testing.T) {
	out := FormatText(Report{})
	if !strings.Contains(out, "No inferential data") {
		t.Error("expected empty message")
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("test-model", "Therefore the evidence supports the conclusion."),
	})
	out := FormatText(r)
	if !strings.Contains(out, "test-model") {
		t.Error("expected model name")
	}
	if !strings.Contains(out, "Inferential Reasoning") {
		t.Error("expected header")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "therefore it follows that"),
	})
	out := FormatJSON(r)
	if !strings.Contains(out, "inferential_score") {
		t.Error("expected inferential_score in JSON")
	}
	if !strings.Contains(out, "m1") {
		t.Error("expected model in JSON")
	}
}

func TestScoreResponseWithEvidence(t *testing.T) {
	score, _ := scoreResponse("basic response", "therefore the data suggests supported by research as evidenced by the findings")
	if score < 40 {
		t.Errorf("expected evidence to boost score, got %.1f", score)
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		finding("a", "therefore it follows that we can conclude logically as evidenced by research"),
		finding("b", "just a plain answer"),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Errorf("expected positive mean, got %.1f", r.MeanScore)
	}
}

func TestScoreBounds(t *testing.T) {
	texts := []string{
		"",
		"hello",
		strings.Repeat("therefore it follows that based on the evidence supported by ", 50),
		strings.Repeat("everyone knows obviously common sense dictates it goes without saying ", 20),
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
		finding("m", "everyone knows obviously you can't deny common sense dictates"),
		finding("m", "it goes without saying no one would disagree clearly this proves"),
	}
	r := Analyze(findings)
	if r.Models[0].Concerns != 2 {
		t.Errorf("expected 2 concerns for fallacious responses, got %d", r.Models[0].Concerns)
	}
}
