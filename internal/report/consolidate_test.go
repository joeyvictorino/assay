package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestLoadRuns(t *testing.T) {
	dir := t.TempDir()
	r := model.RunReport{
		RunID:   "run-001",
		Started: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Verdict: "PASS",
	}
	b, _ := json.MarshalIndent(r, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "run-001.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "not-a-run.json"), []byte("{\"invalid\": true}"), 0o644)
	os.WriteFile(filepath.Join(dir, "skip.txt"), []byte("not json"), 0o644)

	runs, err := LoadRuns(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 {
		t.Errorf("expected 1 run, got %d", len(runs))
	}
	if runs[0].RunID != "run-001" {
		t.Errorf("run ID = %q, want run-001", runs[0].RunID)
	}
}

func TestConsolidateEmpty(t *testing.T) {
	cr := Consolidate(nil)
	if cr.TotalRuns != 0 {
		t.Errorf("total = %d, want 0", cr.TotalRuns)
	}
	if cr.Trend != "none" {
		t.Errorf("trend = %q, want none", cr.Trend)
	}
}

func TestConsolidateTrend(t *testing.T) {
	runs := []model.RunReport{
		{RunID: "r1", Started: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Verdict: "BLOCKED"},
		{RunID: "r2", Started: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC), Verdict: "BLOCKED"},
		{RunID: "r3", Started: time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC), Verdict: "PASS"},
		{RunID: "r4", Started: time.Date(2025, 1, 4, 0, 0, 0, 0, time.UTC), Verdict: "PASS"},
	}
	cr := Consolidate(runs)
	if cr.TotalRuns != 4 {
		t.Errorf("total = %d, want 4", cr.TotalRuns)
	}
	if cr.PassCount != 2 {
		t.Errorf("pass = %d, want 2", cr.PassCount)
	}
	if cr.Trend != "improving" {
		t.Errorf("trend = %q, want improving", cr.Trend)
	}
}

func TestConsolidateDegrading(t *testing.T) {
	runs := []model.RunReport{
		{RunID: "r1", Started: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Verdict: "PASS"},
		{RunID: "r2", Started: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC), Verdict: "PASS"},
		{RunID: "r3", Started: time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC), Verdict: "BLOCKED"},
		{RunID: "r4", Started: time.Date(2025, 1, 4, 0, 0, 0, 0, time.UTC), Verdict: "BLOCKED"},
	}
	cr := Consolidate(runs)
	if cr.Trend != "degrading" {
		t.Errorf("trend = %q, want degrading", cr.Trend)
	}
}

func TestFormatConsolidateText(t *testing.T) {
	cr := ConsolidatedReport{
		TotalRuns:    2,
		PassCount:    1,
		BlockedCount: 1,
		FirstRun:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		LastRun:      time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
		Trend:        "stable",
		Runs: []RunSummaryItem{
			{RunID: "r1", Started: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Verdict: "PASS", Models: 3, SpentUSD: 1.50},
			{RunID: "r2", Started: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC), Verdict: "BLOCKED", Models: 3, SpentUSD: 2.00},
		},
	}
	text := FormatConsolidateText(cr)
	for _, want := range []string{"Consolidated Report:", "Pass: 1", "Blocked: 1", "stable", "r1", "r2"} {
		if !strings.Contains(text, want) {
			t.Errorf("text should contain %q", want)
		}
	}
}

func TestFormatConsolidateHTML(t *testing.T) {
	cr := ConsolidatedReport{
		TotalRuns:    1,
		PassCount:    1,
		FirstRun:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		LastRun:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Trend:        "stable",
		Runs: []RunSummaryItem{
			{RunID: "r1", Started: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Verdict: "PASS", Models: 2},
		},
	}
	html := FormatConsolidateHTML(cr)
	if !strings.Contains(html, "<table>") {
		t.Error("html should contain table")
	}
	if !strings.Contains(html, "PASS") {
		t.Error("html should contain verdict")
	}
}
