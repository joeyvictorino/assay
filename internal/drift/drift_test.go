package drift

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func mkFinding(id, dedup string, sev model.Severity, state model.FindingState, summary string, evidence []model.Evidence) model.Finding {
	return model.Finding{
		ID:       id,
		DedupKey: dedup,
		Severity: sev,
		State:    state,
		Summary:  summary,
		Evidence: evidence,
	}
}

func TestIdenticalFindings(t *testing.T) {
	f := mkFinding("f1", "dk1", model.SevHigh, model.StateValidated, "SQL injection in /api/login", nil)
	r := Analyze([]model.Finding{f}, []model.Finding{f}, "runA", "runB", 0.5)
	if len(r.Drifts) != 1 {
		t.Fatalf("expected 1 drift, got %d", len(r.Drifts))
	}
	if r.Drifts[0].OverallDrift != 0 {
		t.Errorf("expected 0 drift for identical findings, got %.2f", r.Drifts[0].OverallDrift)
	}
	if r.Drifts[0].Flagged {
		t.Error("identical finding should not be flagged")
	}
	if r.Summary.StabilityScore != 1.0 {
		t.Errorf("expected stability 1.0, got %.2f", r.Summary.StabilityScore)
	}
}

func TestSeverityDrift(t *testing.T) {
	a := mkFinding("f1", "dk1", model.SevLow, model.StateValidated, "same text", nil)
	b := mkFinding("f1", "dk1", model.SevCritical, model.StateValidated, "same text", nil)
	r := Analyze([]model.Finding{a}, []model.Finding{b}, "A", "B", 0.1)
	d := r.Drifts[0]
	if d.SeverityDrift != 0.75 {
		t.Errorf("expected severity drift 0.75, got %.2f", d.SeverityDrift)
	}
	if d.SummaryDrift != 0 {
		t.Errorf("expected summary drift 0, got %.2f", d.SummaryDrift)
	}
}

func TestSummaryDrift(t *testing.T) {
	a := mkFinding("f1", "dk1", model.SevHigh, model.StateValidated, "SQL injection in login endpoint", nil)
	b := mkFinding("f1", "dk1", model.SevHigh, model.StateValidated, "XSS reflected in search page", nil)
	r := Analyze([]model.Finding{a}, []model.Finding{b}, "A", "B", 0.1)
	d := r.Drifts[0]
	if d.SummaryDrift <= 0 {
		t.Error("expected nonzero summary drift for different text")
	}
}

func TestEvidenceDrift(t *testing.T) {
	evA := []model.Evidence{{Kind: "http-exchange", StatusCode: 200, Marker: "token"}}
	evB := []model.Evidence{{Kind: "header-missing", StatusCode: 403, Marker: "csrf"}}
	a := mkFinding("f1", "dk1", model.SevHigh, model.StateValidated, "same", evA)
	b := mkFinding("f1", "dk1", model.SevHigh, model.StateValidated, "same", evB)
	r := Analyze([]model.Finding{a}, []model.Finding{b}, "A", "B", 0.1)
	d := r.Drifts[0]
	if d.EvidenceDrift <= 0 {
		t.Error("expected nonzero evidence drift for different evidence")
	}
}

func TestNewAndGoneFindings(t *testing.T) {
	a1 := mkFinding("f1", "dk1", model.SevHigh, model.StateValidated, "finding one", nil)
	a2 := mkFinding("f2", "dk2", model.SevLow, model.StateValidated, "finding two", nil)
	b1 := mkFinding("f1", "dk1", model.SevHigh, model.StateValidated, "finding one", nil)
	b3 := mkFinding("f3", "dk3", model.SevMedium, model.StateValidated, "finding three", nil)

	r := Analyze([]model.Finding{a1, a2}, []model.Finding{b1, b3}, "A", "B", 0.5)
	if len(r.NewInB) != 1 || r.NewInB[0] != "dk3" {
		t.Errorf("expected new dk3, got %v", r.NewInB)
	}
	if len(r.GoneInB) != 1 || r.GoneInB[0] != "dk2" {
		t.Errorf("expected gone dk2, got %v", r.GoneInB)
	}
	if r.Summary.TotalNew != 1 {
		t.Errorf("expected 1 new, got %d", r.Summary.TotalNew)
	}
	if r.Summary.TotalGone != 1 {
		t.Errorf("expected 1 gone, got %d", r.Summary.TotalGone)
	}
}

