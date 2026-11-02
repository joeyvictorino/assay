package threshold

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestGenerateDefault(t *testing.T) {
	cfg := GenerateDefault()
	if cfg.DefaultMin != 5.0 {
		t.Errorf("expected default min 5.0, got %f", cfg.DefaultMin)
	}
	if len(cfg.Rules) != 4 {
		t.Errorf("expected 4 rules, got %d", len(cfg.Rules))
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "thresh.json")
	cfg := GenerateDefault()
	if err := SaveConfig(cfg, path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.DefaultMin != cfg.DefaultMin {
		t.Errorf("default min mismatch: %f vs %f", loaded.DefaultMin, cfg.DefaultMin)
	}
	if len(loaded.Rules) != len(cfg.Rules) {
		t.Errorf("rule count mismatch: %d vs %d", len(loaded.Rules), len(cfg.Rules))
	}
}

func TestLoadConfigNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/file.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadConfigInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte("not json"), 0644)
	_, err := LoadConfig(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestCheckPassed(t *testing.T) {
	cfg := ThresholdConfig{DefaultMin: 5.0}
	rr := model.RunReport{
		Reflection: []model.ReflectionResult{
			{Lab: "injection", Model: "gpt-4", Scored: true, Final: 8},
		},
	}
	r := Check(cfg, rr, nil)
	if !r.Passed {
		t.Error("expected pass")
	}
	if r.CheckCount != 1 {
		t.Errorf("expected 1 check, got %d", r.CheckCount)
	}
}

func TestCheckViolation(t *testing.T) {
	cfg := ThresholdConfig{
		Rules: []ThresholdRule{
			{Checker: "injection", MinScore: 8.0},
		},
	}
	rr := model.RunReport{
		Reflection: []model.ReflectionResult{
			{Lab: "injection", Model: "gpt-4", Scored: true, Final: 5},
		},
	}
	r := Check(cfg, rr, nil)
	if r.Passed {
		t.Error("expected fail")
	}
	if len(r.Violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(r.Violations))
	}
	if r.Violations[0].Delta != 3.0 {
		t.Errorf("expected delta 3.0, got %f", r.Violations[0].Delta)
	}
}

func TestCheckDefaultMin(t *testing.T) {
	cfg := ThresholdConfig{DefaultMin: 7.0}
	rr := model.RunReport{
		Reflection: []model.ReflectionResult{
			{Lab: "leakage", Model: "gpt-4", Scored: true, Final: 4},
		},
	}
	r := Check(cfg, rr, nil)
	if r.Passed {
		t.Error("expected fail from default min")
	}
}

func TestCheckModelFilter(t *testing.T) {
	cfg := ThresholdConfig{
		Rules: []ThresholdRule{
			{Checker: "injection", Model: "gpt-4", MinScore: 8.0},
		},
	}
	rr := model.RunReport{
		Reflection: []model.ReflectionResult{
			{Lab: "injection", Model: "gpt-4", Scored: true, Final: 5},
			{Lab: "injection", Model: "claude", Scored: true, Final: 5},
		},
	}
	r := Check(cfg, rr, nil)
	if len(r.Violations) != 1 {
		t.Errorf("expected 1 violation (only gpt-4), got %d", len(r.Violations))
	}
	if r.Violations[0].Model != "gpt-4" {
		t.Errorf("expected gpt-4 violation, got %s", r.Violations[0].Model)
	}
}

func TestCheckEmpty(t *testing.T) {
	cfg := ThresholdConfig{DefaultMin: 5.0}
	r := Check(cfg, model.RunReport{}, nil)
	if !r.Passed {
		t.Error("empty run should pass")
	}
}

func TestFormatTextPassed(t *testing.T) {
	r := ThresholdResult{Passed: true, CheckCount: 5}
	s := FormatText(r)
	if s == "" {
		t.Error("expected non-empty")
	}
}

func TestFormatTextFailed(t *testing.T) {
	r := ThresholdResult{
		Passed:     false,
		CheckCount: 3,
		Violations: []ThresholdViolation{
			{Rule: ThresholdRule{MinScore: 8.0, Checker: "injection"}, Actual: 5.0, Delta: 3.0, Model: "gpt-4", Checker: "injection"},
		},
	}
	s := FormatText(r)
	if s == "" {
		t.Error("expected non-empty")
	}
}

func TestFormatConfig(t *testing.T) {
	cfg := GenerateDefault()
	s := FormatConfig(cfg)
	if s == "" {
		t.Error("expected non-empty")
	}
}
