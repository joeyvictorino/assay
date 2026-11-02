package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestStabilityCommand(t *testing.T) {
	findings := []model.Finding{
		{
			ID: "f1", DedupKey: "f1", PromptHash: "p1",
			Model: model.ModelRef{Model: "gpt-4"}, State: model.StateValidated,
			Severity: model.SevHigh, Class: model.ClassSQLi,
		},
		{
			ID: "f2", DedupKey: "f2", PromptHash: "p1",
			Model: model.ModelRef{Model: "gpt-4"}, State: model.StateValidated,
			Severity: model.SevHigh, Class: model.ClassSQLi,
		},
		{
			ID: "f3", DedupKey: "f3", PromptHash: "p1",
			Model: model.ModelRef{Model: "gpt-4"}, State: model.StateRefuted,
			Severity: model.SevLow, Class: model.ClassSQLi,
		},
	}
	path := writeTestFindings(t, findings)

	var stdout, stderr bytes.Buffer
	code := runStability([]string{"--findings", path}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected exit 0, got %d: %s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Prompt Stability")) {
		t.Error("expected stability header in output")
	}
}

func TestStabilityCommandJSON(t *testing.T) {
	findings := []model.Finding{
		{
			ID: "f1", DedupKey: "f1", PromptHash: "p1",
			Model: model.ModelRef{Model: "gpt-4"}, State: model.StateValidated,
			Severity: model.SevHigh, Class: model.ClassSQLi,
		},
		{
			ID: "f2", DedupKey: "f2", PromptHash: "p1",
			Model: model.ModelRef{Model: "gpt-4"}, State: model.StateValidated,
			Severity: model.SevHigh, Class: model.ClassSQLi,
		},
		{
			ID: "f3", DedupKey: "f3", PromptHash: "p1",
			Model: model.ModelRef{Model: "gpt-4"}, State: model.StateValidated,
			Severity: model.SevHigh, Class: model.ClassSQLi,
		},
	}
	path := writeTestFindings(t, findings)

	var stdout, stderr bytes.Buffer
	code := runStability([]string{"--findings", path, "--format", "json"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected exit 0, got %d: %s", code, stderr.String())
	}
	var parsed []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
}

func TestStabilityMissingFindings(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runStability(nil, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected exit %d for missing findings, got %d", ExitError, code)
	}
}

func TestStabilityBadFormat(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runStability([]string{"--findings", "x.json", "--format", "xml"}, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected exit %d for bad format, got %d", ExitError, code)
	}
}
