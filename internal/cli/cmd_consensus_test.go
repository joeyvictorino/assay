package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func writeTestFindings(t *testing.T, findings []model.Finding) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "findings.json")
	b, err := json.Marshal(findings)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestConsensusCommand(t *testing.T) {
	findings := []model.Finding{
		{
			ID: "f1", DedupKey: "f1", PromptHash: "p1",
			Model: model.ModelRef{Model: "gpt-4"}, State: model.StateValidated,
			Severity: model.SevHigh, Class: model.ClassSQLi,
		},
		{
			ID: "f2", DedupKey: "f2", PromptHash: "p1",
			Model: model.ModelRef{Model: "claude-3"}, State: model.StateValidated,
			Severity: model.SevHigh, Class: model.ClassSQLi,
		},
	}
	path := writeTestFindings(t, findings)

	var stdout, stderr bytes.Buffer
	code := runConsensus([]string{"--findings", path}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected exit 0, got %d: %s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Multi-Model Consensus")) {
		t.Error("expected consensus header in output")
	}
}

func TestConsensusCommandJSON(t *testing.T) {
	findings := []model.Finding{
		{
			ID: "f1", DedupKey: "f1", PromptHash: "p1",
			Model: model.ModelRef{Model: "gpt-4"}, State: model.StateValidated,
			Severity: model.SevHigh, Class: model.ClassSQLi,
		},
		{
			ID: "f2", DedupKey: "f2", PromptHash: "p1",
			Model: model.ModelRef{Model: "claude-3"}, State: model.StateRefuted,
			Severity: model.SevLow, Class: model.ClassSQLi,
		},
	}
	path := writeTestFindings(t, findings)

	var stdout, stderr bytes.Buffer
	code := runConsensus([]string{"--findings", path, "--format", "json"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected exit 0, got %d: %s", code, stderr.String())
	}
	var parsed []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
}

func TestConsensusMissingFindings(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runConsensus(nil, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected exit %d for missing findings, got %d", ExitError, code)
	}
}

func TestConsensusBadFormat(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runConsensus([]string{"--findings", "x.json", "--format", "xml"}, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected exit %d for bad format, got %d", ExitError, code)
	}
}
