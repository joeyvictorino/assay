package watermark

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
		finding("f1", "model-a", "The quick brown fox jumps over the lazy dog repeatedly"),
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
		finding("f1", "model-a", "Long text with many varied words and diverse vocabulary for analysis purposes"),
		finding("f2", "model-a", "Another substantial text with different content for consistency measurement scoring"),
		finding("f3", "model-b", "Short txt"),
	}
	r := Analyze(f)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.StrongestMark == "" {
		t.Error("expected strongest mark to be set")
	}
	if r.WeakestMark == "" {
		t.Error("expected weakest mark to be set")
	}
}

func TestDimensionNames(t *testing.T) {
	f := []model.Finding{
		finding("f1", "model-a", "sample text for watermark detection analysis"),
	}
	r := Analyze(f)
	want := map[string]bool{
		"watermark_presence":    true,
		"watermark_strength":    true,
		"watermark_consistency": true,
		"watermark_resilience":  true,
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
		finding("f1", "m", "Some text with various words for analysis and watermark detection testing purposes"),
		finding("f2", "m", "More text with different patterns and word choices for evaluation metrics"),
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
		finding("f1", "model-a", "watermark test sample output text"),
	}
	r := Analyze(f)
	text := FormatText(r)
	if !strings.Contains(text, "Watermark") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "model-a") {
		t.Error("missing model name")
	}
	if !strings.Contains(text, "watermark_presence") {
		t.Error("missing dimension name")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No watermark data") {
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

func TestTokenDistEntropy(t *testing.T) {
	if e := tokenDistEntropy(""); e != 0 {
		t.Errorf("empty string entropy = %.2f, want 0", e)
	}
	e := tokenDistEntropy("aaaa")
	if e != 0 {
		t.Errorf("single-char entropy = %.2f, want 0", e)
	}
	e2 := tokenDistEntropy("abcdefghijklmnopqrstuvwxyz")
	if e2 <= 0 {
		t.Errorf("alphabet entropy = %.2f, want > 0", e2)
	}
}

func TestBigramRepeatRatio(t *testing.T) {
	if r := bigramRepeatRatio(""); r != 0 {
		t.Errorf("empty ratio = %.2f, want 0", r)
	}
	if r := bigramRepeatRatio("one"); r != 0 {
		t.Errorf("single word ratio = %.2f, want 0", r)
	}
	r := bigramRepeatRatio("a b a b a b")
	if r <= 0 {
		t.Errorf("repeated bigram ratio = %.2f, want > 0", r)
	}
}

func TestConsistencyMultipleSamples(t *testing.T) {
	f := []model.Finding{
		finding("f1", "m", "The quick brown fox jumps over the lazy dog in the park"),
		finding("f2", "m", "The quick brown fox jumps over the lazy dog in the garden"),
		finding("f3", "m", "The quick brown fox jumps over the lazy dog in the yard"),
	}
	r := Analyze(f)
	var cons float64
	for _, d := range r.Models[0].Dimensions {
		if d.Name == "watermark_consistency" {
			cons = d.Score
		}
	}
	if cons < 50 {
		t.Errorf("similar texts should have high consistency, got %.1f", cons)
	}
}
