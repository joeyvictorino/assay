package modeldrift

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestComputeMetricsEmpty(t *testing.T) {
	m := computeMetrics(nil)
	if m.TotalFindings != 0 {
		t.Fatalf("expected 0 findings, got %d", m.TotalFindings)
	}
}

func TestComputeMetrics(t *testing.T) {
	ff := []model.Finding{
		{Severity: model.SevHigh, Class: "xss", State: model.StateValidated, CWE: "CWE-79", Evidence: []model.Evidence{{}, {}}},
		{Severity: model.SevMedium, Class: "sqli", State: model.StateTheorized, CWE: "CWE-89"},
		{Severity: model.SevHigh, Class: "xss", State: model.StateDeclined},
	}
	m := computeMetrics(ff)
	if m.TotalFindings != 3 {
		t.Fatalf("expected 3 findings, got %d", m.TotalFindings)
	}
	if m.SeverityDistribution["high"] != 2 {
		t.Fatalf("expected 2 high, got %d", m.SeverityDistribution["high"])
	}
	if m.UniqueCWEs != 2 {
		t.Fatalf("expected 2 CWEs, got %d", m.UniqueCWEs)
	}
	if m.ValidationRate < 0.33 || m.ValidationRate > 0.34 {
		t.Fatalf("unexpected validation rate: %.2f", m.ValidationRate)
	}
	if m.RefusalRate < 0.33 || m.RefusalRate > 0.34 {
		t.Fatalf("unexpected refusal rate: %.2f", m.RefusalRate)
	}
}

func TestDistributionDrift(t *testing.T) {
	a := map[string]int{"high": 5, "low": 5}
	b := map[string]int{"high": 5, "low": 5}
	d := distributionDrift(a, b, 10, 10)
	if d != 0 {
		t.Fatalf("identical distributions should have 0 drift, got %.2f", d)
	}

	c := map[string]int{"high": 10}
	d2 := distributionDrift(a, c, 10, 10)
	if d2 <= 0 {
		t.Fatalf("different distributions should have positive drift, got %.2f", d2)
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil, 0.3)
	if r.Summary.TotalComparisons != 0 {
		t.Fatalf("expected 0 comparisons, got %d", r.Summary.TotalComparisons)
	}
}

func TestAnalyzeSingle(t *testing.T) {
	r := Analyze([]RunSnapshot{{RunID: "r1", Model: "m1"}}, 0.3)
	if r.Summary.TotalComparisons != 0 {
		t.Fatal("single snapshot should produce 0 comparisons")
	}
}

func TestAnalyzeIdenticalRuns(t *testing.T) {
	ff := []model.Finding{
		{ID: "f1", DedupKey: "dk1", Severity: model.SevHigh, Class: "xss", State: model.StateValidated},
	}
	snaps := []RunSnapshot{
		{RunID: "r1", Model: "m1", Findings: ff},
		{RunID: "r2", Model: "m1", Findings: ff},
	}
	r := Analyze(snaps, 0.3)
	if r.Summary.TotalComparisons != 1 {
		t.Fatalf("expected 1 comparison, got %d", r.Summary.TotalComparisons)
	}
	if r.Comparisons[0].OverallDrift != 0 {
		t.Fatalf("identical runs should have 0 drift, got %.2f", r.Comparisons[0].OverallDrift)
	}
	if r.Comparisons[0].Flagged {
		t.Fatal("identical runs should not be flagged")
	}
}

func TestAnalyzeDriftDetected(t *testing.T) {
	snapA := RunSnapshot{
		RunID: "r1", Model: "m1",
		Findings: []model.Finding{
			{ID: "f1", Severity: model.SevLow, Class: "info-leak", State: model.StateValidated},
			{ID: "f2", Severity: model.SevLow, Class: "info-leak", State: model.StateValidated},
		},
	}
	snapB := RunSnapshot{
		RunID: "r2", Model: "m1",
		Findings: []model.Finding{
			{ID: "f3", Severity: model.SevCritical, Class: "rce", State: model.StateTheorized},
			{ID: "f4", Severity: model.SevCritical, Class: "rce", State: model.StateTheorized},
			{ID: "f5", Severity: model.SevCritical, Class: "rce", State: model.StateTheorized},
			{ID: "f6", Severity: model.SevHigh, Class: "sqli", State: model.StateDeclined},
		},
	}
	r := Analyze([]RunSnapshot{snapA, snapB}, 0.1)
	if r.Summary.FlaggedCount != 1 {
		t.Fatalf("expected 1 flagged, got %d", r.Summary.FlaggedCount)
	}
	if r.Comparisons[0].OverallDrift <= 0.1 {
		t.Fatalf("expected significant drift, got %.2f", r.Comparisons[0].OverallDrift)
	}
}

func TestAnalyzeMultipleSnapshots(t *testing.T) {
	s1 := RunSnapshot{RunID: "r1", Model: "m1", Findings: []model.Finding{{ID: "f1", Severity: model.SevLow}}}
	s2 := RunSnapshot{RunID: "r2", Model: "m1", Findings: []model.Finding{{ID: "f2", Severity: model.SevHigh}}}
	s3 := RunSnapshot{RunID: "r3", Model: "m2", Findings: []model.Finding{{ID: "f3", Severity: model.SevCritical}}}
	r := Analyze([]RunSnapshot{s1, s2, s3}, 0.3)
	if r.Summary.TotalComparisons != 2 {
		t.Fatalf("expected 2 comparisons, got %d", r.Summary.TotalComparisons)
	}
}

func TestFormatText(t *testing.T) {
	r := Analyze([]RunSnapshot{
		{RunID: "r1", Model: "m1", Findings: []model.Finding{{ID: "f1", Severity: model.SevLow}}},
		{RunID: "r2", Model: "m1", Findings: []model.Finding{{ID: "f2", Severity: model.SevHigh}}},
	}, 0.1)
	text := FormatText(r)
	if !strings.Contains(text, "Cross-Run Model Behavior Drift") {
		t.Fatal("missing header")
	}
	if !strings.Contains(text, "r1") || !strings.Contains(text, "r2") {
		t.Fatal("missing run IDs")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]RunSnapshot{
		{RunID: "r1", Model: "m1", Findings: []model.Finding{{ID: "f1", Severity: model.SevLow}}},
		{RunID: "r2", Model: "m2", Findings: []model.Finding{{ID: "f2", Severity: model.SevHigh}}},
	}, 0.3)
	out := FormatJSON(r)
	var data map[string]any
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := data["comparisons"]; !ok {
		t.Fatal("missing comparisons key")
	}
	if _, ok := data["summary"]; !ok {
		t.Fatal("missing summary key")
	}
}

func TestDefaultThreshold(t *testing.T) {
	r := Analyze([]RunSnapshot{
		{RunID: "r1", Model: "m1"},
		{RunID: "r2", Model: "m1"},
	}, 0)
	if r.Summary.TotalComparisons != 1 {
		t.Fatalf("expected 1 comparison, got %d", r.Summary.TotalComparisons)
	}
}
