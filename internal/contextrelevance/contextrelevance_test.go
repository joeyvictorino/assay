package contextrelevance

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestCountPhrases(t *testing.T) {
	text := "as you asked, regarding your question, the answer is specifically"
	if n := countPhrases(text, alignmentPhrases); n != 2 {
		t.Errorf("expected 2 alignment phrases, got %d", n)
	}
	if n := countPhrases(text, onTopicPhrases); n != 2 {
		t.Errorf("expected 2 on-topic phrases, got %d", n)
	}
}

func TestCountPhrasesEmpty(t *testing.T) {
	if n := countPhrases("", alignmentPhrases); n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

func TestCountPhrasesCaseInsensitive(t *testing.T) {
	text := "AS YOU ASKED To Answer Your Question"
	if n := countPhrases(text, alignmentPhrases); n != 2 {
		t.Errorf("expected 2, got %d", n)
	}
}

func TestScoreResponseHighRelevance(t *testing.T) {
	resp := "As you asked, regarding your question about the API. Based on the context, the key point is that specifically the answer is directly relevant. In this case, the solution is clear."
	score, detail := scoreResponse(resp, "")
	if score < 50 {
		t.Errorf("high-relevance response should score >50, got %.1f", score)
	}
	if detail.OnTopicCount == 0 {
		t.Error("expected on_topic_count > 0")
	}
}

func TestScoreResponseLowRelevance(t *testing.T) {
	resp := "Hello."
	score, _ := scoreResponse(resp, "")
	if score > 60 {
		t.Errorf("low-relevance response should score <=60, got %.1f", score)
	}
}

func TestScoreResponseWithOffTopic(t *testing.T) {
	resp := "By the way, on a related note, incidentally speaking of which, as an aside, tangentially this is off topic but interesting."
	score, detail := scoreResponse(resp, "")
	if detail.OffTopicCount == 0 {
		t.Error("expected off_topic_count > 0")
	}
	if score > 70 {
		t.Errorf("off-topic response should score <70, got %.1f", score)
	}
}

func TestScoreResponseWithTangents(t *testing.T) {
	resp := "Before i answer, first let me give some background. A brief history going back to the bigger picture, zooming out."
	_, detail := scoreResponse(resp, "")
	if detail.TangentCount == 0 {
		t.Error("expected tangent_count > 0")
	}
}

func TestScoreResponseDistractionClean(t *testing.T) {
	resp := "As you asked, the answer is specifically that the solution is clear."
	_, detail := scoreResponse(resp, "")
	if detail.DistractionScore < 0.7 {
		t.Errorf("clean response distraction_score should be >=0.7, got %.2f", detail.DistractionScore)
	}
}

func TestScoreResponseDistractionDirty(t *testing.T) {
	resp := "By the way, on a related note, incidentally, as an aside."
	_, detail := scoreResponse(resp, "")
	if detail.DistractionScore > 0.5 {
		t.Errorf("dirty response distraction_score should be <=0.5, got %.2f", detail.DistractionScore)
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
		{Model: model.ModelRef{Model: "claude-3"}, Lab: "lab1", Summary: "As you asked, the answer is specifically clear."},
		{Model: model.ModelRef{Model: "claude-3"}, Lab: "lab2", Summary: "Based on the context given the information."},
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "claude-3" {
		t.Errorf("expected claude-3, got %s", r.Models[0].Model)
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
		{Model: model.ModelRef{Model: "model-a"}, Lab: "lab1", Summary: "As you asked, to answer your question, specifically the answer is that this directly relates to the solution. Based on the context, the key point is clear."},
		{Model: model.ModelRef{Model: "model-b"}, Lab: "lab1", Summary: "Hello."},
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" || r.WorstModel == "" {
		t.Error("expected best and worst model set")
	}
	if r.BestModel == r.WorstModel {
		t.Error("best and worst should differ")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		{Lab: "lab1", Summary: "test"},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown', got %s", r.Models[0].Model)
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Report{}
	text := FormatText(r)
	if !strings.Contains(text, "No context relevance data") {
		t.Error("expected empty-data message")
	}
}

func TestFormatTextWithModels(t *testing.T) {
	r := Report{
		Models: []ModelRelevance{
			{Model: "test-model", RelevanceScore: 82.3, Grade: "B", SampleCount: 5, LabCount: 3},
		},
		BestModel: "test-model", WorstModel: "test-model", MeanScore: 82.3,
	}
	text := FormatText(r)
	if !strings.Contains(text, "test-model") {
		t.Error("expected model name")
	}
	if !strings.Contains(text, "82.3") {
		t.Error("expected score")
	}
	if !strings.Contains(text, "topic_alignment") {
		t.Error("expected detail fields")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Report{
		Models:    []ModelRelevance{{Model: "m1", RelevanceScore: 80}},
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

func TestContextUsePhrases(t *testing.T) {
	text := "Based on the context, given the information, as mentioned the data indicates."
	if n := countPhrases(text, contextUsePhrases); n < 3 {
		t.Errorf("expected >=3 context use phrases, got %d", n)
	}
}

func TestFocusPhrases(t *testing.T) {
	text := "The key point is that fundamentally the crux and most importantly the central issue."
	if n := countPhrases(text, focusPhrases); n < 3 {
		t.Errorf("expected >=3 focus phrases, got %d", n)
	}
}

func TestConcernsFlag(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Lab: "l1", Summary: "By the way, on a related note, incidentally speaking of which, as an aside, tangentially off topic but interesting."},
	}
	r := Analyze(findings)
	if r.Models[0].Concerns == 0 {
		t.Error("expected concerns > 0 for high off-topic response")
	}
}
