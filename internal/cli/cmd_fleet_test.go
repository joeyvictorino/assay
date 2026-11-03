package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/fleet"
	"github.com/joeyvictorino/assay/internal/model"
)

func setupFleetRun(t *testing.T, dir string, rr model.RunReport, findings []model.Finding) string {
	t.Helper()
	writeJSONFile(t, dir, runJSON, rr)
	if len(findings) > 0 {
		writeJSONFile(t, dir, findingsJSON, findings)
	}
	return dir
}

func TestFleetCLI_MissingPath(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runFleet(nil, &stdout, &stderr)
	if code != ExitError {
		t.Fatalf("expected ExitError, got %d", code)
	}
}

func TestFleetCLI_BadFormat(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runFleet([]string{"--path", "/tmp", "--format", "xml"}, &stdout, &stderr)
	if code != ExitError {
		t.Fatalf("expected ExitError, got %d", code)
	}
}

func TestFleetCLI_BadSort(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runFleet([]string{"--path", "/tmp", "--sort", "cost"}, &stdout, &stderr)
	if code != ExitError {
		t.Fatalf("expected ExitError, got %d", code)
	}
}

func TestFleetCLI_TextOutput(t *testing.T) {
	dir := t.TempDir()
	rr := model.RunReport{
		RunID: "r1",
		Reflection: []model.ReflectionResult{
			{Lab: "injection", Model: "gpt-4", Scored: true, Final: 8, Passed: true},
			{Lab: "injection", Model: "claude", Scored: true, Final: 6, Passed: false},
		},
	}
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "gpt-4"}, DedupKey: "f1"},
	}
	setupFleetRun(t, dir, rr, findings)

	var stdout, stderr bytes.Buffer
	code := runFleet([]string{"--path", dir}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "gpt-4") {
		t.Error("expected gpt-4 in output")
	}
	if !strings.Contains(out, "claude") {
		t.Error("expected claude in output")
	}
}

func TestFleetCLI_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	rr := model.RunReport{
		RunID: "r1",
		Reflection: []model.ReflectionResult{
			{Lab: "x", Model: "gpt-4", Scored: true, Final: 7, Passed: true},
		},
	}
	setupFleetRun(t, dir, rr, nil)

	var stdout, stderr bytes.Buffer
	code := runFleet([]string{"--path", dir, "--format", "json"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	var report fleet.FleetReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(report.Models) != 1 {
		t.Errorf("expected 1 model, got %d", len(report.Models))
	}
}

func TestFleetCLI_ModelFilter(t *testing.T) {
	dir := t.TempDir()
	rr := model.RunReport{
		RunID: "r1",
		Reflection: []model.ReflectionResult{
			{Lab: "x", Model: "gpt-4", Scored: true, Final: 8, Passed: true},
			{Lab: "x", Model: "claude", Scored: true, Final: 6, Passed: true},
		},
	}
	setupFleetRun(t, dir, rr, nil)

	var stdout, stderr bytes.Buffer
	code := runFleet([]string{"--path", dir, "--format", "json", "--model", "claude"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	var report fleet.FleetReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(report.Models) != 1 || report.Models[0].Model != "claude" {
		t.Errorf("expected only claude, got %v", report.Models)
	}
}

func TestFleetCLI_SortByFindings(t *testing.T) {
	dir := t.TempDir()
	rr := model.RunReport{
		RunID: "r1",
		Reflection: []model.ReflectionResult{
			{Lab: "x", Model: "gpt-4", Scored: true, Final: 9, Passed: true},
			{Lab: "x", Model: "claude", Scored: true, Final: 7, Passed: true},
		},
	}
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "claude"}, DedupKey: "f1"},
		{Model: model.ModelRef{Model: "claude"}, DedupKey: "f2"},
		{Model: model.ModelRef{Model: "claude"}, DedupKey: "f3"},
		{Model: model.ModelRef{Model: "gpt-4"}, DedupKey: "f1"},
	}
	setupFleetRun(t, dir, rr, findings)

	var stdout, stderr bytes.Buffer
	code := runFleet([]string{"--path", dir, "--format", "json", "--sort", "findings"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	var report fleet.FleetReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(report.Models) < 2 {
		t.Fatal("expected at least 2 models")
	}
	if report.Models[0].Model != "claude" {
		t.Errorf("expected claude first (most findings), got %s", report.Models[0].Model)
	}
}

func TestFleetCLI_WriteToFile(t *testing.T) {
	dir := t.TempDir()
	rr := model.RunReport{
		RunID: "r1",
		Reflection: []model.ReflectionResult{
			{Lab: "x", Model: "gpt-4", Scored: true, Final: 7, Passed: true},
		},
	}
	setupFleetRun(t, dir, rr, nil)

	outFile := filepath.Join(dir, "out.txt")
	var stdout, stderr bytes.Buffer
	code := runFleet([]string{"--path", dir, "--out", outFile}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	b, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "gpt-4") {
		t.Error("expected gpt-4 in file output")
	}
}
