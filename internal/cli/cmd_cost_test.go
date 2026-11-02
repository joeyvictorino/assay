package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/joeyvictorino/assay/internal/costbreak"
	"github.com/joeyvictorino/assay/internal/model"
)

func TestCostMissingPath(t *testing.T) {
	var out, errOut bytes.Buffer
	code := runCost(nil, &out, &errOut)
	if code != ExitError {
		t.Errorf("exit code = %d, want %d", code, ExitError)
	}
}

func TestCostBadFormat(t *testing.T) {
	var out, errOut bytes.Buffer
	code := runCost([]string{"--path", "/tmp", "--format", "xml"}, &out, &errOut)
	if code != ExitError {
		t.Errorf("exit code = %d, want %d", code, ExitError)
	}
}

func TestCostTextOutput(t *testing.T) {
	dir := t.TempDir()
	rr := model.RunReport{
		RunID:   "cost-test",
		Verdict: "PASS",
		Models: []model.ModelResult{
			{
				Ref:   model.ModelRef{Provider: "openai", Model: "gpt-4o", Agent: "recon"},
				Usage: model.Usage{InputTokens: 500000, OutputTokens: 200000, CostUSD: 3.25},
			},
		},
	}
	writeJSONFile(t, dir, "run.json", rr)

	var out, errOut bytes.Buffer
	code := runCost([]string{"--path", dir}, &out, &errOut)
	if code != ExitPass {
		t.Fatalf("exit code = %d, want %d; stderr: %s", code, ExitPass, errOut.String())
	}
	if out.Len() == 0 {
		t.Error("expected text output")
	}
}

func TestCostJSONOutput(t *testing.T) {
	dir := t.TempDir()
	rr := model.RunReport{
		RunID:   "cost-json",
		Verdict: "PASS",
		Models: []model.ModelResult{
			{
				Ref:   model.ModelRef{Provider: "openai", Model: "gpt-4o", Agent: "recon"},
				Usage: model.Usage{InputTokens: 1000000, OutputTokens: 500000, CostUSD: 7.50},
			},
		},
	}
	writeJSONFile(t, dir, "run.json", rr)

	var out, errOut bytes.Buffer
	code := runCost([]string{"--path", dir, "--format", "json"}, &out, &errOut)
	if code != ExitPass {
		t.Fatalf("exit code = %d, want %d; stderr: %s", code, ExitPass, errOut.String())
	}

	var report costbreak.CostReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if len(report.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(report.Entries))
	}
}

func TestCostGroupBy(t *testing.T) {
	dir := t.TempDir()
	rr := model.RunReport{
		RunID:   "cost-group",
		Verdict: "PASS",
		Models: []model.ModelResult{
			{
				Ref:   model.ModelRef{Provider: "openai", Model: "gpt-4o", Agent: "recon"},
				Usage: model.Usage{CostUSD: 2.00},
			},
			{
				Ref:   model.ModelRef{Provider: "openai", Model: "gpt-4o", Agent: "probe"},
				Usage: model.Usage{CostUSD: 3.00},
			},
		},
	}
	writeJSONFile(t, dir, "run.json", rr)

	var out, errOut bytes.Buffer
	code := runCost([]string{"--path", dir, "--group-by", "checker"}, &out, &errOut)
	if code != ExitPass {
		t.Fatalf("exit code = %d, want %d; stderr: %s", code, ExitPass, errOut.String())
	}
	if out.Len() == 0 {
		t.Error("expected text output")
	}
}
