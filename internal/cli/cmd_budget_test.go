package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/budget"
	"github.com/joeyvictorino/assay/internal/model"
)

func TestBudgetMissingPath(t *testing.T) {
	var out, errOut bytes.Buffer
	code := runBudget(nil, &out, &errOut)
	if code != ExitError {
		t.Errorf("exit code = %d, want %d", code, ExitError)
	}
}

func TestBudgetBadFormat(t *testing.T) {
	var out, errOut bytes.Buffer
	code := runBudget([]string{"--path", "/tmp", "--format", "xml"}, &out, &errOut)
	if code != ExitError {
		t.Errorf("exit code = %d, want %d", code, ExitError)
	}
}

func TestBudgetTextOutput(t *testing.T) {
	dir := t.TempDir()
	rr := model.RunReport{
		RunID:        "budget-cli-1",
		BudgetCapUSD: 10.0,
		SpentUSD:     3.25,
		Verdict:      "PASS",
		Models: []model.ModelResult{
			{
				Ref:   model.ModelRef{Provider: "openai", Model: "gpt-4o", Agent: "recon"},
				Usage: model.Usage{InputTokens: 500000, OutputTokens: 200000, CostUSD: 3.25},
			},
		},
	}
	writeJSONFile(t, dir, "run.json", rr)

	var out, errOut bytes.Buffer
	code := runBudget([]string{"--path", dir}, &out, &errOut)
	if code != ExitPass {
		t.Fatalf("exit code = %d, want %d; stderr: %s", code, ExitPass, errOut.String())
	}
	if !strings.Contains(out.String(), "UNDER_BUDGET") {
		t.Error("expected UNDER_BUDGET in output")
	}
}

func TestBudgetJSONOutput(t *testing.T) {
	dir := t.TempDir()
	rr := model.RunReport{
		RunID:        "budget-json-1",
		BudgetCapUSD: 5.0,
		SpentUSD:     7.50,
		Verdict:      "ERROR",
		Models: []model.ModelResult{
			{
				Ref:   model.ModelRef{Provider: "openai", Model: "gpt-4o", Agent: "recon"},
				Usage: model.Usage{InputTokens: 1000000, OutputTokens: 500000, CostUSD: 7.50},
			},
		},
	}
	writeJSONFile(t, dir, "run.json", rr)

	var out, errOut bytes.Buffer
	code := runBudget([]string{"--path", dir, "--format", "json"}, &out, &errOut)
	if code != ExitPass {
		t.Fatalf("exit code = %d, want %d; stderr: %s", code, ExitPass, errOut.String())
	}

	var report budget.BudgetReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if report.Status != budget.StatusOverBudget {
		t.Errorf("status = %s, want %s", report.Status, budget.StatusOverBudget)
	}
}

func TestBudgetWithLimit(t *testing.T) {
	dir := t.TempDir()
	rr := model.RunReport{
		RunID:        "budget-limit",
		BudgetCapUSD: 20.0,
		SpentUSD:     8.0,
		Verdict:      "PASS",
		Models: []model.ModelResult{
			{
				Ref:   model.ModelRef{Provider: "openai", Model: "gpt-4o", Agent: "recon"},
				Usage: model.Usage{CostUSD: 3.0},
			},
			{
				Ref:   model.ModelRef{Provider: "anthropic", Model: "claude-sonnet-4", Agent: "probe"},
				Usage: model.Usage{CostUSD: 5.0},
			},
		},
	}
	writeJSONFile(t, dir, "run.json", rr)

	var out, errOut bytes.Buffer
	code := runBudget([]string{"--path", dir, "--limit", "1"}, &out, &errOut)
	if code != ExitPass {
		t.Fatalf("exit code = %d, want %d; stderr: %s", code, ExitPass, errOut.String())
	}
	if out.Len() == 0 {
		t.Error("expected text output")
	}
}
