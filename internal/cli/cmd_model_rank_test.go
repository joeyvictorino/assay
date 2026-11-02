package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/modelrank"
)

func TestModelRank_TextOutput(t *testing.T) {
	dir := t.TempDir()
	findings := []model.Finding{
		{Lab: "sqli", Model: model.ModelRef{Model: "gpt-4"}, Severity: model.SevHigh},
		{Lab: "sqli", Model: model.ModelRef{Model: "claude-3"}, Severity: model.SevCritical},
		{Lab: "xss", Model: model.ModelRef{Model: "gpt-4"}, Severity: model.SevLow},
	}
	setupCompareRun(t, dir, "run-1", findings)
	runDir := dir + "/run-1"

	var stdout, stderr bytes.Buffer
	code := runModelRank([]string{"--path", runDir}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "gpt-4") {
		t.Errorf("missing gpt-4:\n%s", out)
	}
	if !strings.Contains(out, "claude-3") {
		t.Errorf("missing claude-3:\n%s", out)
	}
	if !strings.Contains(out, "Leaderboard") {
		t.Errorf("missing header:\n%s", out)
	}
}

func TestModelRank_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	findings := []model.Finding{
		{Lab: "sqli", Model: model.ModelRef{Model: "gpt-4"}, Severity: model.SevHigh},
		{Lab: "sqli", Model: model.ModelRef{Model: "claude-3"}, Severity: model.SevMedium},
	}
	setupCompareRun(t, dir, "run-1", findings)
	runDir := dir + "/run-1"

	var stdout, stderr bytes.Buffer
	code := runModelRank([]string{"--path", runDir, "--format", "json"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}

	var lb modelrank.Leaderboard
	if err := json.Unmarshal(stdout.Bytes(), &lb); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if len(lb.Models) != 2 {
		t.Errorf("models = %d, want 2", len(lb.Models))
	}
}

func TestModelRank_MissingPath(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runModelRank(nil, &stdout, &stderr)
	if code != ExitError {
		t.Fatalf("expected exit %d, got %d", ExitError, code)
	}
}

func TestModelRank_TopN(t *testing.T) {
	dir := t.TempDir()
	findings := []model.Finding{
		{Lab: "sqli", Model: model.ModelRef{Model: "a"}, Severity: model.SevCritical},
		{Lab: "sqli", Model: model.ModelRef{Model: "b"}, Severity: model.SevHigh},
		{Lab: "sqli", Model: model.ModelRef{Model: "c"}, Severity: model.SevLow},
	}
	setupCompareRun(t, dir, "run-1", findings)
	runDir := dir + "/run-1"

	var stdout, stderr bytes.Buffer
	code := runModelRank([]string{"--path", runDir, "--format", "json", "--top", "2"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	var lb modelrank.Leaderboard
	if err := json.Unmarshal(stdout.Bytes(), &lb); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(lb.Models) != 2 {
		t.Errorf("models = %d, want 2", len(lb.Models))
	}
}

func TestModelRank_CheckerFilter(t *testing.T) {
	dir := t.TempDir()
	findings := []model.Finding{
		{Lab: "sqli", Model: model.ModelRef{Model: "gpt-4"}, Severity: model.SevHigh},
		{Lab: "xss", Model: model.ModelRef{Model: "gpt-4"}, Severity: model.SevLow},
	}
	setupCompareRun(t, dir, "run-1", findings)
	runDir := dir + "/run-1"

	var stdout, stderr bytes.Buffer
	code := runModelRank([]string{"--path", runDir, "--format", "json", "--checker", "sqli"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	var lb modelrank.Leaderboard
	if err := json.Unmarshal(stdout.Bytes(), &lb); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(lb.Checkers) != 1 || lb.Checkers[0] != "sqli" {
		t.Errorf("checkers = %v, want [sqli]", lb.Checkers)
	}
}
