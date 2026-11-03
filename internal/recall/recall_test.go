package recall

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestCountPhrases(t *testing.T) {
	text := "first, additionally, furthermore the edge case and corner case matter"
	if n := countPhrases(text, coveragePhrases); n != 3 {
		t.Errorf("expected 3 coverage phrases, got %d", n)
	}
	if n := countPhrases(text, edgeCasePhrases); n != 2 {
		t.Errorf("expected 2 edge case phrases, got %d", n)
	}
}

func TestCountPhrasesEmpty(t *testing.T) {
	if n := countPhrases("", coveragePhrases); n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

func TestCountPhrasesCaseInsensitive(t *testing.T) {
	text := "FIRST Additionally FURTHERMORE"
	if n := countPhrases(text, coveragePhrases); n != 3 {
		t.Errorf("expected 3, got %d", n)
	}
}

func TestScoreResponseHighCoverage(t *testing.T) {
	resp := "First, regarding the API. Additionally, with respect to security. Furthermore, each of the comprehensive aspects covers all edge case and corner case scenarios. In detail, specifically, for example the underlying reason is therefore clear."
	score, detail := scoreResponse(resp, "")
	if score < 50 {
		t.Errorf("high-coverage response should score >50, got %.1f", score)
	}
	if detail.CoveredCount == 0 {
		t.Error("expected covered_count > 0")
	}
}

func TestScoreResponseLowCoverage(t *testing.T) {
	resp := "Yes."
	score, _ := scoreResponse(resp, "")
	if score > 60 {
		t.Errorf("low-coverage response should score <=60, got %.1f", score)
	}
}

func TestScoreResponseWithOmissions(t *testing.T) {
	resp := "I won't cover that. Beyond the scope. Let's focus on briefly in short tldr."
	score, detail := scoreResponse(resp, "")
	if detail.OmittedCount == 0 {
		t.Error("expected omitted_count > 0")
	}
	if score > 70 {
		t.Errorf("omission-heavy response should score <70, got %.1f", score)
	}
}

func TestScoreResponseWithPartials(t *testing.T) {
	resp := "Partially sort of kind of somewhat approximately more or less roughly."
	_, detail := scoreResponse(resp, "")
	if detail.PartialCount == 0 {
		t.Error("expected partial_count > 0")
	}
}

func TestScoreResponseGapResistanceClean(t *testing.T) {
	resp := "First, the API returns a JSON response. Additionally, each field is documented."
	_, detail := scoreResponse(resp, "")
	if detail.GapResistance < 0.7 {
		t.Errorf("clean response gap_resistance should be >=0.7, got %.2f", detail.GapResistance)
	}
}

func TestScoreResponseGapResistanceDirty(t *testing.T) {
	resp := "I won't cover that. Beyond the scope. I'll skip the details. Not going into it."
	_, detail := scoreResponse(resp, "")
	if detail.GapResistance > 0.5 {
		t.Errorf("dirty response gap_resistance should be <=0.5, got %.2f", detail.GapResistance)
	}
}

func TestScoreResponseBounds(t *testing.T) {
	score, _ := scoreResponse("", "")
	if score < 0 || score > 100 {
		t.Errorf("score out of bounds: %.1f", score)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"}, {90, "A"}, {85, "B"}, {80, "B"},
		{75, "C"}, {70, "C"}, {65, "D"}, {60, "D"},
		{55, "F"}, {0, "F"},
	}
	for _, tt := range tests {
		if g := scoreToGrade(tt.score); g != tt.grade {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tt.score, g, tt.grade)
		}
	}
}

func TestEvidenceTextWithMarkers(t *testing.T) {
	f := model.Finding{
		Summary: "fallback",
		Evidence: []model.Evidence{
			{Marker: "marker1"},
			{Marker: "marker2"},
		},
	}
	if got := evidenceText(f); got != "marker1 marker2" {
		t.Errorf("expected 'marker1 marker2', got %q", got)
	}
}

func TestEvidenceTextFallback(t *testing.T) {
	f := model.Finding{Summary: "the summary"}
	if got := evidenceText(f); got != "the summary" {
		t.Errorf("expected 'the summary', got %q", got)
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Error("expected empty report for nil findings")
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "gpt-4"}, Lab: "lab1", Summary: "First, regarding the topic. Additionally, the edge case is important."},
		{Model: model.ModelRef{Model: "gpt-4"}, Lab: "lab2", Summary: "Furthermore, in detail specifically for example."},
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
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "model-a"}, Lab: "lab1", Summary: "First, additionally, furthermore, moreover, also in detail specifically."},
		{Model: model.ModelRef{Model: "model-b"}, Lab: "lab1", Summary: "Yes."},
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" || r.WorstModel == "" {
		t.Error("expected best and worst model set")
	}
	if r.BestModel == r.WorstModel {
		t.Error("best and worst should differ for different-quality responses")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		{Lab: "lab1", Summary: "test content"},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown', got %s", r.Models[0].Model)
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Report{}
	text := FormatText(r)
	if !strings.Contains(text, "No recall data") {
		t.Error("expected empty-data message")
	}
}

func TestFormatTextWithModels(t *testing.T) {
	r := Report{
		Models: []ModelRecall{
			{Model: "test-model", RecallScore: 75.5, Grade: "C", SampleCount: 5, LabCount: 3},
		},
		BestModel: "test-model", WorstModel: "test-model", MeanScore: 75.5,
	}
	text := FormatText(r)
	if !strings.Contains(text, "test-model") {
		t.Error("expected model name in output")
	}
	if !strings.Contains(text, "75.5") {
		t.Error("expected score in output")
	}
	if !strings.Contains(text, "topic_coverage") {
		t.Error("expected detail fields in output")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Report{
		Models:    []ModelRecall{{Model: "m1", RecallScore: 80}},
		BestModel: "m1", WorstModel: "m1", MeanScore: 80,
	}
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.BestModel != "m1" {
		t.Errorf("expected m1, got %s", parsed.BestModel)
	}
}

func TestFormatJSONEmpty(t *testing.T) {
	r := Report{}
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestDepthPhrases(t *testing.T) {
	text := "In detail, specifically, for example the underlying reason is clear."
	if n := countPhrases(text, depthPhrases); n < 3 {
		t.Errorf("expected >=3 depth phrases, got %d", n)
	}
}

func TestExhaustivePhrases(t *testing.T) {
	text := "This comprehensive review covers all aspects entirely without exception."
	if n := countPhrases(text, exhaustivePhrases); n < 3 {
		t.Errorf("expected >=3 exhaustive phrases, got %d", n)
	}
}

func TestConcernsFlag(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Lab: "l1", Summary: "I won't cover that. Beyond the scope. I'll skip it. Not going into details. Out of scope."},
	}
	r := Analyze(findings)
	if r.Models[0].Concerns == 0 {
		t.Error("expected concerns > 0 for high-omission response")
	}
}
