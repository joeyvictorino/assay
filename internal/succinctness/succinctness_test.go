package succinctness

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func mkFinding(modelName, lab, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: modelName},
		Lab:     lab,
		Summary: summary,
	}
}

func TestEmptyFindings(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
}

func TestSuccinctResponse(t *testing.T) {
	findings := []model.Finding{
		mkFinding("concise-model", "SQL injection in login",
			"SQL injection found in login endpoint"),
	}
	r := Analyze(findings)
	if r.Models[0].SuccinctnessScore < 50 {
		t.Errorf("succinct response should score well, got %.1f", r.Models[0].SuccinctnessScore)
	}
}

func TestVerboseResponse(t *testing.T) {
	findings := []model.Finding{
		mkFinding("verbose-model", "XSS in search",
			"It is worth noting that, as a matter of fact, the search functionality, which is fundamentally important, "+
				"basically has an issue where cross-site scripting vulnerabilities exist. In summary, to summarize, "+
				"the search search search search search search search search search endpoint has problems. "+
				"Overall, it should be noted that this is essentially a concern that needs addressing."),
	}
	r := Analyze(findings)
	if r.Models[0].SuccinctnessScore > 70 {
		t.Errorf("verbose response should score low, got %.1f", r.Models[0].SuccinctnessScore)
	}
}

func TestRedundancyDetection(t *testing.T) {
	findings := []model.Finding{
		mkFinding("repeater", "buffer overflow",
			"buffer buffer buffer buffer overflow overflow overflow overflow vulnerability vulnerability vulnerability"),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.RedundancyPen > 80 {
		t.Errorf("redundant response should have low redundancy score, got %.1f", r.Models[0].Detail.RedundancyPen)
	}
}

func TestFillerDetection(t *testing.T) {
	filler := "It is worth noting that, needless to say, basically, essentially, fundamentally this is a finding."
	score := fillerPenalty(filler)
	if score > 40 {
		t.Errorf("filler-heavy text should score low, got %.1f", score)
	}
}

func TestFillerClean(t *testing.T) {
	clean := "SQL injection vulnerability in the login endpoint allows authentication bypass."
	score := fillerPenalty(clean)
	if score < 90 {
		t.Errorf("clean text should score high, got %.1f", score)
	}
}

func TestBrevityShort(t *testing.T) {
	score := brevityScore("SQL injection in login", "SQL injection found")
	if score < 90 {
		t.Errorf("short response should have high brevity, got %.1f", score)
	}
}

func TestBrevityLong(t *testing.T) {
	longSummary := strings.Repeat("word ", 100)
	score := brevityScore("SQL injection", longSummary)
	if score > 30 {
		t.Errorf("very long response should have low brevity, got %.1f", score)
	}
}

func TestMultipleModels(t *testing.T) {
	findings := []model.Finding{
		mkFinding("concise", "CSRF missing", "CSRF protection missing on form endpoint"),
		mkFinding("wordy", "CSRF missing",
			"It is worth noting that basically the CSRF CSRF CSRF protection is fundamentally missing "+
				"on the form endpoint endpoint endpoint. In summary, needless to say, this is a problem."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.MostSuccinct == r.LeastSuccinct {
		t.Error("most and least succinct should differ")
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
		mkFinding("test-model", "test lab", "test summary"),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Succinctness") {
		t.Error("expected header in text output")
	}
	if !strings.Contains(text, "test-model") {
		t.Error("expected model name in text output")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{
		mkFinding("test-model", "test lab", "test summary"),
	}
	r := Analyze(findings)
	out := FormatJSON(r)
	if !strings.Contains(out, `"succinctness_score"`) {
		t.Error("expected succinctness_score in JSON output")
	}
}

func TestEmptyModelName(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{}, Lab: "test", Summary: "test"},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown' for empty model, got %q", r.Models[0].Model)
	}
}
