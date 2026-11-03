package coherence

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
}

func TestSingleFinding(t *testing.T) {
	findings := []model.Finding{
		{ID: "f1", Summary: "SQL injection in login endpoint", Severity: "high", Class: "sqli"},
	}
	r := Analyze(findings, 0.5)
	if r.Scores[0].OverallScore != 0.0 {
		t.Errorf("first finding should have 0 drift, got %.2f", r.Scores[0].OverallScore)
	}
}

func TestCoherentSequence(t *testing.T) {
	findings := []model.Finding{
		{ID: "f1", Summary: "SQL injection in login endpoint allows bypass", Severity: "high", Class: "sqli"},
		{ID: "f2", Summary: "SQL injection in login endpoint enables data extraction", Severity: "high", Class: "sqli"},
	}
	r := Analyze(findings, 0.5)
	if r.Scores[1].TopicDrift > 0.5 {
		t.Errorf("similar summaries should have low drift, got %.2f", r.Scores[1].TopicDrift)
	}
	if r.Scores[1].SeverityStability > 0.0 {
		t.Errorf("same severity should be stable, got %.2f", r.Scores[1].SeverityStability)
	}
}

func TestTopicDriftDetection(t *testing.T) {
	findings := []model.Finding{
		{ID: "f1", Summary: "SQL injection in login endpoint", Severity: "high", Class: "sqli"},
		{ID: "f2", Summary: "Outdated SSL certificate on monitoring dashboard", Severity: "low", Class: "ssl-expired"},
	}
	r := Analyze(findings, 0.3)
	if r.Scores[1].TopicDrift < 0.5 {
		t.Errorf("unrelated summaries should show drift, got %.2f", r.Scores[1].TopicDrift)
	}
	if !r.Scores[1].Flagged {
		t.Error("expected flagged for high drift")
	}
}

func TestSeverityJump(t *testing.T) {
	findings := []model.Finding{
		{ID: "f1", Summary: "informational note about headers", Severity: "info", Class: "header-check"},
		{ID: "f2", Summary: "critical RCE vulnerability found", Severity: "critical", Class: "rce"},
	}
	r := Analyze(findings, 0.5)
	if r.Scores[1].SeverityStability < 0.5 {
		t.Errorf("info->critical jump should be unstable, got %.2f", r.Scores[1].SeverityStability)
	}
}

func TestClassChange(t *testing.T) {
	findings := []model.Finding{
		{ID: "f1", Summary: "XSS reflected in search", Severity: "medium", Class: "xss-reflected"},
		{ID: "f2", Summary: "XSS stored in comments", Severity: "medium", Class: "xss-stored"},
	}
	r := Analyze(findings, 0.5)
	if r.Scores[1].ClassStability != 1.0 {
		t.Errorf("different classes should score 1.0, got %.2f", r.Scores[1].ClassStability)
	}
}

func TestDetailProgression(t *testing.T) {
	findings := []model.Finding{
		{ID: "f1", Summary: "short note", Severity: "low", Class: "info"},
		{ID: "f2", Summary: "this is an extremely detailed and verbose summary that goes on and on about many different things covering multiple aspects of the vulnerability including its impact scope and remediation steps", Severity: "low", Class: "info"},
	}
	r := Analyze(findings, 0.5)
	if r.Scores[1].DetailProgression < 0.5 {
		t.Errorf("large length change should flag detail progression, got %.2f", r.Scores[1].DetailProgression)
	}
}

func TestSummaryAggregation(t *testing.T) {
	findings := []model.Finding{
		{ID: "f1", Summary: "first finding about auth", Severity: "high", Class: "auth"},
		{ID: "f2", Summary: "second finding about auth bypass", Severity: "high", Class: "auth"},
		{ID: "f3", Summary: "completely different topic on networking", Severity: "low", Class: "network"},
	}
	r := Analyze(findings, 0.5)
	if r.Summary.TotalFindings != 3 {
		t.Errorf("expected 3, got %d", r.Summary.TotalFindings)
	}
	if r.Summary.MeanScore <= 0 {
		t.Error("expected non-zero mean")
	}
}

func TestFormatTextOutput(t *testing.T) {
	findings := []model.Finding{
		{ID: "f1", Summary: "test finding", Severity: "medium", Class: "test"},
	}
	r := Analyze(findings, 0.5)
	text := FormatText(r)
	if !strings.Contains(text, "Multi-Turn Coherence") {
		t.Error("expected header in text output")
	}
}

func TestThresholdBoundary(t *testing.T) {
	findings := []model.Finding{
		{ID: "f1", Summary: "auth bypass in login", Severity: "high", Class: "auth"},
		{ID: "f2", Summary: "completely different unrelated server config", Severity: "info", Class: "config"},
	}
	r1 := Analyze(findings, 0.01)
	r2 := Analyze(findings, 0.99)
	if r1.Summary.FlaggedCount <= r2.Summary.FlaggedCount {
		t.Error("lower threshold should flag more")
	}
}

func TestJaccardSimilarity(t *testing.T) {
	tests := []struct {
		a, b string
		min  float64
		max  float64
	}{
		{"hello world", "hello world", 1.0, 1.0},
		{"hello world", "goodbye moon", 0.0, 0.1},
		{"", "", 0.0, 0.0},
	}
	for _, tt := range tests {
		got := jaccardSimilarity(tt.a, tt.b)
		if got < tt.min || got > tt.max {
			t.Errorf("jaccard(%q, %q) = %.2f, want [%.2f, %.2f]", tt.a, tt.b, got, tt.min, tt.max)
		}
	}
}
