package creativity

import (
	"encoding/json"
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
	text := "What if we imagine a novel approach from a different angle"
	got := countPhrases(text, novelPhrases)
	if got < 3 {
		t.Errorf("expected >= 3 novel hits, got %d", got)
	}
}

func TestCountPhrasesZero(t *testing.T) {
	text := "the function returns an integer value"
	got := countPhrases(text, novelPhrases)
	if got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestCountClichePhrases(t *testing.T) {
	text := "Let's leverage synergy and think outside the box using best practices"
	got := countPhrases(text, clichePhrases)
	if got < 3 {
		t.Errorf("expected >= 3 cliche hits, got %d", got)
	}
}

func TestScoreResponseCreative(t *testing.T) {
	text := "What if we imagine a novel approach? For example, picture this: like a river finding its path, the algorithm adapts."
	score, detail := scoreResponse(text, "")
	if detail.NovelCount == 0 {
		t.Error("expected novel count > 0")
	}
	if detail.MetaphorCount == 0 {
		t.Error("expected metaphor count > 0")
	}
	_ = score
}

func TestScoreResponseCliche(t *testing.T) {
	text := "We need to leverage synergy and paradigm shift to move the needle using best practices and streamline the process"
	_, detail := scoreResponse(text, "")
	if detail.ClicheCount == 0 {
		t.Error("expected cliche count > 0")
	}
}

func TestScoreResponseBounded(t *testing.T) {
	score, _ := scoreResponse("test", "")
	if score < 0 || score > 100 {
		t.Errorf("score out of bounds: %.1f", score)
	}

	heavy := strings.Repeat("leverage synergy paradigm shift best practices ", 30)
	score2, _ := scoreResponse(heavy, "")
	if score2 < 0 || score2 > 100 {
		t.Errorf("heavy cliche score out of bounds: %.1f", score2)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"}, {85, "B"}, {75, "C"}, {65, "D"}, {50, "F"},
	}
	for _, tt := range tests {
		got := scoreToGrade(tt.score)
		if got != tt.grade {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tt.score, got, tt.grade)
		}
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Errorf("expected empty models")
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "Imagine a novel approach"),
		finding("gpt-4", "What if we try something different"),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].SampleCount != 2 {
		t.Errorf("expected 2 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		finding("model-a", "Imagine a fresh perspective"),
		finding("model-b", "The standard approach per the documentation"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" || r.WorstModel == "" {
		t.Error("expected best and worst model set")
	}
}

func TestAnalyzeSortedDescending(t *testing.T) {
	findings := []model.Finding{
		finding("boring", "Per the specification, the standard approach as expected"),
		finding("creative", "Imagine what if we try a novel approach from a different angle"),
	}
	r := Analyze(findings)
	if len(r.Models) < 2 {
		t.Skip("not enough models")
	}
	if r.Models[0].CreativityScore < r.Models[1].CreativityScore {
		t.Errorf("expected descending sort")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		{Summary: "test response", Lab: "lab1"},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown, got %s", r.Models[0].Model)
	}
}

func TestAnalyzeLabCount(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Summary: "test", Lab: "lab1"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "test2", Lab: "lab2"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "test3", Lab: "lab1"},
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestAnalyzeConcerns(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "Leverage synergy paradigm shift move the needle best practices streamline utilize deep dive"),
	}
	r := Analyze(findings)
	if r.Models[0].Concerns == 0 {
		t.Error("expected concerns > 0 for cliche-heavy response")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "Creative") {
		t.Error("expected header")
	}
	if !strings.Contains(text, "No creativity data") {
		t.Error("expected empty message")
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	r := Analyze([]model.Finding{finding("gpt-4", "Imagine a creative solution")})
	text := FormatText(r)
	if !strings.Contains(text, "gpt-4") {
		t.Error("expected model name in output")
	}
	if !strings.Contains(text, "novelty") {
		t.Error("expected detail field")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]model.Finding{finding("gpt-4", "A creative response")})
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(parsed.Models) != 1 {
		t.Errorf("expected 1 model in JSON")
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

func TestEvidenceText(t *testing.T) {
	f := model.Finding{
		Summary: "fallback",
		Evidence: []model.Evidence{
			{Marker: "marker one"},
		},
	}
	got := evidenceText(f)
	if !strings.Contains(got, "marker one") {
		t.Errorf("expected marker, got %s", got)
	}
}

func TestEvidenceTextFallback(t *testing.T) {
	f := model.Finding{Summary: "the summary"}
	got := evidenceText(f)
	if got != "the summary" {
		t.Errorf("expected summary fallback, got %s", got)
	}
}

func TestGradeEdgeCases(t *testing.T) {
	if scoreToGrade(90) != "A" {
		t.Error("90 should be A")
	}
	if scoreToGrade(89.9) != "B" {
		t.Error("89.9 should be B")
	}
}
