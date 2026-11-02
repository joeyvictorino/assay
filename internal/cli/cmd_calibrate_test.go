package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/joeyvictorino/assay/internal/calibrate"
	"github.com/joeyvictorino/assay/internal/model"
)

func TestCalibrateMissingPath(t *testing.T) {
	var out, errOut bytes.Buffer
	code := runCalibrate(nil, &out, &errOut)
	if code != ExitError {
		t.Errorf("exit code = %d, want %d", code, ExitError)
	}
}

func TestCalibrateBadFormat(t *testing.T) {
	var out, errOut bytes.Buffer
	code := runCalibrate([]string{"--path", "/tmp", "--format", "xml"}, &out, &errOut)
	if code != ExitError {
		t.Errorf("exit code = %d, want %d", code, ExitError)
	}
}

func TestCalibrateTextOutput(t *testing.T) {
	dir := t.TempDir()
	rr := model.RunReport{
		RunID:   "cal-test",
		Verdict: "PASS",
		Reflection: []model.ReflectionResult{
			{Lab: "lab1", Model: "gpt-4o", ReviewerKind: "self", Scored: true, Final: 8},
			{Lab: "lab1", Model: "gpt-4o", ReviewerKind: "peer", Scored: true, Final: 7},
		},
	}
	writeJSONFile(t, dir, "run.json", rr)

	var out, errOut bytes.Buffer
	code := runCalibrate([]string{"--path", dir}, &out, &errOut)
	if code != ExitPass {
		t.Fatalf("exit code = %d, want %d; stderr: %s", code, ExitPass, errOut.String())
	}
	if out.Len() == 0 {
		t.Error("expected text output")
	}
}

func TestCalibrateJSONOutput(t *testing.T) {
	dir := t.TempDir()
	rr := model.RunReport{
		RunID:   "cal-test-json",
		Verdict: "PASS",
		Reflection: []model.ReflectionResult{
			{Lab: "lab1", Model: "claude", ReviewerKind: "self", Scored: true, Final: 9},
			{Lab: "lab1", Model: "claude", ReviewerKind: "peer", Scored: true, Final: 8},
		},
	}
	writeJSONFile(t, dir, "run.json", rr)

	var out, errOut bytes.Buffer
	code := runCalibrate([]string{"--path", dir, "--format", "json"}, &out, &errOut)
	if code != ExitPass {
		t.Fatalf("exit code = %d, want %d; stderr: %s", code, ExitPass, errOut.String())
	}

	var report calibrate.CalibrationReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
}

func TestCalibrateCheckerFilter(t *testing.T) {
	dir := t.TempDir()
	rr := model.RunReport{
		RunID:   "cal-filter",
		Verdict: "PASS",
		Reflection: []model.ReflectionResult{
			{Lab: "lab1", Model: "gpt-4o", ReviewerKind: "self", Scored: true, Final: 8},
			{Lab: "lab1", Model: "gpt-4o", ReviewerKind: "peer", Scored: true, Final: 7},
			{Lab: "lab1", Model: "claude", ReviewerKind: "self", Scored: true, Final: 9},
			{Lab: "lab1", Model: "claude", ReviewerKind: "peer", Scored: true, Final: 9},
		},
	}
	writeJSONFile(t, dir, "run.json", rr)

	var out, errOut bytes.Buffer
	code := runCalibrate([]string{"--path", dir, "--format", "json", "--checker", "claude"}, &out, &errOut)
	if code != ExitPass {
		t.Fatalf("exit code = %d; stderr: %s", code, errOut.String())
	}

	var report calibrate.CalibrationReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if len(report.Checkers) != 1 {
		t.Errorf("expected 1 checker after filter, got %d", len(report.Checkers))
	}
}

