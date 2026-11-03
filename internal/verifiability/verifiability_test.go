package verifiability

import (
	"math"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(modelName, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: modelName},
		Summary: summary,
		Lab:     "test-lab",
	}
}

func findingWithEvidence(modelName, summary, marker string) model.Finding {
	return model.Finding{
		Model:    model.ModelRef{Model: modelName},
		Summary:  summary,
		Lab:      "test-lab",
		Evidence: []model.Evidence{{Marker: marker}},
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Errorf("expected empty models, got %d", len(r.Models))
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("gpt-4", "to verify this, run the following command and check the output, the result should be deterministic"),
	})
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "gpt-4" {
		t.Errorf("expected model gpt-4, got %s", r.Models[0].Model)
	}
	if r.BestModel != "gpt-4" {
		t.Errorf("expected best model gpt-4, got %s", r.BestModel)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("model-a", "to verify this, run the following test case and check the output is deterministic and reproducible"),
		finding("model-b", "i think in my opinion it seems to me this is obviously self-evident"),
	})
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.Models[0].VerifiabilityScore < r.Models[1].VerifiabilityScore {
		t.Error("models should be sorted descending by score")
	}
	if r.BestModel != r.Models[0].Model {
		t.Errorf("best model mismatch: %s vs %s", r.BestModel, r.Models[0].Model)
	}
	if r.WorstModel != r.Models[len(r.Models)-1].Model {
		t.Errorf("worst model mismatch")
	}
}

func TestScoreResponseHighVerifiability(t *testing.T) {
	text := "to verify this, run the following command and check the output. The result should be deterministic and reproducible with the same output every time. The empirical evidence shows a confidence interval of 95%."
	score, detail := scoreResponse(text, "")
	if score < 50 {
		t.Errorf("highly verifiable response should score >= 50, got %.1f", score)
	}
	if detail.VerifiableCount == 0 {
		t.Error("expected nonzero verifiable count")
	}
}

func TestScoreResponseLowVerifiability(t *testing.T) {
	text := "i think in my opinion this is obviously self-evident and everyone knows it goes without saying that trust me on this"
	score, detail := scoreResponse(text, "")
	if score > 50 {
		t.Errorf("unverifiable response should score <= 50, got %.1f", score)
	}
	if detail.UnverifiableCount == 0 {
		t.Error("expected nonzero unverifiable count")
	}
}

func TestScoreResponseSubjective(t *testing.T) {
	text := "i think i believe in my opinion personally from my perspective this is debatable"
	_, detail := scoreResponse(text, "")
	if detail.SubjectiveCount == 0 {
		t.Error("expected nonzero subjective count")
	}
}

func TestScoreResponseWithEvidence(t *testing.T) {
	response := "the vulnerability exists"
	evidence := "to verify this, run the following reproducible test case with deterministic results"
	score, _ := scoreResponse(response, evidence)
	scoreNoEvidence, _ := scoreResponse(response, "")
	if score <= scoreNoEvidence {
		t.Errorf("adding verifiable evidence should increase score: %.1f vs %.1f", score, scoreNoEvidence)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"},
		{90, "A"},
		{85, "B"},
		{80, "B"},
		{75, "C"},
		{70, "C"},
		{65, "D"},
		{60, "D"},
		{55, "F"},
		{0, "F"},
	}
	for _, tt := range tests {
		got := scoreToGrade(tt.score)
		if got != tt.grade {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tt.score, got, tt.grade)
		}
	}
}

func TestScoreBounds(t *testing.T) {
	score1, _ := scoreResponse("", "")
	if score1 < 0 || score1 > 100 {
		t.Errorf("score out of bounds: %.1f", score1)
	}

	long := strings.Repeat("i think in my opinion i believe personally trust me obviously ", 50)
	score2, _ := scoreResponse(long, "")
	if score2 < 0 {
		t.Errorf("score should not be negative: %.1f", score2)
	}
}

