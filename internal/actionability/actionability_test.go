package actionability

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

func TestHighActionability(t *testing.T) {
	findings := []model.Finding{
		mkFinding("remediation-model",
			"SQL injection in login endpoint",
			"To fix this SQL injection, replace string concatenation with parameterized queries. "+
				"Step 1: Update the query to use `cursor.execute(query, (username,))` instead of "+
				"f-string interpolation. Step 2: Add input validation by sanitizing the username parameter. "+
				"Upgrade to the latest ORM version which includes built-in escaping."),
	}
	r := Analyze(findings)
	if r.Models[0].ActionabilityScore < 50 {
		t.Errorf("actionable response should score well, got %.1f", r.Models[0].ActionabilityScore)
	}
}

func TestLowActionability(t *testing.T) {
	findings := []model.Finding{
		mkFinding("vague-model",
			"Security issues in the application",
			"There are some security problems that should be looked into."),
	}
	r := Analyze(findings)
	if r.Models[0].ActionabilityScore > 60 {
		t.Errorf("vague response should score low, got %.1f", r.Models[0].ActionabilityScore)
	}
}

func TestActionTermScoreHigh(t *testing.T) {
	score := actionTermScore("Fix the vulnerability by updating the library, sanitize inputs, and restrict access to the endpoint.")
	if score < 50 {
		t.Errorf("action-rich text should score high, got %.1f", score)
	}
}

func TestActionTermScoreLow(t *testing.T) {
	score := actionTermScore("Something seems wrong somewhere.")
	if score > 30 {
		t.Errorf("vague text should score low on action terms, got %.1f", score)
	}
}

func TestRemedyPhraseScoreHigh(t *testing.T) {
	score := remedyPhraseScore("To fix this issue, the parameter should be sanitized. The recommended solution is to use parameterized queries instead of string concatenation.")
	if score < 50 {
		t.Errorf("remedy-rich text should score high, got %.1f", score)
	}
}

func TestRemedyPhraseScoreLow(t *testing.T) {
	score := remedyPhraseScore("found a problem")
	if score > 40 {
		t.Errorf("low-remedy text should score low, got %.1f", score)
	}
}

func TestCodeExampleScoreHigh(t *testing.T) {
	score := codeExampleScore("Use ```python\ncursor.execute(query, (username,))``` to parameterize the query. Import the sanitizer from the library.")
	if score < 40 {
		t.Errorf("code-rich text should score higher, got %.1f", score)
	}
}

func TestCodeExampleScoreLow(t *testing.T) {
	score := codeExampleScore("something is wrong")
	if score > 30 {
		t.Errorf("no-code text should score low, got %.1f", score)
	}
}

func TestStepGuidanceScoreHigh(t *testing.T) {
	score := stepGuidanceScore("Step 1: Identify the vulnerable endpoint. Step 2: Add input validation. Step 3: Deploy the fix. Finally, verify the patch.")
	if score < 50 {
		t.Errorf("step-rich text should score high, got %.1f", score)
	}
}

func TestStepGuidanceScoreLow(t *testing.T) {
	score := stepGuidanceScore("bad")
	if score > 30 {
		t.Errorf("no-steps text should score low, got %.1f", score)
	}
}

func TestMultipleModels(t *testing.T) {
	findings := []model.Finding{
		mkFinding("actionable-model",
			"XSS in search",
			"To fix this XSS, sanitize the output by encoding HTML entities. Replace `innerHTML` with `textContent`. Upgrade to a framework with built-in escaping."),
		mkFinding("vague-model",
			"XSS in search",
			"Something is wrong with the search page."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.MostActionable == r.LeastActionable {
		t.Error("most and least actionable should differ")
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

func TestIsActionable(t *testing.T) {
	if !isActionable("Fix the SQL injection by updating to parameterized queries. Replace the vulnerable code. To fix this, sanitize input and restrict access. The recommended mitigation is to upgrade the library.") {
		t.Error("action-rich text should be flagged as actionable")
	}
}

func TestIsVagueRemedy(t *testing.T) {
	if !isVagueRemedy("something seems bad") {
		t.Error("vague text should be flagged as vague remedy")
	}
}

func TestFormatText(t *testing.T) {
	findings := []model.Finding{
		mkFinding("test-model", "test lab", "fix the issue by updating the library and sanitizing inputs."),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Actionability") {
		t.Error("expected header in text output")
	}
	if !strings.Contains(text, "test-model") {
		t.Error("expected model name in text output")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{
		mkFinding("test-model", "test lab", "fix the issue by updating the library."),
	}
	r := Analyze(findings)
	out := FormatJSON(r)
	if !strings.Contains(out, `"actionability_score"`) {
		t.Error("expected actionability_score in JSON output")
	}
}

func TestEmptyModelName(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{}, Lab: "test", Summary: "fix the issue."},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown' for empty model, got %q", r.Models[0].Model)
	}
}

func TestVagueCountTracking(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m1", "vuln", "something seems bad maybe"),
		mkFinding("m1", "vuln", "Fix the SQL injection by updating to parameterized queries. To remediate this, replace the deprecated function."),
	}
	r := Analyze(findings)
	if r.Models[0].VagueCount < 1 {
		t.Errorf("expected at least 1 vague finding, got %d", r.Models[0].VagueCount)
	}
}
