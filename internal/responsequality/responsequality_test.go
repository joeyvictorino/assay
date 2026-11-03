package responsequality

import (
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
)

func baseFinding(id string) model.Finding {
	return model.Finding{
		ID:       id,
		Summary:  "SQL injection in user login form allows bypass of authentication",
		Severity: model.SevHigh,
		Class:    "injection",
		CWE:      "CWE-89",
		Location: model.Location{Method: "POST", PathTemplate: "/api/login"},
		Evidence: []model.Evidence{{Kind: "http-exchange"}},
		Remediation: &model.Remediation{
			ConfigChange: "Use parameterized queries",
		},
		FirstSeen: time.Now(),
		LastSeen:  time.Now(),
	}
}

func minimalFinding(id string) model.Finding {
	return model.Finding{
		ID:        id,
		Summary:   "bad",
		Severity:  model.SevInfo,
		FirstSeen: time.Now(),
		LastSeen:  time.Now(),
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil, 0.5)
	if r.Summary.TotalFindings != 0 {
		t.Fatalf("expected 0 findings, got %d", r.Summary.TotalFindings)
	}
}

func TestHighQualityFinding(t *testing.T) {
	f := baseFinding("f1")
	r := Analyze([]model.Finding{f}, 0.5)
	if len(r.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(r.Findings))
	}
	fq := r.Findings[0]
	if fq.Overall < 0.7 {
		t.Errorf("expected high quality score, got %.2f", fq.Overall)
	}
	if fq.Grade != "A" && fq.Grade != "B" {
		t.Errorf("expected grade A or B, got %s", fq.Grade)
	}
	if fq.Flagged {
		t.Error("high quality finding should not be flagged")
	}
}

func TestLowQualityFinding(t *testing.T) {
	f := minimalFinding("f2")
	r := Analyze([]model.Finding{f}, 0.5)
	fq := r.Findings[0]
	if fq.Overall > 0.5 {
		t.Errorf("expected low quality score, got %.2f", fq.Overall)
	}
	if !fq.Flagged {
		t.Error("low quality finding should be flagged")
	}
}

func TestGradeDistribution(t *testing.T) {
	findings := []model.Finding{
		baseFinding("f1"),
		minimalFinding("f2"),
		baseFinding("f3"),
	}
	r := Analyze(findings, 0.5)
	if r.Summary.TotalFindings != 3 {
		t.Fatalf("expected 3 findings, got %d", r.Summary.TotalFindings)
	}
	total := 0
	for _, c := range r.Summary.GradeDistrib {
		total += c
	}
	if total != 3 {
		t.Errorf("grade distribution should sum to 3, got %d", total)
	}
}

func TestDimensionAverages(t *testing.T) {
	findings := []model.Finding{baseFinding("f1")}
	r := Analyze(findings, 0.5)
	for _, key := range []string{"completeness", "specificity", "actionability", "clarity"} {
		if _, ok := r.Summary.ByDimension[key]; !ok {
			t.Errorf("missing dimension average for %s", key)
		}
	}
}

func TestLowestDimension(t *testing.T) {
	f := minimalFinding("f1")
	r := Analyze([]model.Finding{f}, 0.5)
	if r.Summary.LowestDim == "" {
		t.Error("expected a lowest dimension to be identified")
	}
}

func TestCompletenessAllFields(t *testing.T) {
	f := baseFinding("f1")
	ds := scoreCompleteness(f)
	if ds.Score < 0.8 {
		t.Errorf("full finding should have high completeness, got %.2f", ds.Score)
	}
}

func TestCompletenessMissingFields(t *testing.T) {
	f := model.Finding{ID: "f1", Summary: "test"}
	ds := scoreCompleteness(f)
	if ds.Score > 0.5 {
		t.Errorf("sparse finding should have low completeness, got %.2f", ds.Score)
	}
}

func TestSpecificityEmpty(t *testing.T) {
	f := model.Finding{ID: "f1"}
	ds := scoreSpecificity(f)
	if ds.Score != 0 {
		t.Errorf("empty summary should have 0 specificity, got %.2f", ds.Score)
	}
}

func TestSpecificityRich(t *testing.T) {
	f := baseFinding("f1")
	ds := scoreSpecificity(f)
	if ds.Score < 0.5 {
		t.Errorf("rich finding should have high specificity, got %.2f", ds.Score)
	}
}

func TestActionabilityWithRemediation(t *testing.T) {
	f := baseFinding("f1")
	ds := scoreActionability(f)
	if ds.Score < 0.6 {
		t.Errorf("finding with remediation should score well, got %.2f", ds.Score)
	}
}

func TestActionabilityWithoutRemediation(t *testing.T) {
	f := model.Finding{
		ID:       "f1",
		Severity: model.SevHigh,
		Location: model.Location{Method: "GET"},
	}
	ds := scoreActionability(f)
	if ds.Score > 0.5 {
		t.Errorf("no remediation should score low, got %.2f", ds.Score)
	}
}

func TestClarityGoodSummary(t *testing.T) {
	f := baseFinding("f1")
	ds := scoreClarity(f)
	if ds.Score < 0.5 {
		t.Errorf("clear summary should score well, got %.2f", ds.Score)
	}
}

func TestClarityEmptySummary(t *testing.T) {
	f := model.Finding{ID: "f1"}
	ds := scoreClarity(f)
	if ds.Score != 0 {
		t.Errorf("empty summary should have 0 clarity, got %.2f", ds.Score)
	}
}

func TestLetterGrades(t *testing.T) {
	cases := []struct {
		score float64
		grade string
	}{
		{0.95, "A"},
		{0.85, "B"},
		{0.75, "C"},
		{0.65, "D"},
		{0.45, "F"},
	}
	for _, tc := range cases {
		got := letterGrade(tc.score)
		if got != tc.grade {
			t.Errorf("letterGrade(%.2f) = %s, want %s", tc.score, got, tc.grade)
		}
	}
}

func TestThreshold(t *testing.T) {
	f := baseFinding("f1")
	r := Analyze([]model.Finding{f}, 0.99)
	if !r.Findings[0].Flagged {
		t.Error("finding should be flagged at very high threshold")
	}
}

func TestFormatTextNotEmpty(t *testing.T) {
	findings := []model.Finding{baseFinding("f1")}
	r := Analyze(findings, 0.5)
	text := FormatText(r)
	if text == "" {
		t.Error("format text should not be empty")
	}
	if !containsStr(text, "Response Quality") {
		t.Error("should contain header")
	}
	if !containsStr(text, "completeness") {
		t.Error("should contain dimension names")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil, 0.5)
	text := FormatText(r)
	if !containsStr(text, "Response Quality") {
		t.Error("empty report should still have header")
	}
}

func TestMultipleFindings(t *testing.T) {
	findings := make([]model.Finding, 10)
	for i := range findings {
		if i%2 == 0 {
			findings[i] = baseFinding("f" + string(rune('0'+i)))
		} else {
			findings[i] = minimalFinding("f" + string(rune('0'+i)))
		}
	}
	r := Analyze(findings, 0.5)
	if r.Summary.TotalFindings != 10 {
		t.Errorf("expected 10, got %d", r.Summary.TotalFindings)
	}
	if r.Summary.MeanQuality <= 0 {
		t.Error("mean quality should be positive")
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && findSubstr(s, sub))
}

func findSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
