package conciseness

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(id, modelName, summary string) model.Finding {
	return model.Finding{
		ID:       id,
		Class:    "other",
		Severity: model.SevLow,
		Model:    model.ModelRef{Model: modelName},
		Summary:  summary,
		State:    model.StateValidated,
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
	if r.MeanScore != 0 {
		t.Fatalf("expected 0 mean, got %.1f", r.MeanScore)
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	f := []model.Finding{
		finding("f1", "model-a", "The server responds with a clear error code."),
	}
	r := Analyze(f)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "model-a" {
		t.Errorf("expected model-a, got %s", r.Models[0].Model)
	}
	if len(r.Models[0].Dimensions) != 4 {
		t.Errorf("expected 4 dimensions, got %d", len(r.Models[0].Dimensions))
	}
}

func TestAnalyzeMultiModel(t *testing.T) {
	f := []model.Finding{
		finding("f1", "model-a", "Short and direct response to the query."),
		finding("f2", "model-a", "Another concise reply with relevant data."),
		finding("f3", "model-b", "Verbose wordy explanation text here."),
	}
	r := Analyze(f)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.MostConcise == "" {
		t.Error("expected most concise to be set")
	}
	if r.LeastConcise == "" {
		t.Error("expected least concise to be set")
	}
}

func TestDimensionNames(t *testing.T) {
	f := []model.Finding{
		finding("f1", "model-a", "sample text for conciseness analysis scoring"),
	}
	r := Analyze(f)
	want := map[string]bool{
		"word_efficiency":  true,
		"redundancy_score": true,
		"filler_ratio":     true,
		"directness":       true,
	}
	for _, d := range r.Models[0].Dimensions {
		if !want[d.Name] {
			t.Errorf("unexpected dimension %q", d.Name)
		}
		delete(want, d.Name)
	}
	for name := range want {
		t.Errorf("missing dimension %q", name)
	}
}

func TestScoresInRange(t *testing.T) {
	f := []model.Finding{
		finding("f1", "m", "Some text with various words for analysis and conciseness testing."),
		finding("f2", "m", "More text with different patterns and word choices for evaluation."),
	}
	r := Analyze(f)
	for _, m := range r.Models {
		if m.OverallScore < 0 || m.OverallScore > 100 {
			t.Errorf("overall score %.1f out of range", m.OverallScore)
		}
		for _, d := range m.Dimensions {
			if d.Score < 0 || d.Score > 100 {
				t.Errorf("dimension %s score %.1f out of range", d.Name, d.Score)
			}
		}
	}
}

func TestUnknownModel(t *testing.T) {
	f := []model.Finding{
		{ID: "f1", Class: "other", Severity: model.SevLow, State: model.StateValidated, Summary: "test text"},
	}
	r := Analyze(f)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown', got %q", r.Models[0].Model)
	}
}

func TestGradeAssignment(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{100, "A"}, {90, "A"}, {89, "B"}, {80, "B"},
		{79, "C"}, {70, "C"}, {69, "D"}, {60, "D"},
		{59, "F"}, {0, "F"},
	}
	for _, tt := range tests {
		got := scoreToGrade(tt.score)
		if got != tt.want {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestSampleCount(t *testing.T) {
	f := []model.Finding{
		finding("f1", "m", "sample one"),
		finding("f2", "m", "sample two"),
		finding("f3", "m", "sample three"),
	}
	r := Analyze(f)
	if r.Models[0].SampleCount != 3 {
		t.Errorf("expected 3 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestFormatTextOutput(t *testing.T) {
	f := []model.Finding{
		finding("f1", "model-a", "conciseness test sample output text"),
	}
	r := Analyze(f)
	text := FormatText(r)
	if !strings.Contains(text, "Conciseness") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "model-a") {
		t.Error("missing model name")
	}
	if !strings.Contains(text, "word_efficiency") {
		t.Error("missing dimension name")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No conciseness data") {
		t.Error("expected empty message")
	}
}

func TestFormatJSON(t *testing.T) {
	f := []model.Finding{
		finding("f1", "model-a", "text for json format testing"),
	}
	r := Analyze(f)
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(parsed.Models) != 1 {
		t.Errorf("expected 1 model, got %d", len(parsed.Models))
	}
}

func TestFillerDetection(t *testing.T) {
	clean := []string{"The server returned an error code."}
	filly := []string{"Actually basically the server really just returned an error code obviously."}
	cleanScore := fillerRatio(clean)
	fillyScore := fillerRatio(filly)
	if fillyScore >= cleanScore {
		t.Errorf("filler-heavy text (%.1f) should score lower than clean (%.1f)", fillyScore, cleanScore)
	}
}

func TestRedundancyDetection(t *testing.T) {
	unique := []string{"The cat sat down. Dogs run in the park. Birds fly south."}
	redundant := []string{"The cat sat down. The cat sat down. The cat sat down."}
	uniqueScore := redundancyScore(unique)
	redundantScore := redundancyScore(redundant)
	if redundantScore >= uniqueScore {
		t.Errorf("redundant text (%.1f) should score lower than unique (%.1f)", redundantScore, uniqueScore)
	}
}

func TestWordOverlap(t *testing.T) {
	a := map[string]bool{"cat": true, "dog": true, "bird": true}
	b := map[string]bool{"cat": true, "dog": true, "fish": true}
	o := wordOverlap(a, b)
	if o < 0.6 || o > 0.7 {
		t.Errorf("expected ~0.67 overlap, got %.2f", o)
	}
	if wordOverlap(nil, b) != 0 {
		t.Error("expected 0 for nil")
	}
}

func TestSplitSentences(t *testing.T) {
	s := splitSentences("Hello world. How are you? Fine!")
	if len(s) != 3 {
		t.Errorf("expected 3 sentences, got %d", len(s))
	}
}
