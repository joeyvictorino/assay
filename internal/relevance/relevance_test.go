package relevance

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(m, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: m},
		Summary: summary,
	}
}

func TestCountWords(t *testing.T) {
	if got := countWords("one two three"); got != 3 {
		t.Errorf("countWords = %d, want 3", got)
	}
	if got := countWords(""); got != 0 {
		t.Errorf("countWords empty = %d, want 0", got)
	}
}

func TestCountPhrases(t *testing.T) {
	got := countPhrases("sql injection and another sql injection", []string{"sql injection"})
	if got != 2 {
		t.Errorf("countPhrases = %d, want 2", got)
	}
}

func TestAnalyzeSampleRelevant(t *testing.T) {
	d := analyzeSample("SQL injection vulnerability in the authentication endpoint allows remote code execution. Remediation: sanitize input parameter.")
	if d.SecurityCount == 0 {
		t.Error("expected security terms")
	}
	if d.TechnicalCount == 0 {
		t.Error("expected technical terms")
	}
	if d.ActionCount == 0 {
		t.Error("expected action terms")
	}
}

func TestAnalyzeSampleDigression(t *testing.T) {
	d := analyzeSample("By the way this is off topic and as an aside here is a fun fact on a different note.")
	if d.DigressionCount == 0 {
		t.Error("expected digression markers")
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Errorf("expected 0 models, got %d", len(r.Models))
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "SQL injection vulnerability in login endpoint."),
		finding("gpt-4", "XSS in the response header parameter."),
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
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		finding("focused", "SQL injection vulnerability in authentication endpoint requires input sanitization."),
		finding("rambling", "By the way on a different note as an aside here is a fun fact incidentally speaking of which."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" || r.WorstModel == "" {
		t.Error("expected best and worst models")
	}
}

func TestRelevantScoresHigher(t *testing.T) {
	relevant := []model.Finding{
		finding("relevant", "SQL injection vulnerability in the authentication endpoint allows privilege escalation via unsanitized parameter input."),
		finding("relevant", "Cross-site scripting exploit in response header. Remediation: implement input validation and encoding."),
	}
	irrelevant := []model.Finding{
		finding("irrelevant", "By the way on a different note as an aside here is something unrelated. Speaking of which, incidentally, this reminds me of nothing."),
		finding("irrelevant", "Fun fact tangentially not directly related but on another note changing the subject before i forget."),
	}
	rr := Analyze(relevant)
	ir := Analyze(irrelevant)
	if rr.Models[0].Score <= ir.Models[0].Score {
		t.Errorf("relevant score %.2f should be > irrelevant score %.2f", rr.Models[0].Score, ir.Models[0].Score)
	}
}

func TestGrade(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{0.96, "A+"},
		{0.91, "A"},
		{0.86, "A-"},
		{0.81, "B+"},
		{0.76, "B"},
		{0.71, "B-"},
		{0.66, "C+"},
		{0.61, "C"},
		{0.56, "C-"},
		{0.51, "D"},
		{0.40, "F"},
	}
	for _, tt := range tests {
		if got := grade(tt.score); got != tt.want {
			t.Errorf("grade(%.2f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestFormatText(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("test-model", "Vulnerability in endpoint."),
	})
	text := FormatText(r)
	if !strings.Contains(text, "Relevance Analysis") {
		t.Error("missing title")
	}
	if !strings.Contains(text, "test-model") {
		t.Error("missing model name")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	text := FormatText(Report{})
	if !strings.Contains(text, "No models analyzed") {
		t.Error("expected empty message")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("test-model", "Finding about vulnerability."),
	})
	j := FormatJSON(r)
	if !strings.Contains(j, "test-model") {
		t.Error("missing model in JSON")
	}
}

func TestScoreBounds(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m", "word"),
	})
	for _, m := range r.Models {
		if m.Score < 0 || m.Score > 1 {
			t.Errorf("score out of bounds: %.2f", m.Score)
		}
	}
}

func TestMeanScore(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("a", "Vulnerability exploit."),
		finding("b", "Another injection."),
	})
	if r.MeanScore < 0 || r.MeanScore > 1 {
		t.Errorf("mean score out of bounds: %.2f", r.MeanScore)
	}
}

func TestSortedByScore(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("irrelevant", "By the way on a different note tangentially speaking of which incidentally."),
		finding("relevant", "SQL injection vulnerability in authentication endpoint parameter."),
	})
	if len(r.Models) == 2 && r.Models[0].Score < r.Models[1].Score {
		t.Error("models should be sorted by score descending")
	}
}

func TestDigressionPenalty(t *testing.T) {
	clean := analyzeSample("SQL injection vulnerability in endpoint parameter.")
	dirty := analyzeSample("SQL injection vulnerability in endpoint parameter. By the way on a different note as an aside.")
	if dirty.DigressionCount == 0 {
		t.Error("expected digressions detected")
	}
	if dirty.RelevanceRatio >= clean.RelevanceRatio {
		t.Error("digression should lower relevance ratio")
	}
}
