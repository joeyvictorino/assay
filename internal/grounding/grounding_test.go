package grounding

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestEmptyFindings(t *testing.T) {
	r := Analyze(nil, 0.5)
	if r.Summary.TotalFindings != 0 {
		t.Errorf("expected 0, got %d", r.Summary.TotalFindings)
	}
	if r.Summary.MinScore != 1.0 {
		t.Errorf("expected min 1.0, got %.2f", r.Summary.MinScore)
	}
}

func TestWellGroundedFinding(t *testing.T) {
	findings := []model.Finding{
		{
			ID:       "f1",
			Summary:  "SQL injection in /api/v2/users allows authentication bypass via id parameter",
			Severity: "high",
			Class:    "sqli",
			CWE:      "CWE-89",
			Location: model.Location{Method: "POST", PathTemplate: "/api/v2/users", Param: "id", Component: "backend"},
			Evidence: []model.Evidence{
				{Marker: "error-based", StatusCode: 500, RequestDigest: "abc123"},
			},
		},
	}
	r := Analyze(findings, 0.5)
	if r.Scores[0].OverallScore < 0.5 {
		t.Errorf("well-grounded finding should score high, got %.2f", r.Scores[0].OverallScore)
	}
	if r.Scores[0].Flagged {
		t.Error("well-grounded finding should not be flagged")
	}
}

func TestPoorlyGroundedFinding(t *testing.T) {
	findings := []model.Finding{
		{
			ID:       "f2",
			Summary:  "something bad might happen",
			Severity: "medium",
		},
	}
	r := Analyze(findings, 0.5)
	if r.Scores[0].OverallScore >= 0.5 {
		t.Errorf("poorly grounded finding should score low, got %.2f", r.Scores[0].OverallScore)
	}
	if !r.Scores[0].Flagged {
		t.Error("poorly grounded finding should be flagged")
	}
}

func TestEvidenceGrading(t *testing.T) {
	noEvidence := model.Finding{ID: "f1"}
	if scoreEvidenceGrounding(noEvidence) != 0.0 {
		t.Error("no evidence should score 0")
	}

	someEvidence := model.Finding{ID: "f2", Evidence: []model.Evidence{{Marker: "test"}}}
	if scoreEvidenceGrounding(someEvidence) < 0.5 {
		t.Error("some evidence should score at least 0.5")
	}

	richEvidence := model.Finding{ID: "f3", Evidence: []model.Evidence{
		{Marker: "a", StatusCode: 200, RequestDigest: "abc"},
	}}
	if scoreEvidenceGrounding(richEvidence) != 1.0 {
		t.Errorf("rich evidence should score 1.0, got %.2f", scoreEvidenceGrounding(richEvidence))
	}
}

func TestLocationGrading(t *testing.T) {
	none := model.Finding{ID: "f1"}
	if scoreLocationGrounding(none) != 0.0 {
		t.Error("no location should score 0")
	}

	full := model.Finding{ID: "f2", Location: model.Location{
		Method: "GET", PathTemplate: "/api", Param: "id", Component: "web",
	}}
	if scoreLocationGrounding(full) != 1.0 {
		t.Errorf("full location should score 1.0, got %.2f", scoreLocationGrounding(full))
	}
}

func TestCWEGrounding(t *testing.T) {
	tests := []struct {
		cwe  string
		want float64
	}{
		{"", 0.0},
		{"CWE-89", 1.0},
		{"invalid", 0.2},
		{"CWE-", 0.3},
	}
	for _, tt := range tests {
		f := model.Finding{CWE: tt.cwe}
		got := scoreCWEGrounding(f)
		if got != tt.want {
			t.Errorf("scoreCWE(%q) = %.2f, want %.2f", tt.cwe, got, tt.want)
		}
	}
}

func TestSummarySpecificity(t *testing.T) {
	vague := model.Finding{ID: "f1", Summary: "something is wrong here"}
	if scoreSummarySpecificity(vague) > 0.3 {
		t.Errorf("vague summary should score low, got %.2f", scoreSummarySpecificity(vague))
	}

	specific := model.Finding{ID: "f2", Summary: "SQL injection in /api/v2/users CWE-89 allows param=1 OR 1=1"}
	if scoreSummarySpecificity(specific) < 0.6 {
		t.Errorf("specific summary should score high, got %.2f", scoreSummarySpecificity(specific))
	}
}

func TestClassGrounding(t *testing.T) {
	noClass := model.Finding{ID: "f1", Summary: "injection found"}
	if scoreClassGrounding(noClass) != 0.0 {
		t.Error("no class should score 0")
	}

	matched := model.Finding{ID: "f2", Class: "sql-injection", Summary: "SQL injection found via automated sql scan"}
	if scoreClassGrounding(matched) < 0.5 {
		t.Errorf("matched class should score well, got %.2f", scoreClassGrounding(matched))
	}
}

func TestFormatTextOutput(t *testing.T) {
	findings := []model.Finding{
		{ID: "f1", Summary: "test", Severity: "low"},
	}
	r := Analyze(findings, 0.5)
	text := FormatText(r)
	if !strings.Contains(text, "Factual Grounding") {
		t.Error("expected header in text output")
	}
}

func TestMultipleFindingsAggregation(t *testing.T) {
	findings := []model.Finding{
		{
			ID: "f1", Summary: "SQL injection in /api/v2/login via username",
			Severity: "high", Class: "sqli", CWE: "CWE-89",
			Location: model.Location{Method: "POST", PathTemplate: "/api/v2/login", Param: "username"},
			Evidence: []model.Evidence{{Marker: "error", StatusCode: 500}},
		},
		{
			ID: "f2", Summary: "something vague",
			Severity: "medium",
		},
	}
	r := Analyze(findings, 0.5)
	if r.Summary.FlaggedCount != 1 {
		t.Errorf("expected 1 flagged, got %d", r.Summary.FlaggedCount)
	}
}
