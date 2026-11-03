package consistency

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func writeFindingsFile(t *testing.T, dir, name string, findings []model.Finding) string {
	t.Helper()
	data, err := json.Marshal(findings)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func makeFinding(class model.Class, sev model.Severity, method string, path string, summary string) model.Finding {
	return model.Finding{
		Class:    class,
		Severity: sev,
		Location: model.Location{Method: method, PathTemplate: path},
		Summary:  summary,
	}
}

func TestAnalyzeConsistencyAllStable(t *testing.T) {
	dir := t.TempDir()
	f1 := makeFinding(model.ClassSQLi, model.SevHigh, "POST", "/api/query", "SQL injection in query")
	f2 := makeFinding(model.ClassXSSReflected, model.SevMedium, "GET", "/api/search", "Reflected XSS")

	p1 := writeFindingsFile(t, dir, "run1.json", []model.Finding{f1, f2})
	p2 := writeFindingsFile(t, dir, "run2.json", []model.Finding{f1, f2})
	p3 := writeFindingsFile(t, dir, "run3.json", []model.Finding{f1, f2})

	r, err := AnalyzeConsistency([]string{p1, p2, p3}, 0.8)
	if err != nil {
		t.Fatal(err)
	}
	if r.TotalRuns != 3 {
		t.Errorf("TotalRuns = %d, want 3", r.TotalRuns)
	}
	if r.UniqueFindings != 2 {
		t.Errorf("UniqueFindings = %d, want 2", r.UniqueFindings)
	}
	if r.StableFindings != 2 {
		t.Errorf("StableFindings = %d, want 2", r.StableFindings)
	}
	if r.FlakyFindings != 0 {
		t.Errorf("FlakyFindings = %d, want 0", r.FlakyFindings)
	}
	if r.OverallScore != 1.0 {
		t.Errorf("OverallScore = %f, want 1.0", r.OverallScore)
	}
}

func TestAnalyzeConsistencyFlaky(t *testing.T) {
	dir := t.TempDir()
	f1 := makeFinding(model.ClassSQLi, model.SevHigh, "POST", "/api/query", "SQL injection")
	f2 := makeFinding(model.ClassXSSReflected, model.SevMedium, "GET", "/api/search", "XSS")

	p1 := writeFindingsFile(t, dir, "run1.json", []model.Finding{f1, f2})
	p2 := writeFindingsFile(t, dir, "run2.json", []model.Finding{f1})
	p3 := writeFindingsFile(t, dir, "run3.json", []model.Finding{f1})

	r, err := AnalyzeConsistency([]string{p1, p2, p3}, 0.8)
	if err != nil {
		t.Fatal(err)
	}
	if r.StableFindings != 1 {
		t.Errorf("StableFindings = %d, want 1", r.StableFindings)
	}
	if r.FlakyFindings != 1 {
		t.Errorf("FlakyFindings = %d, want 1", r.FlakyFindings)
	}
	for _, f := range r.Findings {
		if f.Class == "xss-reflected" && f.Status != "flaky" {
			t.Errorf("XSS finding status = %q, want flaky", f.Status)
		}
	}
}

func TestAnalyzeConsistencyTooFewRuns(t *testing.T) {
	_, err := AnalyzeConsistency([]string{"single.json"}, 0.8)
	if err == nil {
		t.Fatal("expected error for single run")
	}
}

func TestAnalyzeConsistencyMostlyStable(t *testing.T) {
	dir := t.TempDir()
	f1 := makeFinding(model.ClassIDOR, model.SevHigh, "GET", "/api/users/{id}", "IDOR")

	p1 := writeFindingsFile(t, dir, "run1.json", []model.Finding{f1})
	p2 := writeFindingsFile(t, dir, "run2.json", []model.Finding{f1})
	p3 := writeFindingsFile(t, dir, "run3.json", []model.Finding{f1})
	p4 := writeFindingsFile(t, dir, "run4.json", []model.Finding{f1})
	p5 := writeFindingsFile(t, dir, "run5.json", []model.Finding{})

	r, err := AnalyzeConsistency([]string{p1, p2, p3, p4, p5}, 0.8)
	if err != nil {
		t.Fatal(err)
	}
	if r.UniqueFindings != 1 {
		t.Errorf("UniqueFindings = %d, want 1", r.UniqueFindings)
	}
	if r.Findings[0].Status != "mostly-stable" {
		t.Errorf("status = %q, want mostly-stable (rate=0.8)", r.Findings[0].Status)
	}
	if r.StableFindings != 1 {
		t.Errorf("StableFindings = %d, want 1", r.StableFindings)
	}
}

func TestAnalyzeConsistencyDedupWithinRun(t *testing.T) {
	dir := t.TempDir()
	f1 := makeFinding(model.ClassSQLi, model.SevHigh, "POST", "/api/query", "SQL injection")

	p1 := writeFindingsFile(t, dir, "run1.json", []model.Finding{f1, f1, f1})
	p2 := writeFindingsFile(t, dir, "run2.json", []model.Finding{f1})

	r, err := AnalyzeConsistency([]string{p1, p2}, 0.8)
	if err != nil {
		t.Fatal(err)
	}
	if r.UniqueFindings != 1 {
		t.Errorf("UniqueFindings = %d, want 1", r.UniqueFindings)
	}
	if r.Findings[0].RunCount != 2 {
		t.Errorf("RunCount = %d, want 2", r.Findings[0].RunCount)
	}
}

func TestAnalyzeConsistencyEmpty(t *testing.T) {
	dir := t.TempDir()
	p1 := writeFindingsFile(t, dir, "run1.json", []model.Finding{})
	p2 := writeFindingsFile(t, dir, "run2.json", []model.Finding{})

	r, err := AnalyzeConsistency([]string{p1, p2}, 0.8)
	if err != nil {
		t.Fatal(err)
	}
	if r.UniqueFindings != 0 {
		t.Errorf("UniqueFindings = %d, want 0", r.UniqueFindings)
	}
	if r.OverallScore != 0.0 {
		t.Errorf("OverallScore = %f, want 0", r.OverallScore)
	}
}

func TestFormatText(t *testing.T) {
	r := ConsistencyReport{
		TotalRuns:      3,
		UniqueFindings: 2,
		StableFindings: 1,
		FlakyFindings:  1,
		OverallScore:   0.833,
		Threshold:      0.8,
		Findings: []FindingPresence{
			{Key: "sqli|high|app.go:10", Class: "sqli", Severity: "high", Summary: "SQL injection", RunCount: 3, TotalRuns: 3, Rate: 1.0, Status: "stable"},
			{Key: "xss-reflected|medium|h.go:20", Class: "xss-reflected", Severity: "medium", Summary: "XSS", RunCount: 2, TotalRuns: 3, Rate: 0.667, Status: "flaky"},
		},
	}
	text := FormatText(r)
	if text == "" {
		t.Fatal("empty output")
	}
	if len(text) < 50 {
		t.Errorf("output too short: %d chars", len(text))
	}
}