func TestEvidenceText(t *testing.T) {
	f := model.Finding{
		Summary: "test summary",
		Evidence: []model.Evidence{
			{Marker: "marker1"},
			{Marker: "marker2"},
		},
	}
	got := evidenceText(f)
	if got != "marker1 marker2" {
		t.Errorf("expected 'marker1 marker2', got %q", got)
	}
}

func TestEvidenceTextFallback(t *testing.T) {
	f := model.Finding{
		Summary: "test summary",
	}
	got := evidenceText(f)
	if got != "test summary" {
		t.Errorf("expected summary fallback, got %q", got)
	}
}

func TestUnknownModelName(t *testing.T) {
	f := model.Finding{
		Summary: "test",
		Lab:     "lab1",
	}
	r := Analyze([]model.Finding{f})
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown', got %s", r.Models[0].Model)
	}
}

func TestLabCounting(t *testing.T) {
	r := Analyze([]model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Summary: "test", Lab: "lab-a"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "test", Lab: "lab-b"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "test", Lab: "lab-a"},
	})
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
	if r.Models[0].SampleCount != 3 {
		t.Errorf("expected 3 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestConcernCounting(t *testing.T) {
	text := "trust me obviously everyone knows no need to verify it goes without saying clearly self-evident in my experience intuitively common sense suggests"
	r := Analyze([]model.Finding{
		finding("m1", text),
	})
	_, d := scoreResponse(text, text)
	if d.UnverifiableCount > 3 && r.Models[0].Concerns == 0 {
		t.Error("expected concerns > 0 for highly unverifiable response")
	}
}

func TestMeanScore(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "to verify this run the following command"),
		finding("m2", "i think maybe obviously"),
	})
	expected := (r.Models[0].VerifiabilityScore + r.Models[1].VerifiabilityScore) / 2
	if math.Abs(r.MeanScore-expected) > 0.2 {
		t.Errorf("mean score %.1f doesn't match expected %.1f", r.MeanScore, expected)
	}
}

func TestFormatTextEmpty(t *testing.T) {
	out := FormatText(Report{})
	if !strings.Contains(out, "No verifiability data") {
		t.Error("expected 'No verifiability data' message")
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("test-model", "to verify this run the test"),
	})
	out := FormatText(r)
	if !strings.Contains(out, "Response Verifiability Analysis") {
		t.Error("expected header in output")
	}
	if !strings.Contains(out, "test-model") {
		t.Error("expected model name in output")
	}
	if !strings.Contains(out, "testability_score") {
		t.Error("expected detail fields in output")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("test-model", "to verify this check the output"),
	})
	out := FormatJSON(r)
	if !strings.Contains(out, "verifiability_score") {
		t.Error("expected verifiability_score in JSON")
	}
	if !strings.Contains(out, "test-model") {
		t.Error("expected model name in JSON")
	}
}

func TestCountPhrases(t *testing.T) {
	text := "to verify this, to verify this again"
	count := countPhrases(text, []string{"to verify this"})
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestFalsifiabilityBoost(t *testing.T) {
	text := "this fails when the input is empty, which is an edge case and a known limitation with a specific failure mode"
	_, detail := scoreResponse(text, "")
	if detail.FalsifiabilityScore <= 0.2 {
		t.Errorf("expected falsifiability boost, got %.2f", detail.FalsifiabilityScore)
	}
}

func TestMethodTransparencyHighWhenClean(t *testing.T) {
	text := "the function returns the sum of its arguments"
	_, detail := scoreResponse(text, "")
	if detail.MethodTransparency < 0.5 {
		t.Errorf("expected high method transparency for clean text, got %.2f", detail.MethodTransparency)
	}
}

func TestReproducibilityDetection(t *testing.T) {
	text := "deterministic reproducible consistent results with the same output given the same input"
	_, detail := scoreResponse(text, "")
	if detail.ReproducibilityScore <= 0.2 {
		t.Errorf("expected reproducibility score > 0.2, got %.2f", detail.ReproducibilityScore)
	}
}
