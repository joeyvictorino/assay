package selfawareness

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(mdl, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: mdl},
		Lab:     "lab1",
		Summary: summary,
	}
}

func TestCountPhrasesUncertainty(t *testing.T) {
	text := "i'm not sure and i may be wrong about this uncertain claim"
	c := countPhrases(text, uncertaintyPhrases)
	if c < 2 {
		t.Errorf("expected >=2, got %d", c)
	}
}

func TestCountPhrasesLimitation(t *testing.T) {
	text := "i can't verify this as a language model with my knowledge cutoff"
	c := countPhrases(text, limitationPhrases)
	if c < 2 {
		t.Errorf("expected >=2, got %d", c)
	}
}

func TestCountPhrasesCorrection(t *testing.T) {
	text := "actually, let me correct that. upon reflection I was wrong."
	c := countPhrases(text, correctionPhrases)
	if c < 2 {
		t.Errorf("expected >=2, got %d", c)
	}
}

func TestCountPhrasesOverconfidence(t *testing.T) {
	text := "i am absolutely certain and there is no doubt about this"
	c := countPhrases(text, overconfidencePhrases)
	if c < 2 {
		t.Errorf("expected >=2, got %d", c)
	}
}

func TestScoreResponseEmpty(t *testing.T) {
	score, _ := scoreResponse("")
	if score != 0 {
		t.Errorf("expected 0, got %f", score)
	}
}

func TestScoreResponseHighAwareness(t *testing.T) {
	text := "I'm not sure about this. I may be wrong. My knowledge cutoff means I can't verify. Let me reconsider upon reflection."
	score, detail := scoreResponse(text)
	if score <= 50 {
		t.Errorf("expected >50 for self-aware text, got %f", score)
	}
	if detail.UncertaintyCount == 0 {
		t.Error("expected uncertainty count")
	}
}

func TestScoreResponseLowAwareness(t *testing.T) {
	text := "I am absolutely certain and there is no doubt this is the definitive answer that I guarantee is irrefutably correct."
	score, detail := scoreResponse(text)
	if score >= 50 {
		t.Errorf("expected <50 for overconfident text, got %f", score)
	}
	if detail.OverconfidenceCount == 0 {
		t.Error("expected overconfidence count")
	}
}

func TestScoreResponseBounded(t *testing.T) {
	text := strings.Repeat("i'm not sure uncertain i may be wrong ", 100)
	score, _ := scoreResponse(text)
	if score > 100 || score < 0 {
		t.Errorf("score out of bounds: %f", score)
	}
}

func TestScoreToGrade(t *testing.T) {
	cases := []struct {
		score float64
		grade string
	}{
		{95, "A"}, {85, "B"}, {75, "C"}, {65, "D"}, {45, "F"},
	}
	for _, c := range cases {
		if g := scoreToGrade(c.score); g != c.grade {
			t.Errorf("score %.0f: expected %s, got %s", c.score, c.grade, g)
		}
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Error("expected empty report")
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "I'm not sure but my knowledge cutoff means I can't verify this."),
		finding("gpt-4", "Upon reflection, I may be wrong about this uncertain claim."),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].SampleCount != 2 {
		t.Errorf("expected 2 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestAnalyzeMultiModel(t *testing.T) {
	findings := []model.Finding{
		finding("model-a", "I'm not sure and I may be wrong. My knowledge cutoff limits me."),
		finding("model-b", "I am absolutely certain and there is no doubt about this."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.MostAware == r.LeastAware {
		t.Error("most and least should differ")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		{Lab: "lab1", Summary: "I'm not sure."},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown, got %s", r.Models[0].Model)
	}
}

func TestAnalyzeLabCount(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab1", Summary: "test"},
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab2", Summary: "test"},
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestAnalyzeSorted(t *testing.T) {
	findings := []model.Finding{
		finding("low", "I am absolutely certain there is no doubt."),
		finding("high", "I'm not sure and I may be wrong. My knowledge cutoff. Upon reflection, let me reconsider."),
	}
	r := Analyze(findings)
	if r.Models[0].AwarenessScore < r.Models[1].AwarenessScore {
		t.Error("expected sorted descending")
	}
}

func TestAnalyzeMeanScore(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "I'm not sure."),
		finding("m2", "I'm not sure."),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Error("expected positive mean score")
	}
}

func TestAnalyzeHighLowCounts(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "I'm not sure and I may be wrong. My knowledge cutoff. Upon reflection let me reconsider this uncertain claim."),
		finding("m1", "I am absolutely certain there is no doubt I guarantee this is irrefutably the definitive answer."),
	}
	r := Analyze(findings)
	if r.Models[0].HighAwareness == 0 && r.Models[0].LowAwareness == 0 {
		t.Error("expected at least one high or low count")
	}
}

func TestEvidenceTextMarker(t *testing.T) {
	f := model.Finding{
		Summary:  "summary",
		Evidence: []model.Evidence{{Marker: "marker"}},
	}
	if evidenceText(f) != "marker" {
		t.Error("expected marker text")
	}
}

func TestEvidenceTextFallback(t *testing.T) {
	f := model.Finding{Summary: "summary"}
	if evidenceText(f) != "summary" {
		t.Error("expected summary fallback")
	}
}

func TestFormatTextOutput(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "I'm not sure about this."),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Self-Awareness") {
		t.Error("expected 'Self-Awareness' header")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No findings") {
		t.Error("expected 'No findings' message")
	}
}

func TestFormatJSONValid(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "I'm not sure."),
	}
	r := Analyze(findings)
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestFormatJSONEmpty(t *testing.T) {
	r := Analyze(nil)
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}
