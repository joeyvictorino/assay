package temporal

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(m, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: m},
		Summary: summary,
		Lab:     "test-lab",
	}
}

func TestCountPhrases(t *testing.T) {
	text := "before this event and after that event occurred previously"
	got := countPhrases(text, chronologicalPhrases)
	if got < 2 {
		t.Errorf("expected >=2 chronological phrases, got %d", got)
	}
}

func TestCountPhrasesEmpty(t *testing.T) {
	got := countPhrases("", chronologicalPhrases)
	if got != 0 {
		t.Errorf("expected 0 phrases for empty text, got %d", got)
	}
}

func TestCountPhrasesCaseInsensitive(t *testing.T) {
	text := "BEFORE THIS and AFTER THAT happened PREVIOUSLY"
	got := countPhrases(text, chronologicalPhrases)
	if got < 2 {
		t.Errorf("expected >=2 chronological phrases (case-insensitive), got %d", got)
	}
}

func TestScoreResponseHighTemporal(t *testing.T) {
	text := "before this occurred previously subsequently in chronological order step by step the next phase lasted for the delay caused timing led to"
	score, detail := scoreResponse(text, "")
	if score < 50 {
		t.Errorf("expected high-temporal score >=50, got %.1f", score)
	}
	if detail.TemporalCount < 5 {
		t.Errorf("expected temporal count >=5, got %d", detail.TemporalCount)
	}
}

func TestScoreResponseLowTemporal(t *testing.T) {
	text := "the cat sat on the mat"
	score, detail := scoreResponse(text, "")
	if detail.TemporalCount != 0 {
		t.Errorf("expected 0 temporal count, got %d", detail.TemporalCount)
	}
	if score > 70 {
		t.Errorf("expected moderate score for neutral text, got %.1f", score)
	}
}

func TestScoreResponseAnachronism(t *testing.T) {
	text := "contradictory timeline temporal inconsistency anachronistic out of chronological order conflicting dates impossible sequence"
	score, detail := scoreResponse(text, "")
	if detail.AnachronismCount < 4 {
		t.Errorf("expected anachronism count >=4, got %d", detail.AnachronismCount)
	}
	if score > 50 {
		t.Errorf("expected low score for anachronistic text, got %.1f", score)
	}
}

func TestScoreResponseVague(t *testing.T) {
	text := "at some point sometime around roughly when vaguely timed at an unspecified time loosely timed"
	_, detail := scoreResponse(text, "")
	if detail.VagueCount < 4 {
		t.Errorf("expected vague count >=4, got %d", detail.VagueCount)
	}
}

func TestScoreResponseBounds(t *testing.T) {
	score1, _ := scoreResponse("", "")
	if score1 < 0 || score1 > 100 {
		t.Errorf("score out of bounds: %.1f", score1)
	}

	longText := strings.Repeat("before this after that previously subsequently ", 100)
	score2, _ := scoreResponse(longText, "")
	if score2 > 100 {
		t.Errorf("score exceeds 100: %.1f", score2)
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
	for _, tc := range tests {
		got := scoreToGrade(tc.score)
		if got != tc.grade {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tc.score, got, tc.grade)
		}
	}
}

func TestEvidenceText(t *testing.T) {
	f := model.Finding{
		Summary: "fallback summary",
		Evidence: []model.Evidence{
			{Marker: "marker one"},
			{Marker: "marker two"},
		},
	}
	got := evidenceText(f)
	if got != "marker one marker two" {
		t.Errorf("unexpected evidence text: %s", got)
	}
}

func TestEvidenceTextFallback(t *testing.T) {
	f := model.Finding{Summary: "my summary"}
	got := evidenceText(f)
	if got != "my summary" {
		t.Errorf("expected fallback to summary, got: %s", got)
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Errorf("expected no models for nil input")
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "before this the events occurred in chronological order previously"),
		finding("gpt-4", "step by step the sequence progressed through each phase subsequently"),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.BestModel != "gpt-4" {
		t.Errorf("expected best model gpt-4, got %s", r.BestModel)
	}
	if r.Models[0].SampleCount != 2 {
		t.Errorf("expected 2 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		finding("model-a", "before this previously subsequently in chronological order first occurred then followed"),
		finding("model-b", "the answer is 42"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == r.WorstModel {
		t.Errorf("best and worst should differ with different quality responses")
	}
	if r.Models[0].TemporalScore < r.Models[1].TemporalScore {
		t.Errorf("models should be sorted descending by score")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		{Summary: "before this previously in sequence", Lab: "lab1"},
	}
	r := Analyze(findings)
	if r.BestModel != "unknown" {
		t.Errorf("expected unknown model, got %s", r.BestModel)
	}
}

func TestAnalyzeLabCount(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Summary: "test", Lab: "lab1"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "test", Lab: "lab2"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "test", Lab: "lab1"},
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestAnalyzeConcerns(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "contradictory timeline temporal inconsistency anachronistic out of chronological order conflicting dates impossible sequence timeline contradiction misordered events"),
	}
	r := Analyze(findings)
	if r.Models[0].Concerns < 1 {
		t.Errorf("expected concerns for anachronistic response")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Report{}
	text := FormatText(r)
	if !strings.Contains(text, "No temporal reasoning data") {
		t.Errorf("expected empty message in text output")
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("test-model", "before this the events in chronological order subsequently"),
	})
	text := FormatText(r)
	if !strings.Contains(text, "test-model") {
		t.Errorf("expected model name in text output")
	}
	if !strings.Contains(text, "chronological_accuracy") {
		t.Errorf("expected detail fields in text output")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "before this and subsequently in sequence"),
	})
	j := FormatJSON(r)
	if !strings.Contains(j, "temporal_score") {
		t.Errorf("expected temporal_score in JSON output")
	}
	if !strings.Contains(j, "chronological_accuracy") {
		t.Errorf("expected chronological_accuracy in JSON output")
	}
}

func TestMeanScoreCalculation(t *testing.T) {
	findings := []model.Finding{
		finding("a", "before this previously subsequently in chronological order"),
		finding("b", "the cat sat on the mat"),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Errorf("expected positive mean score, got %.1f", r.MeanScore)
	}
}

func TestSequenceDetection(t *testing.T) {
	text := "step by step in order the first step the next phase sequential from start to finish"
	_, detail := scoreResponse(text, "")
	if detail.SequenceUnderstanding < 0.3 {
		t.Errorf("expected higher sequence understanding score, got %.2f", detail.SequenceUnderstanding)
	}
}
