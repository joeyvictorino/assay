package report

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
)

func trendRuns() []model.RunReport {
	return []model.RunReport{
		{
			RunID:   "run-001",
			Started: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
			Verdict: "pass",
			Models: []model.ModelResult{
				{
					Ref:      model.ModelRef{Model: "claude-sonnet"},
					Findings: map[model.FindingState]int{model.StateValidated: 10},
					Usage:    model.Usage{CostUSD: 0.05},
				},
			},
		},
		{
			RunID:   "run-002",
			Started: time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC),
			Verdict: "pass",
			Models: []model.ModelResult{
				{
					Ref:      model.ModelRef{Model: "claude-sonnet"},
					Findings: map[model.FindingState]int{model.StateValidated: 12},
					Usage:    model.Usage{CostUSD: 0.06},
				},
			},
		},
		{
			RunID:   "run-003",
			Started: time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC),
			Verdict: "pass",
			Models: []model.ModelResult{
				{
					Ref:      model.ModelRef{Model: "claude-sonnet"},
					Findings: map[model.FindingState]int{model.StateValidated: 14},
					Usage:    model.Usage{CostUSD: 0.07},
				},
			},
		},
		{
			RunID:   "run-004",
			Started: time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC),
			Verdict: "fail",
			Models: []model.ModelResult{
				{
					Ref:      model.ModelRef{Model: "claude-sonnet"},
					Findings: map[model.FindingState]int{model.StateValidated: 16, model.StateTheorized: 3},
					Usage:    model.Usage{CostUSD: 0.08},
				},
				{
					Ref:      model.ModelRef{Model: "gpt-4o"},
					Findings: map[model.FindingState]int{model.StateValidated: 8},
					Refusals: 2,
					Usage:    model.Usage{CostUSD: 0.04},
				},
			},
		},
	}
}

func TestBuildTrend_RunCount(t *testing.T) {
	tr := BuildTrend(trendRuns())
	if tr.RunCount != 4 {
		t.Errorf("RunCount = %d, want 4", tr.RunCount)
	}
}

func TestBuildTrend_Direction_Increasing(t *testing.T) {
	tr := BuildTrend(trendRuns())
	if tr.Direction != "increasing" {
		t.Errorf("Direction = %q, want increasing", tr.Direction)
	}
}

func TestBuildTrend_CostTrend_Increasing(t *testing.T) {
	tr := BuildTrend(trendRuns())
	if tr.CostTrend != "increasing" {
		t.Errorf("CostTrend = %q, want increasing", tr.CostTrend)
	}
}

func TestBuildTrend_FailStreak(t *testing.T) {
	tr := BuildTrend(trendRuns())
	if tr.FailStreak != 1 {
		t.Errorf("FailStreak = %d, want 1", tr.FailStreak)
	}
	if tr.PassStreak != 0 {
		t.Errorf("PassStreak = %d, want 0", tr.PassStreak)
	}
}

func TestBuildTrend_PassStreak(t *testing.T) {
	runs := trendRuns()[:3]
	tr := BuildTrend(runs)
	if tr.PassStreak != 3 {
		t.Errorf("PassStreak = %d, want 3", tr.PassStreak)
	}
}

func TestBuildTrend_ModelPresence(t *testing.T) {
	tr := BuildTrend(trendRuns())
	if tr.ModelPresence["claude-sonnet"] != 4 {
		t.Errorf("claude-sonnet presence = %d, want 4", tr.ModelPresence["claude-sonnet"])
	}
	if tr.ModelPresence["gpt-4o"] != 1 {
		t.Errorf("gpt-4o presence = %d, want 1", tr.ModelPresence["gpt-4o"])
	}
}

func TestBuildTrend_ModelStability(t *testing.T) {
	tr := BuildTrend(trendRuns())
	if tr.ModelStability < 0.5 || tr.ModelStability > 1.0 {
		t.Errorf("ModelStability = %f, want between 0.5 and 1.0", tr.ModelStability)
	}
}

func TestBuildTrend_Empty(t *testing.T) {
	tr := BuildTrend(nil)
	if tr.RunCount != 0 {
		t.Errorf("RunCount = %d, want 0", tr.RunCount)
	}
	if tr.Direction != "stable" {
		t.Errorf("Direction = %q, want stable", tr.Direction)
	}
}

func TestBuildTrend_SingleRun(t *testing.T) {
	tr := BuildTrend(trendRuns()[:1])
	if tr.RunCount != 1 {
		t.Errorf("RunCount = %d, want 1", tr.RunCount)
	}
	if tr.Direction != "stable" {
		t.Errorf("Direction = %q, want stable for single run", tr.Direction)
	}
}

func TestTrendJSON_Valid(t *testing.T) {
	tr := BuildTrend(trendRuns())
	js, err := TrendJSON(tr)
	if err != nil {
		t.Fatal(err)
	}
	var parsed TrendReport
	if err := json.Unmarshal([]byte(js), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.RunCount != 4 {
		t.Errorf("parsed RunCount = %d, want 4", parsed.RunCount)
	}
}

func TestBuildTrend_RunEntries(t *testing.T) {
	tr := BuildTrend(trendRuns())
	if len(tr.Runs) != 4 {
		t.Fatalf("Runs len = %d, want 4", len(tr.Runs))
	}
	last := tr.Runs[3]
	if last.Validated != 24 {
		t.Errorf("last run validated = %d, want 24", last.Validated)
	}
	if last.Refusals != 2 {
		t.Errorf("last run refusals = %d, want 2", last.Refusals)
	}
	if last.Models != 2 {
		t.Errorf("last run models = %d, want 2", last.Models)
	}
}
