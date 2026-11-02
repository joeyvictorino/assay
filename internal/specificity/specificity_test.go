package specificity

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

func TestHighSpecificity(t *testing.T) {
	findings := []model.Finding{
		mkFinding("precise-model",
			"SQL injection in login endpoint",
			"SQL injection vulnerability found in file src/auth/login.py line 42, function handle_login. The parameter 'username' is concatenated into the SQL query without parameterization. CVE-2024-1234 applies. Payload: admin' OR 1=1-- bypasses authentication via the /api/login endpoint on port 8080."),
	}
	r := Analyze(findings)
	if r.Models[0].SpecificityScore < 50 {
		t.Errorf("specific response should score well, got %.1f", r.Models[0].SpecificityScore)
	}
}

func TestLowSpecificity(t *testing.T) {
	findings := []model.Finding{
		mkFinding("vague-model",
			"Security issues in the application",
			"There are some issues with various things that might be problems. Something seems somewhat wrong with certain stuff."),
	}
	r := Analyze(findings)
	if r.Models[0].SpecificityScore > 60 {
		t.Errorf("vague response should score low, got %.1f", r.Models[0].SpecificityScore)
	}
}

func TestVaguenessScoreHigh(t *testing.T) {
	score := vaguenessScore("There might be some issues with various things that could possibly be problems somehow.")
	if score > 60 {
		t.Errorf("vague text should have low vagueness score (high penalty), got %.1f", score)
	}
}

func TestVaguenessScoreLow(t *testing.T) {
	score := vaguenessScore("SQL injection in login.py line 42 allows authentication bypass via unsanitized input.")
	if score < 80 {
		t.Errorf("specific text should have high vagueness score (low penalty), got %.1f", score)
	}
}

func TestTechnicalDepthDetailed(t *testing.T) {
	score := technicalDepth("CVE-2024-1234 describes an SQL injection at endpoint /api/login via parameter username. CVSS 9.8. XSS also present in the response header.")
	if score < 60 {
		t.Errorf("technical text should score high on depth, got %.1f", score)
	}
}

func TestTechnicalDepthEmpty(t *testing.T) {
	score := technicalDepth("")
	if score != 0 {
		t.Errorf("empty should score 0, got %.1f", score)
	}
}

func TestLocationPrecisionWithFile(t *testing.T) {
	score := locationPrecision("Vulnerability found in file src/auth.py at line 42 in function handle_login")
	if score < 60 {
		t.Errorf("location-rich text should score high, got %.1f", score)
	}
}

func TestLocationPrecisionGeneric(t *testing.T) {
	score := locationPrecision("found a vulnerability")
	if score > 40 {
		t.Errorf("generic text should have low location precision, got %.1f", score)
	}
}

func TestEvidenceQualityGood(t *testing.T) {
	score := evidenceQuality("SQL injection in login",
		"SQL injection because the input parameter is concatenated. For example, payload admin' OR 1=1-- demonstrates the vulnerability. The error message reveals database structure.")
	if score < 60 {
		t.Errorf("evidence-rich text should score high, got %.1f", score)
	}
}

func TestEvidenceQualityPoor(t *testing.T) {
	score := evidenceQuality("SQL injection in login", "bad code")
	if score > 60 {
		t.Errorf("low-evidence text should score low, got %.1f", score)
	}
}

func TestMultipleModels(t *testing.T) {
	findings := []model.Finding{
		mkFinding("specific-model",
			"XSS in search",
			"Reflected XSS in file app/search.js line 15 via the query parameter. Payload: <script>alert(1)</script> at endpoint /search. CWE-79 applies."),
		mkFinding("vague-model",
			"XSS in search",
			"There might be some kind of XSS issue somewhere."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.MostSpecific == r.LeastSpecific {
		t.Error("most and least specific should differ")
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

func TestIsVague(t *testing.T) {
	if !isVague("something might possibly be somewhat wrong with various things") {
		t.Error("heavily vague text should be flagged as vague")
	}
}

func TestIsSpecific(t *testing.T) {
	if !isSpecific("CVE-2024-1234 SQL injection at endpoint /api/login via parameter username. CVSS 9.8. XSS also found.") {
		t.Error("technical text should be flagged as specific")
	}
}

func TestFormatText(t *testing.T) {
	findings := []model.Finding{
		mkFinding("test-model", "test lab", "test summary for specificity analysis."),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Specificity") {
		t.Error("expected header in text output")
	}
	if !strings.Contains(text, "test-model") {
		t.Error("expected model name in text output")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{
		mkFinding("test-model", "test lab", "test summary for specificity analysis."),
	}
	r := Analyze(findings)
	out := FormatJSON(r)
	if !strings.Contains(out, `"specificity_score"`) {
		t.Error("expected specificity_score in JSON output")
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

func TestVagueCountTracking(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m1", "vuln", "something might possibly be somewhat wrong with various things and stuff"),
		mkFinding("m1", "vuln", "specific SQL injection in file auth.py at line 42 via the username parameter"),
	}
	r := Analyze(findings)
	if r.Models[0].VagueCount < 1 {
		t.Errorf("expected at least 1 vague finding, got %d", r.Models[0].VagueCount)
	}
}
