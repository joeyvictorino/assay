package verbosityscore

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func mkFinding(modelName, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: modelName},
		Summary: summary,
	}
}

func TestEmptyFindings(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
}

func TestShortResponseHighScore(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "SQL injection found in login endpoint."),
		mkFinding("gpt-4", "XSS via reflected parameter."),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].VerbosityScore < 70 {
		t.Errorf("short response should score high, got %.1f", r.Models[0].VerbosityScore)
	}
}

func TestVerboseResponseLowScore(t *testing.T) {
	long := strings.Repeat("It is important to note that the vulnerability exists and we should be aware of this fact. ", 20)
	findings := []model.Finding{
		mkFinding("verbose-model", long),
	}
	r := Analyze(findings)
	if r.Models[0].VerbosityScore > 60 {
		t.Errorf("verbose response should score low, got %.1f", r.Models[0].VerbosityScore)
	}
}

func TestPaddingDetection(t *testing.T) {
	text := "It is important to note that SQL injection was found. It should be noted that this is critical. As mentioned earlier, the endpoint is vulnerable."
	count := countPaddingPhrases(text)
	if count < 3 {
		t.Errorf("expected at least 3 padding phrases, got %d", count)
	}
}

func TestDisclaimerDetection(t *testing.T) {
	text := "As an AI, I cannot verify this directly. Please consult a security expert. Disclaimer: this is automated analysis."
	count := countDisclaimerPhrases(text)
	if count < 3 {
		t.Errorf("expected at least 3 disclaimers, got %d", count)
	}
}

func TestRepetitionPenalty(t *testing.T) {
	repeated := "the vulnerability is critical. the vulnerability is critical. the vulnerability is critical."
	findings := []model.Finding{
		mkFinding("repeater", repeated),
	}
	r := Analyze(findings)
	if r.Models[0].Breakdown.RepetitionScore >= 90 {
		t.Errorf("repeated text should lower repetition score, got %.1f", r.Models[0].Breakdown.RepetitionScore)
	}
}

func TestMultipleModels(t *testing.T) {
	findings := []model.Finding{
		mkFinding("concise-model", "SQL injection in login."),
		mkFinding("verbose-model", strings.Repeat("It is important to note that there is a vulnerability here. ", 15)),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" || r.WorstModel == "" {
		t.Error("expected best/worst model to be set")
	}
	if r.BestModel == r.WorstModel {
		t.Error("best and worst models should differ")
	}
}

func TestGradeAssignment(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"},
		{85, "B"},
		{75, "C"},
		{65, "D"},
		{45, "F"},
	}
	for _, tc := range tests {
		got := scoreToGrade(tc.score)
		if got != tc.grade {
			t.Errorf("scoreToGrade(%.0f) = %q, want %q", tc.score, got, tc.grade)
		}
	}
}

func TestFormatText(t *testing.T) {
	findings := []model.Finding{
		mkFinding("test-model", "Simple finding."),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Verbosity Score") {
		t.Error("expected header in text output")
	}
	if !strings.Contains(text, "test-model") {
		t.Error("expected model name in text output")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{
		mkFinding("test-model", "SQL injection found."),
	}
	r := Analyze(findings)
	out := FormatJSON(r)
	if !strings.Contains(out, `"verbosity_score"`) {
		t.Error("expected verbosity_score in JSON output")
	}
	if !strings.Contains(out, `"test-model"`) {
		t.Error("expected model name in JSON output")
	}
}

func TestNoPaddingHighScore(t *testing.T) {
	findings := []model.Finding{
		mkFinding("clean-model", "Buffer overflow in parse_header allows remote code execution via crafted HTTP request."),
	}
	r := Analyze(findings)
	if r.Models[0].Breakdown.PaddingScore < 90 {
		t.Errorf("no-padding response should score high on padding, got %.1f", r.Models[0].Breakdown.PaddingScore)
	}
}

func TestEmptyModelName(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{}, Summary: "test finding"},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown' for empty model, got %q", r.Models[0].Model)
	}
}
