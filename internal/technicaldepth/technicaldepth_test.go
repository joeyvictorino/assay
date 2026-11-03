package technicaldepth

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(mid, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: mid},
		Summary: summary,
	}
}

func TestCountWords(t *testing.T) {
	if n := countWords("hello world foo"); n != 3 {
		t.Errorf("countWords = %d, want 3", n)
	}
	if n := countWords(""); n != 0 {
		t.Errorf("countWords empty = %d, want 0", n)
	}
}

func TestCountPhrases(t *testing.T) {
	text := "the algorithm uses encryption and another algorithm"
	n := countPhrases(text, []string{"algorithm", "encryption"})
	if n != 3 {
		t.Errorf("countPhrases = %d, want 3", n)
	}
}

func TestCountPhrasesEmpty(t *testing.T) {
	n := countPhrases("hello world", []string{"missing"})
	if n != 0 {
		t.Errorf("countPhrases = %d, want 0", n)
	}
}

func TestAnalyzeSample(t *testing.T) {
	text := "The algorithm implementation uses encryption because the underlying protocol requires authentication"
	d := analyzeSample(text)
	if d.TechnicalTerms < 3 {
		t.Errorf("TechnicalTerms = %d, want >= 3", d.TechnicalTerms)
	}
	if d.MechanismWords < 1 {
		t.Errorf("MechanismWords = %d, want >= 1", d.MechanismWords)
	}
	if d.TotalWords == 0 {
		t.Error("TotalWords should not be 0")
	}
	if d.DepthRatio <= 0 {
		t.Error("DepthRatio should be positive")
	}
}

func TestAnalyzeSampleEmpty(t *testing.T) {
	d := analyzeSample("")
	if d.TotalWords != 0 {
		t.Errorf("TotalWords = %d, want 0", d.TotalWords)
	}
	if d.DepthRatio != 0 {
		t.Errorf("DepthRatio = %f, want 0", d.DepthRatio)
	}
}

func TestGrade(t *testing.T) {
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
			t.Errorf("grade(%.2f) = %q, want %q", tt.score, got, tt.want)
		}
	}
}

func TestAnalyze(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "The algorithm uses encryption because the underlying protocol needs authentication"),
		finding("gpt-4", "The implementation of the hash function specifically addresses the buffer overflow vulnerability"),
		finding("claude", "It works fine and is good"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("Models = %d, want 2", len(r.Models))
	}
	if r.BestModel == "" {
		t.Error("BestModel should not be empty")
	}
	if r.WorstModel == "" {
		t.Error("WorstModel should not be empty")
	}
	if r.MeanScore <= 0 {
		t.Error("MeanScore should be positive")
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Errorf("Models = %d, want 0", len(r.Models))
	}
	if r.BestModel != "" {
		t.Errorf("BestModel = %q, want empty", r.BestModel)
	}
}

func TestAnalyzeSorting(t *testing.T) {
	findings := []model.Finding{
		finding("shallow", "yes ok fine"),
		finding("deep", "The algorithm implementation uses encryption because the underlying protocol requires cryptographic authentication and the hash function addresses the buffer overflow vulnerability specifically in the stack trace"),
	}
	r := Analyze(findings)
	if len(r.Models) < 2 {
		t.Fatal("expected at least 2 models")
	}
	if r.Models[0].DepthScore < r.Models[1].DepthScore {
		t.Error("models should be sorted by depth score descending")
	}
	if r.BestModel != "deep" {
		t.Errorf("BestModel = %q, want deep", r.BestModel)
	}
}

func TestFormatText(t *testing.T) {
	findings := []model.Finding{
		finding("model-a", "The algorithm uses encryption because the protocol requires it"),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Technical Depth Analysis") {
		t.Error("expected header in text output")
	}
	if !strings.Contains(text, "model-a") {
		t.Error("expected model name in text output")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{
		finding("model-b", "The implementation handles the vulnerability"),
	}
	r := Analyze(findings)
	j := FormatJSON(r)
	if !strings.Contains(j, "model-b") {
		t.Error("expected model name in JSON output")
	}
	if !strings.Contains(j, "depth_score") {
		t.Error("expected depth_score field in JSON output")
	}
}

func TestDeepCount(t *testing.T) {
	findings := []model.Finding{
		finding("m", "The algorithm implementation uses encryption because the underlying protocol requires cryptographic authentication and the hash function addresses the buffer overflow vulnerability specifically in the stack trace and the root cause is the memory leak"),
		finding("m", "ok"),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatal("expected 1 model")
	}
	if r.Models[0].DeepCount != 1 {
		t.Errorf("DeepCount = %d, want 1", r.Models[0].DeepCount)
	}
}
