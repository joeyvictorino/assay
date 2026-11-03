package report

import (
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestFindingsCSVHeader(t *testing.T) {
	out := FindingsCSV(nil)
	r := csv.NewReader(strings.NewReader(out))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 row (header only), got %d", len(records))
	}
	if records[0][0] != "id" {
		t.Errorf("first header should be 'id', got %q", records[0][0])
	}
}

func TestFindingsCSVRows(t *testing.T) {
	findings := []model.Finding{
		{
			ID:       "f-001",
			Class:    model.ClassSQLi,
			Severity: model.SevHigh,
			State:    model.StateValidated,
			Location: model.Location{Method: "POST", PathTemplate: "/api/login", Param: "username"},
			Lab:      "dvwa",
			Model:    model.ModelRef{Model: "claude-sonnet-5-5"},
			CWE:      "CWE-89",
			Summary:  "SQL injection in login form",
			Reconcile: model.ReconcileStatus{Checked: true, Match: true},
			FirstSeen: time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC),
		},
		{
			ID:       "f-002",
			Class:    model.ClassXSSReflected,
			Severity: model.SevMedium,
			State:    model.StateTheorized,
			Location: model.Location{Method: "GET", PathTemplate: "/search"},
			Lab:      "juiceshop",
			Model:    model.ModelRef{Model: "gpt-4o"},
			Summary:  "Reflected XSS in search parameter",
			Reconcile: model.ReconcileStatus{Checked: true, Match: false},
			FirstSeen: time.Date(2026, 10, 7, 13, 0, 0, 0, time.UTC),
		},
	}
	out := FindingsCSV(findings)
	r := csv.NewReader(strings.NewReader(out))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 rows (header + 2), got %d", len(records))
	}
	row := records[1]
	if row[0] != "f-001" {
		t.Errorf("id: got %q", row[0])
	}
	if row[1] != "sqli" {
		t.Errorf("class: got %q", row[1])
	}
	if row[2] != "high" {
		t.Errorf("severity: got %q", row[2])
	}
	if row[11] != "true" {
		t.Errorf("reconciled: got %q", row[11])
	}

	row2 := records[2]
	if row2[11] != "false" {
		t.Errorf("reconciled for mismatch: got %q", row2[11])
	}
}

func TestFindingsCSVEscaping(t *testing.T) {
	findings := []model.Finding{
		{
			ID:       "f-003",
			Summary:  `injection via "quotes" and, commas`,
			Location: model.Location{Method: "GET", PathTemplate: "/path"},
			Model:    model.ModelRef{Model: "m"},
			FirstSeen: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}
	out := FindingsCSV(findings)
	r := csv.NewReader(strings.NewReader(out))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("CSV with special chars should parse: %v", err)
	}
	if records[1][10] != `injection via "quotes" and, commas` {
		t.Errorf("summary not preserved: got %q", records[1][10])
	}
}
