package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestPromote_CopiesRunAndFindings(t *testing.T) {
	dir := t.TempDir()
	runDir := setupCompareRun(t, dir, "run-src", []model.Finding{
		{ID: "f1", DedupKey: "k1", State: model.StateValidated, Severity: model.SevHigh},
	})
	baseDir := filepath.Join(dir, "baseline")

	var stdout, stderr bytes.Buffer
	code := runPromote([]string{"--run", runDir, "--baseline-dir", baseDir}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}

	if _, err := os.Stat(filepath.Join(baseDir, runJSON)); err != nil {
		t.Errorf("run.json not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(baseDir, findingsJSON)); err != nil {
		t.Errorf("findings not copied: %v", err)
	}

	var meta promotedMeta
	b, err := os.ReadFile(filepath.Join(baseDir, "promoted.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &meta); err != nil {
		t.Fatal(err)
	}
	if meta.SourceRunID != "run-src" {
		t.Errorf("source_run_id = %q, want run-src", meta.SourceRunID)
	}
	if meta.PromotedAt == "" {
		t.Error("promoted_at is empty")
	}
}

func TestPromote_NoFindings(t *testing.T) {
	dir := t.TempDir()
	runDir := setupCompareRun(t, dir, "run-nof", nil)
	baseDir := filepath.Join(dir, "baseline")

	var stdout, stderr bytes.Buffer
	code := runPromote([]string{"--run", runDir, "--baseline-dir", baseDir}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(baseDir, runJSON)); err != nil {
		t.Error("run.json should be copied even without findings")
	}
	if _, err := os.Stat(filepath.Join(baseDir, findingsJSON)); !os.IsNotExist(err) {
		t.Error("findings file should not exist when source has none")
	}
}

func TestPromote_MissingRunJSON(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := runPromote([]string{"--run", dir, "--baseline-dir", filepath.Join(dir, "out")}, &stdout, &stderr)
	if code != ExitError {
		t.Fatalf("expected exit %d, got %d", ExitError, code)
	}
}

func TestPromote_CreatesBaselineDir(t *testing.T) {
	dir := t.TempDir()
	runDir := setupCompareRun(t, dir, "src", nil)
	nested := filepath.Join(dir, "a", "b", "c")

	var stdout, stderr bytes.Buffer
	code := runPromote([]string{"--run", runDir, "--baseline-dir", nested}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(nested, runJSON)); err != nil {
		t.Error("baseline dir should have been created recursively")
	}
}

func TestPromote_MissingArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runPromote(nil, &stdout, &stderr)
	if code != ExitError {
		t.Fatalf("expected exit %d, got %d", ExitError, code)
	}
}
