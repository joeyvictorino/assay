package sourcing

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
		finding("gpt-4", "according to the official documentation, the api reference shows version 2.0"),
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
		finding("model-a", "according to the specification, as documented in the official reference"),
		finding("model-b", "i think maybe it works this way probably"),
	})
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.Models[0].SourcingScore < r.Models[1].SourcingScore {
		t.Error("models should be sorted descending by score")
	}
	if r.BestModel != r.Models[0].Model {
		t.Errorf("best model mismatch: %s vs %s", r.BestModel, r.Models[0].Model)
	}
	if r.WorstModel != r.Models[len(r.Models)-1].Model {
		t.Errorf("worst model mismatch")
	}
}

func TestScoreResponseHighCitation(t *testing.T) {
	text := "according to the official documentation, as stated in the specification, referenced in the technical report authored by the research team"
	score, detail := scoreResponse(text, "")
	if score < 50 {
		t.Errorf("well-cited response should score >= 50, got %.1f", score)
	}
	if detail.CitedCount == 0 {
		t.Error("expected nonzero cited count")
	}
}

func TestScoreResponseLowCitation(t *testing.T) {
	text := "i think this is how it works, probably, as far as i know it seems correct"
	score, detail := scoreResponse(text, "")
	if score > 50 {
		t.Errorf("poorly-cited response should score <= 50, got %.1f", score)
	}
	if detail.UncitedCount == 0 {
		t.Error("expected nonzero uncited count")
	}
}

func TestScoreResponseVagueSources(t *testing.T) {
	text := "some sources say this works, many experts think it is widely believed to be correct"
	_, detail := scoreResponse(text, "")
	if detail.VagueCount == 0 {
		t.Error("expected nonzero vague count")
	}
}

func TestScoreResponseWithEvidence(t *testing.T) {
	response := "the vulnerability exists"
	evidence := "according to the official documentation, confirmed by peer-reviewed research"
	score, _ := scoreResponse(response, evidence)
	scoreNoEvidence, _ := scoreResponse(response, "")
	if score <= scoreNoEvidence {
		t.Errorf("adding cited evidence should increase score: %.1f vs %.1f", score, scoreNoEvidence)
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

	long := strings.Repeat("i think probably maybe i believe as far as i know ", 50)
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
	text := "i think probably maybe i believe as far as i know from what i understand"
	r := Analyze([]model.Finding{
		finding("m1", text),
	})
	if r.Models[0].Concerns == 0 {
		_, d := scoreResponse(text, text)
		if d.UncitedCount > 3 {
			t.Errorf("expected concerns > 0 for highly uncited response")
		}
	}
}

func TestMeanScore(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "according to the specification"),
		finding("m2", "i think maybe"),
	})
	expected := (r.Models[0].SourcingScore + r.Models[1].SourcingScore) / 2
	if math.Abs(r.MeanScore-expected) > 0.2 {
		t.Errorf("mean score %.1f doesn't match expected %.1f", r.MeanScore, expected)
	}
}

func TestFormatTextEmpty(t *testing.T) {
	out := FormatText(Report{})
	if !strings.Contains(out, "No sourcing data") {
		t.Error("expected 'No sourcing data' message")
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("test-model", "according to the documentation"),
	})
	out := FormatText(r)
	if !strings.Contains(out, "Source Attribution Quality Analysis") {
		t.Error("expected header in output")
	}
	if !strings.Contains(out, "test-model") {
		t.Error("expected model name in output")
	}
	if !strings.Contains(out, "citation_density") {
		t.Error("expected detail fields in output")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("test-model", "according to the specification"),
	})
	out := FormatJSON(r)
	if !strings.Contains(out, "sourcing_score") {
		t.Error("expected sourcing_score in JSON")
	}
	if !strings.Contains(out, "test-model") {
		t.Error("expected model name in JSON")
	}
}

func TestCountPhrases(t *testing.T) {
	text := "according to the official docs, according to a study"
	count := countPhrases(text, []string{"according to"})
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestHighVarietyScore(t *testing.T) {
	text := "this research paper and peer-reviewed journal article from a technical report and case study confirm the finding"
	_, detail := scoreResponse(text, "")
	if detail.SourceVariety < 0.2 {
		t.Errorf("expected source variety > 0.2, got %.2f", detail.SourceVariety)
	}
}

func TestTraceabilityHighWhenClean(t *testing.T) {
	text := "the api returns status 200 for valid requests"
	_, detail := scoreResponse(text, "")
	if detail.Traceability < 0.5 {
		t.Errorf("expected high traceability for clean text, got %.2f", detail.Traceability)
	}
}

func TestAccuracyBoost(t *testing.T) {
	text := "verified against the specification, cross-referenced with the official docs, confirmed by testing"
	_, detail := scoreResponse(text, "")
	if detail.ReferenceAccuracy <= 0.2 {
		t.Errorf("expected accuracy boost, got %.2f", detail.ReferenceAccuracy)
	}
}
