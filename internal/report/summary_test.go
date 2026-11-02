package report

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/reconcile"
)

func TestBuildSummaryBasic(t *testing.T) {
	rr := model.RunReport{
		RunID:        "r1",
		Verdict:      "PASS",
		Started:      time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC),
		Finished:     time.Date(2026, 10, 7, 0, 1, 0, 0, time.UTC),
		SpentUSD:     0.05,
		BudgetCapUSD: 15,
		Labs:         []string{"dvwa"},
		Models: []model.ModelResult{
			{Ref: model.ModelRef{Model: "claude-sonnet"}, Findings: map[model.FindingState]int{model.StateValidated: 1}},
		},
	}
	findings := []model.Finding{
		{ID: "f1", Class: model.ClassSQLi, Severity: model.SevHigh, State: model.StateValidated, Model: model.ModelRef{Model: "claude-sonnet"}},
		{ID: "f2", Class: model.ClassXSSReflected, Severity: model.SevMedium, State: model.StateTheorized, Model: model.ModelRef{Model: "claude-sonnet"}},
	}
	rec := &reconcile.Report{Checked: 3, Mismatches: []reconcile.Mismatch{{ToolCallID: "c1"}}}

	s := BuildSummary(rr, findings, rec)

	if s.RunID != "r1" {
		t.Errorf("RunID = %q", s.RunID)
	}
	if s.TotalFindings != 2 {
		t.Errorf("TotalFindings = %d", s.TotalFindings)
	}
	if s.BySeverity["high"] != 1 || s.BySeverity["medium"] != 1 {
		t.Errorf("BySeverity = %v", s.BySeverity)
	}
	if s.ByClass["sqli"] != 1 {
		t.Errorf("ByClass = %v", s.ByClass)
	}
	if s.ByState["validated"] != 1 || s.ByState["theorized"] != 1 {
		t.Errorf("ByState = %v", s.ByState)
	}
	if s.Reconciled != 2 || s.Mismatched != 1 {
		t.Errorf("Reconciled=%d Mismatched=%d", s.Reconciled, s.Mismatched)
	}
	if s.DurationSec != 60 {
		t.Errorf("DurationSec = %f", s.DurationSec)
	}
}

func TestBuildSummaryEmpty(t *testing.T) {
	rr := model.RunReport{
		RunID:    "empty",
		Verdict:  "PASS",
		Started:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Finished: time.Date(2026, 1, 1, 0, 0, 30, 0, time.UTC),
	}
	s := BuildSummary(rr, nil, nil)
	if s.TotalFindings != 0 {
		t.Errorf("TotalFindings = %d", s.TotalFindings)
	}
	if s.Reconciled != 0 || s.Mismatched != 0 {
		t.Errorf("Reconciled=%d Mismatched=%d", s.Reconciled, s.Mismatched)
	}
}

func TestSummaryJSON(t *testing.T) {
	s := Summary{
		RunID:         "r1",
		Verdict:       "PASS",
		TotalFindings: 3,
		BySeverity:    map[string]int{"high": 2, "medium": 1},
		ByClass:       map[string]int{"sqli": 3},
		ByState:       map[string]int{"validated": 3},
		ByModel:       map[string]int{"test": 3},
	}
	js, err := SummaryJSON(s)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(js), &parsed); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	if parsed["run_id"] != "r1" {
		t.Errorf("run_id = %v", parsed["run_id"])
	}
	if parsed["total_findings"].(float64) != 3 {
		t.Errorf("total_findings = %v", parsed["total_findings"])
	}
}
