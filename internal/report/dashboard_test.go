package report

import (
	"strings"
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
)

func testRuns() []model.RunReport {
	return []model.RunReport{
		{
			RunID:   "run-aaa111222333",
			Started: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
			Verdict: "pass",
			Models: []model.ModelResult{
				{
					Ref: model.ModelRef{Model: "claude-sonnet-4-20250514"},
					Findings: map[model.FindingState]int{
						model.StateValidated: 5,
						model.StateTheorized: 2,
					},
					Refusals: 1,
					Usage:    model.Usage{CostUSD: 0.0123},
					P50MS:    200,
					P95MS:    800,
				},
			},
			Overlap: &model.OverlapMatrix{Union: 7, Intersection: 4},
		},
		{
			RunID:   "run-bbb444555666",
			Started: time.Date(2026, 6, 2, 14, 0, 0, 0, time.UTC),
			Verdict: "fail",
			Models: []model.ModelResult{
				{
					Ref:      model.ModelRef{Model: "gpt-4o"},
					Findings: map[model.FindingState]int{model.StateValidated: 3},
					Refusals: 0,
					Usage:    model.Usage{CostUSD: 0.045},
					P50MS:    350,
					P95MS:    1200,
				},
			},
		},
	}
}

func TestDashboardHTML_Doctype(t *testing.T) {
	h := DashboardHTML(testRuns())
	if !strings.HasPrefix(h, "<!DOCTYPE html>") {
		t.Error("should start with <!DOCTYPE html>")
	}
}

func TestDashboardHTML_ContainsTitle(t *testing.T) {
	h := DashboardHTML(testRuns())
	if !strings.Contains(h, "<title>assay dashboard</title>") {
		t.Error("should contain the page title")
	}
}

func TestDashboardHTML_SummaryCards(t *testing.T) {
	h := DashboardHTML(testRuns())
	if !strings.Contains(h, "Runs") {
		t.Error("should contain Runs card")
	}
	if !strings.Contains(h, "Models") {
		t.Error("should contain Models card")
	}
	if !strings.Contains(h, "Validated") {
		t.Error("should contain Validated card")
	}
}

func TestDashboardHTML_RunTable(t *testing.T) {
	h := DashboardHTML(testRuns())
	if !strings.Contains(h, "Run History") {
		t.Error("should contain Run History heading")
	}
	if !strings.Contains(h, "run-aaa11122") {
		t.Error("should contain truncated run ID from first run")
	}
	if !strings.Contains(h, "run-bbb44455") {
		t.Error("should contain truncated run ID from second run")
	}
}

func TestDashboardHTML_VerdictClasses(t *testing.T) {
	h := DashboardHTML(testRuns())
	if !strings.Contains(h, `class="pass"`) {
		t.Error("should contain pass class for passing run")
	}
	if !strings.Contains(h, `class="fail"`) {
		t.Error("should contain fail class for failing run")
	}
}

func TestDashboardHTML_ModelBreakdown(t *testing.T) {
	h := DashboardHTML(testRuns())
	if !strings.Contains(h, "Model Breakdown") {
		t.Error("should contain Model Breakdown heading")
	}
	if !strings.Contains(h, "gpt-4o") {
		t.Error("should contain model name from latest run")
	}
}

func TestDashboardHTML_DarkMode(t *testing.T) {
	h := DashboardHTML(testRuns())
	if !strings.Contains(h, "prefers-color-scheme: dark") {
		t.Error("should contain dark mode media query")
	}
}

func TestDashboardHTML_EmbeddedJSON(t *testing.T) {
	h := DashboardHTML(testRuns())
	if !strings.Contains(h, "const DATA=") {
		t.Error("should embed JSON data in script tag")
	}
}

func TestDashboardHTML_Empty(t *testing.T) {
	h := DashboardHTML(nil)
	if !strings.Contains(h, "0 run(s)") {
		t.Error("empty input should show 0 runs")
	}
	if strings.Contains(h, "Model Breakdown") {
		t.Error("empty input should not show model breakdown")
	}
}

func TestDashboardHTML_OverlapColumn(t *testing.T) {
	h := DashboardHTML(testRuns())
	if !strings.Contains(h, "4/7") {
		t.Error("should contain overlap ratio 4/7 for first run")
	}
}
