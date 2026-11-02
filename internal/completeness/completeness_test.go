package completeness

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

func TestHighCompleteness(t *testing.T) {
	findings := []model.Finding{
		mkFinding("thorough-model",
			"SQL injection vulnerability in login endpoint",
			"SQL injection vulnerability found in the login endpoint. The user input parameter is not sanitized before being concatenated into the SQL query. Specifically, the username field accepts arbitrary SQL syntax. This leads to authentication bypass because an attacker can inject conditions that always evaluate to true."),
	}
	r := Analyze(findings)
	if r.Models[0].CompletenessScore < 50 {
		t.Errorf("thorough response should score well, got %.1f", r.Models[0].CompletenessScore)
	}
}

func TestLowCompleteness(t *testing.T) {
	findings := []model.Finding{
		mkFinding("terse-model",
			"SQL injection in login endpoint with authentication bypass and data exfiltration risk",
			"Found SQL issue"),
	}
	r := Analyze(findings)
	if r.Models[0].CompletenessScore > 70 {
		t.Errorf("terse response should score low, got %.1f", r.Models[0].CompletenessScore)
	}
}

func TestTrailOff(t *testing.T) {
	findings := []model.Finding{
		mkFinding("trailing-model",
			"XSS vulnerability analysis",
			"The system has several XSS vulnerabilities including reflected XSS, stored XSS, and more..."),
	}
	r := Analyze(findings)
	if r.Models[0].TruncatedCount < 1 {
		t.Errorf("expected truncated count >= 1, got %d", r.Models[0].TruncatedCount)
	}
}

func TestPartialAnswer(t *testing.T) {
	findings := []model.Finding{
		mkFinding("partial-model",
			"CSRF protection missing on form submission and API endpoint and file upload handler",
			"The API endpoint lacks CSRF protection"),
	}
	r := Analyze(findings)
	if r.Models[0].PartialCount < 1 {
		t.Errorf("expected partial count >= 1, got %d", r.Models[0].PartialCount)
	}
}

func TestMultipleModels(t *testing.T) {
	findings := []model.Finding{
		mkFinding("complete-model",
			"CSRF protection missing",
			"CSRF protection is missing on the form submission endpoint. The form does not include a CSRF token, and the server does not validate the Origin header. This allows cross-site request forgery attacks."),
		mkFinding("incomplete-model",
			"CSRF protection missing",
			"Missing CSRF"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.MostComplete == r.LeastComplete {
		t.Error("most and least complete should differ")
	}
}

func TestComponentCoverageFunc(t *testing.T) {
	score := componentCoverage("SQL injection vulnerability", "SQL injection found in endpoint")
	if score < 30 {
		t.Errorf("overlapping components should score > 30, got %.1f", score)
	}
}

func TestDepthScoreShort(t *testing.T) {
	score := depthScore("short")
	if score > 40 {
		t.Errorf("single-word response should score low on depth, got %.1f", score)
	}
}

func TestDepthScoreDetailed(t *testing.T) {
	detailed := "The vulnerability exists because the input is not sanitized. Specifically, the username field passes directly to the query. Therefore an attacker can inject SQL. For example, entering admin' OR 1=1-- bypasses authentication. Additionally, the password field has the same issue."
	score := depthScore(detailed)
	if score < 60 {
		t.Errorf("detailed response should score high on depth, got %.1f", score)
	}
}

func TestTrailOffPenaltyClean(t *testing.T) {
	score := trailOffPenalty("The vulnerability is a SQL injection in the login form.")
	if score < 90 {
		t.Errorf("clean ending should have low penalty, got %.1f", score)
	}
}

func TestTrailOffPenaltyBad(t *testing.T) {
	score := trailOffPenalty("The system has issues with XSS, CSRF, and more...")
	if score > 80 {
		t.Errorf("trailing off should have penalty, got %.1f", score)
	}
}

func TestStructureScoreEmpty(t *testing.T) {
	score := structureScore("")
	if score != 0 {
		t.Errorf("empty should score 0, got %.1f", score)
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
		mkFinding("test-model", "test lab", "test summary for completeness analysis."),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Completeness") {
		t.Error("expected header in text output")
	}
	if !strings.Contains(text, "test-model") {
		t.Error("expected model name in text output")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{
		mkFinding("test-model", "test lab", "test summary for completeness analysis."),
	}
	r := Analyze(findings)
	out := FormatJSON(r)
	if !strings.Contains(out, `"completeness_score"`) {
		t.Error("expected completeness_score in JSON output")
	}
}

func TestEmptyModelName(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{}, Lab: "test", Summary: "test summary."},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown' for empty model, got %q", r.Models[0].Model)
	}
}

func TestIsTruncatedTrue(t *testing.T) {
	if !isTruncated("The system has multiple vulnerabilities including") {
		t.Error("sentence ending without punctuation should be truncated")
	}
}

func TestIsTruncatedFalse(t *testing.T) {
	if isTruncated("The system has vulnerabilities.") {
		t.Error("properly ended sentence should not be truncated")
	}
}
