package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/retry"
)

func TestRetryMissingPath(t *testing.T) {
	var out, errOut bytes.Buffer
	code := runRetry(nil, &out, &errOut)
	if code != ExitError {
		t.Errorf("exit code = %d, want %d", code, ExitError)
	}
}

func TestRetryBadFormat(t *testing.T) {
	var out, errOut bytes.Buffer
	code := runRetry([]string{"--path", "/tmp", "--format", "xml"}, &out, &errOut)
	if code != ExitError {
		t.Errorf("exit code = %d, want %d", code, ExitError)
	}
}


func TestRetryPassingRun(t *testing.T) {
	dir := t.TempDir()
	rr := model.RunReport{
		RunID:   "retry-pass",
		Verdict: "PASS",
		Models: []model.ModelResult{
			{
				Ref:   model.ModelRef{Provider: "openai", Model: "gpt-4o", Agent: "recon"},
				Usage: model.Usage{InputTokens: 100000, OutputTokens: 50000},
			},
		},
	}
	writeJSONFile(t, dir, "run.json", rr)

	var out, errOut bytes.Buffer
	code := runRetry([]string{"--path", dir}, &out, &errOut)
	if code != ExitPass {
		t.Fatalf("exit code = %d, want %d; stderr: %s", code, ExitPass, errOut.String())
	}
	if !strings.Contains(out.String(), "false") {
		t.Error("expected retry=false for passing run")
	}
}

func TestRetryFailedRun(t *testing.T) {
	dir := t.TempDir()
	rr := model.RunReport{
		RunID:    "retry-fail",
		Verdict:  "ERROR",
		ExitCode: 1,
		Models: []model.ModelResult{
			{
				Ref:       model.ModelRef{Provider: "openai", Model: "gpt-4o", Agent: "recon"},
				Failovers: 3,
			},
		},
	}
	writeJSONFile(t, dir, "run.json", rr)

	var out, errOut bytes.Buffer
	code := runRetry([]string{"--path", dir, "--max-retries", "5"}, &out, &errOut)
	if code != ExitPass {
		t.Fatalf("exit code = %d, want %d; stderr: %s", code, ExitPass, errOut.String())
	}
	if !strings.Contains(out.String(), "true") {
		t.Error("expected retry=true for failed run")
	}
	if !strings.Contains(out.String(), "Backoff Schedule") {
		t.Error("expected backoff schedule in output")
	}
}

func TestRetryJSONOutput(t *testing.T) {
	dir := t.TempDir()
	rr := model.RunReport{
		RunID:   "retry-json",
		Verdict: "BLOCKED",
		Models: []model.ModelResult{
			{
				Ref:      model.ModelRef{Provider: "anthropic", Model: "claude-sonnet-4", Agent: "probe"},
				Refusals: 5,
			},
		},
	}
	writeJSONFile(t, dir, "run.json", rr)

	var out, errOut bytes.Buffer
	code := runRetry([]string{"--path", dir, "--format", "json"}, &out, &errOut)
	if code != ExitPass {
		t.Fatalf("exit code = %d, want %d; stderr: %s", code, ExitPass, errOut.String())
	}

	var plan retry.RetryPlan
	if err := json.Unmarshal(out.Bytes(), &plan); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if !plan.ShouldRetry {
		t.Error("expected should_retry = true")
	}
	if len(plan.FailedModels) != 1 {
		t.Errorf("failed_models count = %d, want 1", len(plan.FailedModels))
	}
}

