package technicaldepth

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(mid, lab, summary string) model.Finding {
	return model.Finding{Model: model.ModelRef{Model: mid}, Lab: lab, Summary: summary}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Error("expected empty models")
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "lab1", "The algorithm uses a hash function with cryptographic properties to prevent injection attacks."),
		finding("gpt-4", "lab2", "The root cause is a buffer overflow vulnerability in the implementation."),
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
	if r.BestModel != "gpt-4" {
		t.Errorf("expected best model gpt-4, got %s", r.BestModel)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		finding("modelA", "lab1", "The algorithm exploits a buffer overflow vulnerability in the implementation's cryptographic hash function."),
		finding("modelB", "lab1", "It was bad."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == r.WorstModel {
		t.Error("best and worst should differ for models with different depth")
	}
}

func TestGradeRanges(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{0.95, "A+"},
		{0.90, "A"},
		{0.85, "A-"},
		{0.80, "B+"},
		{0.75, "B"},
		{0.70, "B-"},
		{0.60, "C"},
		{0.50, "D"},
		{0.30, "F"},
	}
	for _, tt := range tests {
		got := grade(tt.score)
		if got != tt.want {
			t.Errorf("grade(%.2f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestCountPhrases(t *testing.T) {
	text := "The algorithm uses a hash function and another hash function for encryption."
	n := countPhrases(text, []string{"hash function", "encryption"})
	if n != 3 {
		t.Errorf("expected 3 phrase matches, got %d", n)
	}
}

func TestCountWords(t *testing.T) {
	n := countWords("one two three four")
	if n != 4 {
		t.Errorf("expected 4, got %d", n)
	}
}

func TestAnalyzeSample(t *testing.T) {
	d := analyzeSample("The root cause is a buffer overflow because the implementation has a vulnerability.")
	if d.TechnicalTerms == 0 {
		t.Error("expected technical terms")
	}
	if d.MechanismWords == 0 {
		t.Error("expected mechanism words")
	}
	if d.TotalWords == 0 {
		t.Error("expected non-zero word count")
	}
}

func TestLabCount(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "labA", "The algorithm has a vulnerability."),
		finding("m1", "labB", "The implementation has a buffer overflow."),
		finding("m1", "labA", "Another implementation vulnerability exploit."),
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestFormatText(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "lab1", "The algorithm implementation has a vulnerability."),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Technical Depth Analysis") {
		t.Error("expected header")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Error("expected model name in output")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "Models evaluated:  0") {
		t.Error("expected zero models line")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "lab1", "The algorithm vulnerability exploit."),
	}
	r := Analyze(findings)
	j := FormatJSON(r)
	if !strings.Contains(j, "depth_score") {
		t.Error("expected depth_score in JSON")
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "lab1", "The algorithm implementation has a vulnerability using cryptographic hash function."),
		finding("m2", "lab1", "ok"),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Error("expected positive mean score")
	}
}

func TestDeepCount(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "lab1", "short"),
		finding("m1", "lab2", "The algorithm implementation vulnerability exploit buffer overflow hash function cryptographic authentication authorization protocol encryption specifically the root cause because therefore."),
	}
	r := Analyze(findings)
	if r.Models[0].DeepCount == 0 {
		t.Error("expected at least one deep sample")
	}
}
