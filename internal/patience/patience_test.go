package patience

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
	text := "first, do this then, do that and then, finish"
	n := countPhrases(text, []string{"first,", "then,"})
	if n != 3 {
		t.Errorf("countPhrases = %d, want 3", n)
	}
}

func TestAnalyzeSample(t *testing.T) {
	text := "First, start by checking the logs. Second, note that the error code matters. For example, a timeout indicates network issues. If you need more help, feel free to ask."
	d := analyzeSample(text)
	if d.StepByStep < 2 {
		t.Errorf("StepByStep = %d, want >= 2", d.StepByStep)
	}
	if d.TeachingCues < 1 {
		t.Errorf("TeachingCues = %d, want >= 1", d.TeachingCues)
	}
	if d.Elaboration < 1 {
		t.Errorf("Elaboration = %d, want >= 1", d.Elaboration)
	}
	if d.FollowUp < 1 {
		t.Errorf("FollowUp = %d, want >= 1", d.FollowUp)
	}
	if d.TotalWords == 0 {
		t.Error("TotalWords should not be 0")
	}
	if d.PatienceRatio <= 0 {
		t.Error("PatienceRatio should be positive")
	}
}

func TestAnalyzeSampleEmpty(t *testing.T) {
	d := analyzeSample("")
	if d.TotalWords != 0 {
		t.Errorf("TotalWords = %d, want 0", d.TotalWords)
	}
	if d.PatienceRatio != 0 {
		t.Errorf("PatienceRatio = %f, want 0", d.PatienceRatio)
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
		finding("gpt-4", "First, start by reviewing the code. Second, note that the error is important. For example, check the logs. If you need help, feel free to ask."),
		finding("gpt-4", "Step 1 do this. Step 2 do that. To clarify, the key takeaway is to be thorough. You might also check the documentation."),
		finding("claude", "It works."),
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
		finding("terse", "yes ok fine"),
		finding("patient", "First, start by checking the issue. Second, note that the root cause matters. To clarify, for example consider a timeout scenario. The key takeaway is to be thorough. If you need further help, feel free to ask. You might also try the documentation. Additionally, remember that step 1 is always to reproduce the problem. After that, moving on to the fix is straightforward."),
	}
	r := Analyze(findings)
	if len(r.Models) < 2 {
		t.Fatal("expected at least 2 models")
	}
	if r.Models[0].PatienceScore < r.Models[1].PatienceScore {
		t.Error("models should be sorted by patience score descending")
	}
	if r.BestModel != "patient" {
		t.Errorf("BestModel = %q, want patient", r.BestModel)
	}
}

func TestPatientCount(t *testing.T) {
	findings := []model.Finding{
		finding("m", "First, start by doing step 1. Second, note that it's important to understand. To clarify, for example consider the edge case. The key takeaway is patience. If you need more help, feel free to ask for further guidance. You might also review the docs. Additionally, moving on to step 3 is straightforward."),
		finding("m", "ok"),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatal("expected 1 model")
	}
	if r.Models[0].PatientCount != 1 {
		t.Errorf("PatientCount = %d, want 1", r.Models[0].PatientCount)
	}
}

func TestFormatText(t *testing.T) {
	findings := []model.Finding{
		finding("model-a", "First, check the logs. Note that errors matter."),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Patience Analysis") {
		t.Error("expected header in text output")
	}
	if !strings.Contains(text, "model-a") {
		t.Error("expected model name in text output")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{
		finding("model-b", "Step 1 check things. Note that details matter."),
	}
	r := Analyze(findings)
	j := FormatJSON(r)
	if !strings.Contains(j, "model-b") {
		t.Error("expected model name in JSON output")
	}
	if !strings.Contains(j, "patience_score") {
		t.Error("expected patience_score field in JSON output")
	}
}
