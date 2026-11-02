package cli

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/dashdata"
	"github.com/joeyvictorino/assay/internal/model"
)

func TestDashboardDataMissingPath(t *testing.T) {
	var out, errOut bytes.Buffer
	code := runDashboardData(nil, &out, &errOut)
	if code != ExitError {
		t.Errorf("exit code = %d, want %d", code, ExitError)
	}
}

func TestDashboardDataBasic(t *testing.T) {
	dir := t.TempDir()
	rr := model.RunReport{
		RunID:   "dash-test",
		Verdict: "PASS",
		Started:  time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC),
		Finished: time.Date(2025, 1, 1, 10, 5, 0, 0, time.UTC),
		Models:   []model.ModelResult{{Ref: model.ModelRef{Provider: "test", Model: "m1"}}},
		Reflection: []model.ReflectionResult{
			{Lab: "lab1", Model: "m1", Reviewer: "c1", Scored: true, Final: 7, Passed: true, Validated: 1},
		},
		WeightedScore: 7.0,
	}
	writeJSONFile(t, dir, "run.json", rr)

	var out, errOut bytes.Buffer
	code := runDashboardData([]string{"--path", dir}, &out, &errOut)
	if code != ExitPass {
		t.Fatalf("exit code = %d, want %d; stderr: %s", code, ExitPass, errOut.String())
	}

	var d dashdata.DashboardData
	if err := json.Unmarshal(out.Bytes(), &d); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if d.GeneratedAt == "" {
		t.Error("generated_at should be set")
	}
	if d.RunSummary.TotalModels != 1 {
		t.Errorf("total models = %d, want 1", d.RunSummary.TotalModels)
	}
}

func TestDashboardDataWithPrev(t *testing.T) {
	dir := t.TempDir()
	prevDir := t.TempDir()

	rr := model.RunReport{
		RunID:         "dash-curr",
		Verdict:       "PASS",
		Started:       time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC),
		Finished:      time.Date(2025, 1, 1, 10, 5, 0, 0, time.UTC),
		WeightedScore: 8.0,
	}
	prev := model.RunReport{
		RunID:         "dash-prev",
		Verdict:       "PASS",
		Started:       time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC),
		Finished:      time.Date(2025, 1, 1, 9, 5, 0, 0, time.UTC),
		WeightedScore: 2.0,
	}
	writeJSONFile(t, dir, "run.json", rr)
	writeJSONFile(t, prevDir, "run.json", prev)

	var out, errOut bytes.Buffer
	code := runDashboardData([]string{"--path", dir, "--prev", prevDir}, &out, &errOut)
	if code != ExitPass {
		t.Fatalf("exit code = %d, want %d; stderr: %s", code, ExitPass, errOut.String())
	}

	var d dashdata.DashboardData
	if err := json.Unmarshal(out.Bytes(), &d); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if d.RiskGauge.Trend != "degrading" {
		t.Errorf("trend = %s, want degrading", d.RiskGauge.Trend)
	}
}

func TestDashboardDataOutputFile(t *testing.T) {
	dir := t.TempDir()
	rr := model.RunReport{
		RunID:         "dash-file",
		Verdict:       "PASS",
		WeightedScore: 5.0,
	}
	writeJSONFile(t, dir, "run.json", rr)

	outFile := t.TempDir() + "/dashboard.json"
	var out, errOut bytes.Buffer
	code := runDashboardData([]string{"--path", dir, "--output", outFile}, &out, &errOut)
	if code != ExitPass {
		t.Fatalf("exit code = %d, want %d; stderr: %s", code, ExitPass, errOut.String())
	}
	if out.Len() != 0 {
		t.Error("stdout should be empty when --output is used")
	}
}
