package report

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
)

func testFinding(class model.Class, sev model.Severity, state model.FindingState) model.Finding {
	return model.Finding{
		ID:       "f-001",
		DedupKey: "sqli-GET-/api/v1/users-id",
		RunID:    "run-test",
		Lab:      "juice-shop",
		Class:    class,
		CWE:      "CWE-89",
		Location: model.Location{
			Method:       "GET",
			PathTemplate: "/api/v1/users/{id}",
			Param:        "id",
		},
		Severity:  sev,
		State:     state,
		Summary:   "SQL injection in user lookup endpoint",
		Model:     model.ModelRef{Provider: "anthropic", Model: "claude-sonnet-4-20250514"},
		FirstSeen: time.Now(),
		LastSeen:  time.Now(),
	}
}

func TestFindingsToSARIF_ValidJSON(t *testing.T) {
	findings := []model.Finding{
		testFinding(model.ClassSQLi, model.SevHigh, model.StateValidated),
		testFinding(model.ClassXSSReflected, model.SevMedium, model.StateTheorized),
	}
	out, err := FindingsToSARIF(findings, "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	var log sarifLog
	if err := json.Unmarshal([]byte(out), &log); err != nil {
		t.Fatalf("invalid SARIF JSON: %v", err)
	}
	if log.Version != "2.1.0" {
		t.Errorf("version = %q, want 2.1.0", log.Version)
	}
	if len(log.Runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(log.Runs))
	}
	if log.Runs[0].Tool.Driver.Name != "assay" {
		t.Errorf("tool name = %q, want assay", log.Runs[0].Tool.Driver.Name)
	}
	if log.Runs[0].Tool.Driver.Version != "0.1.0" {
		t.Errorf("tool version = %q, want 0.1.0", log.Runs[0].Tool.Driver.Version)
	}
}

func TestFindingsToSARIF_SeverityMapping(t *testing.T) {
	cases := []struct {
		sev  model.Severity
		want string
	}{
		{model.SevCritical, "error"},
		{model.SevHigh, "error"},
		{model.SevMedium, "warning"},
		{model.SevLow, "note"},
		{model.SevInfo, "note"},
	}
	for _, tc := range cases {
		got := severityToSARIF(tc.sev)
		if got != tc.want {
			t.Errorf("severityToSARIF(%q) = %q, want %q", tc.sev, got, tc.want)
		}
	}
}

func TestFindingsToSARIF_EmptyFindings(t *testing.T) {
	out, err := FindingsToSARIF(nil, "dev")
	if err != nil {
		t.Fatal(err)
	}
	var log sarifLog
	if err := json.Unmarshal([]byte(out), &log); err != nil {
		t.Fatalf("invalid SARIF JSON for empty findings: %v", err)
	}
	if len(log.Runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(log.Runs))
	}
	if log.Runs[0].Results != nil {
		t.Errorf("results should be nil for empty findings, got %d", len(log.Runs[0].Results))
	}
}

func TestFindingsToSARIF_FiltersRefutedAndDeclined(t *testing.T) {
	findings := []model.Finding{
		testFinding(model.ClassSQLi, model.SevHigh, model.StateValidated),
		testFinding(model.ClassXSSReflected, model.SevMedium, model.StateRefuted),
		testFinding(model.ClassIDOR, model.SevHigh, model.StateDeclined),
		testFinding(model.ClassAuthBypass, model.SevCritical, model.StateTheorized),
	}
	out, err := FindingsToSARIF(findings, "dev")
	if err != nil {
		t.Fatal(err)
	}
	var log sarifLog
	if err := json.Unmarshal([]byte(out), &log); err != nil {
		t.Fatal(err)
	}
	if len(log.Runs[0].Results) != 2 {
		t.Errorf("results = %d, want 2 (only validated + theorized)", len(log.Runs[0].Results))
	}
}

func TestFindingsToSARIF_RuleDeduplication(t *testing.T) {
	f1 := testFinding(model.ClassSQLi, model.SevHigh, model.StateValidated)
	f2 := testFinding(model.ClassSQLi, model.SevMedium, model.StateTheorized)
	f2.Location.PathTemplate = "/api/v2/products/{id}"
	findings := []model.Finding{f1, f2}
	out, err := FindingsToSARIF(findings, "dev")
	if err != nil {
		t.Fatal(err)
	}
	var log sarifLog
	if err := json.Unmarshal([]byte(out), &log); err != nil {
		t.Fatal(err)
	}
	if len(log.Runs[0].Tool.Driver.Rules) != 1 {
		t.Errorf("rules = %d, want 1 (same class deduped)", len(log.Runs[0].Tool.Driver.Rules))
	}
	if len(log.Runs[0].Results) != 2 {
		t.Errorf("results = %d, want 2", len(log.Runs[0].Results))
	}
}

func TestFindingsToSARIF_LocationMapping(t *testing.T) {
	f := testFinding(model.ClassSQLi, model.SevHigh, model.StateValidated)
	out, err := FindingsToSARIF([]model.Finding{f}, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "/api/v1/users/{id}") {
		t.Error("SARIF should contain the path template as artifact URI")
	}
	if !strings.Contains(out, "param: id") {
		t.Error("SARIF message should mention the param")
	}
}

func TestFindingsToSARIF_DefaultVersion(t *testing.T) {
	out, err := FindingsToSARIF(nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"version": "dev"`) {
		t.Error("empty version should default to 'dev'")
	}
}

func TestFindingsToSARIF_CWEInHelp(t *testing.T) {
	f := testFinding(model.ClassSQLi, model.SevHigh, model.StateValidated)
	f.CWE = "CWE-89"
	out, err := FindingsToSARIF([]model.Finding{f}, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "CWE-89") {
		t.Error("SARIF rule help should include the CWE")
	}
}
