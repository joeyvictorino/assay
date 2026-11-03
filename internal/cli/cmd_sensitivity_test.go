package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/sensitivity"
)

func TestSensitivityMissingPath(t *testing.T) {
	var out, errOut bytes.Buffer
	code := runSensitivity(nil, &out, &errOut)
	if code != ExitError {
		t.Errorf("exit code = %d, want %d", code, ExitError)
	}
}

func TestSensitivityBadFormat(t *testing.T) {
	var out, errOut bytes.Buffer
	code := runSensitivity([]string{"--path", "/tmp", "--format", "xml"}, &out, &errOut)
	if code != ExitError {
		t.Errorf("exit code = %d, want %d", code, ExitError)
	}
}

func TestSensitivityTextOutput(t *testing.T) {
	dir := t.TempDir()
	rr := model.RunReport{
		RunID:   "sens-test",
		Verdict: "PASS",
		Reflection: []model.ReflectionResult{
			{Lab: "lab1", Model: "m1", Reviewer: "checker-a", PromptVariant: "v1", Scored: true, Final: 8},
			{Lab: "lab1", Model: "m1", Reviewer: "checker-a", PromptVariant: "v2", Scored: true, Final: 6},
		},
	}
	writeJSONFile(t, dir, "run.json", rr)

	var out, errOut bytes.Buffer
	code := runSensitivity([]string{"--path", dir}, &out, &errOut)
	if code != ExitPass {
		t.Fatalf("exit code = %d, want %d; stderr: %s", code, ExitPass, errOut.String())
	}
	if out.Len() == 0 {
		t.Error("expected text output")
	}
}

func TestSensitivityJSONOutput(t *testing.T) {
	dir := t.TempDir()
	rr := model.RunReport{
		RunID:   "sens-json",
		Verdict: "PASS",
		Reflection: []model.ReflectionResult{
			{Lab: "lab1", Model: "m1", Reviewer: "checker-a", PromptVariant: "v1", Scored: true, Final: 9},
			{Lab: "lab1", Model: "m1", Reviewer: "checker-a", PromptVariant: "v2", Scored: true, Final: 7},
		},
	}
	writeJSONFile(t, dir, "run.json", rr)

	var out, errOut bytes.Buffer
	code := runSensitivity([]string{"--path", dir, "--format", "json"}, &out, &errOut)
	if code != ExitPass {
		t.Fatalf("exit code = %d, want %d; stderr: %s", code, ExitPass, errOut.String())
	}

	var report sensitivity.SensitivityReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if len(report.Checkers) == 0 {
		t.Error("expected at least 1 checker in report")
	}
}

func TestSensitivityCheckerFilter(t *testing.T) {
	dir := t.TempDir()
	rr := model.RunReport{
		RunID:   "sens-filter",
		Verdict: "PASS",
		Reflection: []model.ReflectionResult{
			{Lab: "lab1", Model: "m1", Reviewer: "checker-a", PromptVariant: "v1", Scored: true, Final: 8},
			{Lab: "lab1", Model: "m1", Reviewer: "checker-a", PromptVariant: "v2", Scored: true, Final: 6},
			{Lab: "lab1", Model: "m1", Reviewer: "checker-b", PromptVariant: "v1", Scored: true, Final: 9},
			{Lab: "lab1", Model: "m1", Reviewer: "checker-b", PromptVariant: "v2", Scored: true, Final: 3},
		},
	}
	writeJSONFile(t, dir, "run.json", rr)

	var out, errOut bytes.Buffer
	code := runSensitivity([]string{"--path", dir, "--format", "json", "--checker", "checker-a"}, &out, &errOut)
	if code != ExitPass {
		t.Fatalf("exit code = %d; stderr: %s", code, errOut.String())
	}

	var report sensitivity.SensitivityReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if len(report.Checkers) != 1 {
		t.Errorf("expected 1 checker after filter, got %d", len(report.Checkers))
	}
}