func TestEmptyRuns(t *testing.T) {
	r := Analyze(nil, nil, "A", "B", 0.5)
	if r.Summary.TotalMatched != 0 {
		t.Errorf("expected 0 matched, got %d", r.Summary.TotalMatched)
	}
	if r.Summary.StabilityScore != 1.0 {
		t.Errorf("expected stability 1.0 for empty, got %.2f", r.Summary.StabilityScore)
	}
}

func TestThresholdFlagging(t *testing.T) {
	a := mkFinding("f1", "dk1", model.SevInfo, model.StateValidated, "alpha beta gamma", nil)
	b := mkFinding("f1", "dk1", model.SevCritical, model.StateValidated, "completely different text", nil)
	r := Analyze([]model.Finding{a}, []model.Finding{b}, "A", "B", 0.3)
	if !r.Drifts[0].Flagged {
		t.Error("high-drift finding should be flagged at threshold 0.3")
	}
	if r.Summary.FlaggedCount != 1 {
		t.Errorf("expected 1 flagged, got %d", r.Summary.FlaggedCount)
	}
}

func TestDedupKeyFallsBackToID(t *testing.T) {
	a := mkFinding("f1", "", model.SevHigh, model.StateValidated, "same", nil)
	b := mkFinding("f1", "", model.SevHigh, model.StateValidated, "same", nil)
	r := Analyze([]model.Finding{a}, []model.Finding{b}, "A", "B", 0.5)
	if len(r.Drifts) != 1 {
		t.Fatalf("expected 1 drift, got %d", len(r.Drifts))
	}
	if r.Drifts[0].FindingID != "f1" {
		t.Errorf("expected finding ID f1, got %s", r.Drifts[0].FindingID)
	}
}

func TestSeverityDriftValues(t *testing.T) {
	tests := []struct {
		a, b string
		want float64
	}{
		{"info", "info", 0},
		{"low", "high", 0.5},
		{"info", "critical", 1.0},
		{"medium", "high", 0.25},
	}
	for _, tt := range tests {
		got := severityDrift(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("severityDrift(%s,%s) = %.2f, want %.2f", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestJaccardSimilarity(t *testing.T) {
	if jaccardSimilarity("", "") != 0 {
		t.Error("empty strings should yield 0")
	}
	if jaccardSimilarity("hello world", "hello world") != 1.0 {
		t.Error("identical strings should yield 1.0")
	}
	sim := jaccardSimilarity("hello world", "goodbye world")
	if sim <= 0 || sim >= 1 {
		t.Errorf("partial overlap should be between 0 and 1, got %.2f", sim)
	}
}

func TestFormatTextOutput(t *testing.T) {
	a := mkFinding("f1", "dk1", model.SevLow, model.StateValidated, "alpha", nil)
	b := mkFinding("f1", "dk1", model.SevCritical, model.StateValidated, "beta", nil)
	r := Analyze([]model.Finding{a}, []model.Finding{b}, "runA", "runB", 0.1)
	text := FormatText(r)
	if !strings.Contains(text, "Model Output Drift Report") {
		t.Error("expected header")
	}
	if !strings.Contains(text, "runA") || !strings.Contains(text, "runB") {
		t.Error("expected run names")
	}
	if !strings.Contains(text, "[DRIFT]") {
		t.Error("expected flagged drift marker")
	}
}

func TestMaxDriftTracking(t *testing.T) {
	a1 := mkFinding("f1", "dk1", model.SevInfo, model.StateValidated, "aaa", nil)
	b1 := mkFinding("f1", "dk1", model.SevCritical, model.StateValidated, "zzz", nil)
	a2 := mkFinding("f2", "dk2", model.SevHigh, model.StateValidated, "same text", nil)
	b2 := mkFinding("f2", "dk2", model.SevHigh, model.StateValidated, "same text", nil)

	r := Analyze([]model.Finding{a1, a2}, []model.Finding{b1, b2}, "A", "B", 0.1)
	if r.Summary.MaxDrift <= 0 {
		t.Error("expected positive max drift")
	}
	if r.Summary.MaxDrift < r.Summary.MeanDrift {
		t.Error("max drift should be >= mean drift")
	}
}

func TestBothEvidenceEmpty(t *testing.T) {
	d := evidenceDrift(
		model.Finding{Evidence: nil},
		model.Finding{Evidence: nil},
	)
	if d != 0 {
		t.Errorf("expected 0 evidence drift for both empty, got %.2f", d)
	}
}
