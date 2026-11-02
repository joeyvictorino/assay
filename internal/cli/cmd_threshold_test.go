package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/threshold"
)

func writeThresholdConfig(t *testing.T, dir string, cfg threshold.ThresholdConfig) string {
	t.Helper()
	p := filepath.Join(dir, "thresh.json")
	if err := threshold.SaveConfig(cfg, p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestThresholdCLI_NoSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runThreshold(nil, &stdout, &stderr)
	if code != ExitError {
		t.Fatalf("expected ExitError, got %d", code)
	}
}

func TestThresholdCLI_UnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runThreshold([]string{"bogus"}, &stdout, &stderr)
	if code != ExitError {
		t.Fatalf("expected ExitError, got %d", code)
	}
}

func TestThresholdCLI_CheckPass(t *testing.T) {
	dir := t.TempDir()
	cfg := threshold.ThresholdConfig{DefaultMin: 5.0}
	cfgPath := writeThresholdConfig(t, dir, cfg)

	rr := model.RunReport{
		RunID: "r1",
		Reflection: []model.ReflectionResult{
			{Lab: "injection", Model: "gpt-4", Scored: true, Final: 8},
		},
	}
	writeJSONFile(t, dir, runJSON, rr)

	var stdout, stderr bytes.Buffer
	code := runThreshold([]string{"check", "--config", cfgPath, "--path", dir}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "PASSED") {
		t.Error("expected PASSED in output")
	}
}

func TestThresholdCLI_CheckFail(t *testing.T) {
	dir := t.TempDir()
	cfg := threshold.ThresholdConfig{
		Rules: []threshold.ThresholdRule{
			{Checker: "injection", MinScore: 9.0},
		},
	}
	cfgPath := writeThresholdConfig(t, dir, cfg)

	rr := model.RunReport{
		RunID: "r1",
		Reflection: []model.ReflectionResult{
			{Lab: "injection", Model: "gpt-4", Scored: true, Final: 5},
		},
	}
	writeJSONFile(t, dir, runJSON, rr)

	var stdout, stderr bytes.Buffer
	code := runThreshold([]string{"check", "--config", cfgPath, "--path", dir}, &stdout, &stderr)
	if code != ExitFail {
		t.Fatalf("expected ExitFail, got %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "FAILED") {
		t.Error("expected FAILED in output")
	}
}

func TestThresholdCLI_CheckJSON(t *testing.T) {
	dir := t.TempDir()
	cfg := threshold.ThresholdConfig{DefaultMin: 5.0}
	cfgPath := writeThresholdConfig(t, dir, cfg)

	rr := model.RunReport{
		RunID: "r1",
		Reflection: []model.ReflectionResult{
			{Lab: "x", Model: "gpt-4", Scored: true, Final: 8},
		},
	}
	writeJSONFile(t, dir, runJSON, rr)

	var stdout, stderr bytes.Buffer
	code := runThreshold([]string{"check", "--config", cfgPath, "--path", dir, "--format", "json"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	var result threshold.ThresholdResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !result.Passed {
		t.Error("expected passed=true")
	}
}

func TestThresholdCLI_CheckMissingArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runThreshold([]string{"check"}, &stdout, &stderr)
	if code != ExitError {
		t.Fatalf("expected ExitError, got %d", code)
	}
}

func TestThresholdCLI_InitStdout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runThreshold([]string{"init"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	var cfg threshold.ThresholdConfig
	if err := json.Unmarshal(stdout.Bytes(), &cfg); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if cfg.DefaultMin != 5.0 {
		t.Errorf("expected default_min 5.0, got %f", cfg.DefaultMin)
	}
	if len(cfg.Rules) != 4 {
		t.Errorf("expected 4 rules, got %d", len(cfg.Rules))
	}
}

func TestThresholdCLI_InitToFile(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "config.json")

	var stdout, stderr bytes.Buffer
	code := runThreshold([]string{"init", "--output", outPath}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	b, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	var cfg threshold.ThresholdConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		t.Fatalf("invalid JSON in file: %v", err)
	}
	if cfg.DefaultMin != 5.0 {
		t.Errorf("expected default_min 5.0, got %f", cfg.DefaultMin)
	}
}

func TestThresholdCLI_Show(t *testing.T) {
	dir := t.TempDir()
	cfg := threshold.GenerateDefault()
	cfgPath := writeThresholdConfig(t, dir, cfg)

	var stdout, stderr bytes.Buffer
	code := runThreshold([]string{"show", "--config", cfgPath}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "prompt_injection") {
		t.Error("expected prompt_injection in output")
	}
	if !strings.Contains(out, "Default minimum") {
		t.Error("expected default minimum label")
	}
}

func TestThresholdCLI_ShowMissingConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runThreshold([]string{"show"}, &stdout, &stderr)
	if code != ExitError {
		t.Fatalf("expected ExitError, got %d", code)
	}
}

func TestThresholdCLI_CheckWriteToFile(t *testing.T) {
	dir := t.TempDir()
	cfg := threshold.ThresholdConfig{DefaultMin: 5.0}
	cfgPath := writeThresholdConfig(t, dir, cfg)

	rr := model.RunReport{
		RunID: "r1",
		Reflection: []model.ReflectionResult{
			{Lab: "x", Model: "gpt-4", Scored: true, Final: 8},
		},
	}
	writeJSONFile(t, dir, runJSON, rr)

	outFile := filepath.Join(dir, "result.txt")
	var stdout, stderr bytes.Buffer
	code := runThreshold([]string{"check", "--config", cfgPath, "--path", dir, "--out", outFile}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	b, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "PASSED") {
		t.Error("expected PASSED in file output")
	}
}
