package budget

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestAnalyzeUnderBudget(t *testing.T) {
	rr := model.RunReport{
		RunID:        "budget-test-1",
		BudgetCapUSD: 10.0,
		SpentUSD:     3.25,
		Models: []model.ModelResult{
			{
				Ref:   model.ModelRef{Provider: "openai", Model: "gpt-4o", Agent: "recon"},
				Usage: model.Usage{InputTokens: 500000, OutputTokens: 200000, CostUSD: 3.25},
			},
		},
	}

	r := Analyze(rr)
	if r.Status != StatusUnderBudget {
		t.Errorf("status = %s, want %s", r.Status, StatusUnderBudget)
	}
	if r.RemainingUSD != 6.75 {
		t.Errorf("remaining = %f, want 6.75", r.RemainingUSD)
	}
	if r.UsedPercent < 32 || r.UsedPercent > 33 {
		t.Errorf("used percent = %f, want ~32.5", r.UsedPercent)
	}
}

func TestAnalyzeOverBudget(t *testing.T) {
	rr := model.RunReport{
		RunID:        "budget-test-2",
		BudgetCapUSD: 5.0,
		SpentUSD:     7.50,
		Models: []model.ModelResult{
			{
				Ref:   model.ModelRef{Provider: "openai", Model: "gpt-4o", Agent: "recon"},
				Usage: model.Usage{InputTokens: 1000000, OutputTokens: 500000, CostUSD: 7.50},
			},
		},
	}

	r := Analyze(rr)
	if r.Status != StatusOverBudget {
		t.Errorf("status = %s, want %s", r.Status, StatusOverBudget)
	}
	if r.RemainingUSD != 0 {
		t.Errorf("remaining = %f, want 0", r.RemainingUSD)
	}
}

func TestAnalyzeAtLimit(t *testing.T) {
	rr := model.RunReport{
		RunID:        "budget-test-3",
		BudgetCapUSD: 10.0,
		SpentUSD:     9.50,
		Models: []model.ModelResult{
			{
				Ref:   model.ModelRef{Provider: "openai", Model: "gpt-4o", Agent: "recon"},
				Usage: model.Usage{InputTokens: 500000, OutputTokens: 200000, CostUSD: 9.50},
			},
		},
	}

	r := Analyze(rr)
	if r.Status != StatusAtLimit {
		t.Errorf("status = %s, want %s", r.Status, StatusAtLimit)
	}
}

func TestAnalyzeNoBudget(t *testing.T) {
	rr := model.RunReport{
		RunID: "budget-test-4",
		Models: []model.ModelResult{
			{
				Ref:   model.ModelRef{Provider: "openai", Model: "gpt-4o", Agent: "recon"},
				Usage: model.Usage{InputTokens: 100000, OutputTokens: 50000, CostUSD: 0.75},
			},
		},
	}

	r := Analyze(rr)
	if r.Status != StatusNoBudget {
		t.Errorf("status = %s, want %s", r.Status, StatusNoBudget)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	rr := model.RunReport{
		RunID:        "budget-test-5",
		BudgetCapUSD: 20.0,
		SpentUSD:     8.0,
		Models: []model.ModelResult{
			{
				Ref:   model.ModelRef{Provider: "openai", Model: "gpt-4o", Agent: "recon"},
				Usage: model.Usage{InputTokens: 500000, OutputTokens: 200000, CostUSD: 3.25},
			},
			{
				Ref:   model.ModelRef{Provider: "anthropic", Model: "claude-sonnet-4", Agent: "probe"},
				Usage: model.Usage{InputTokens: 800000, OutputTokens: 300000, CostUSD: 4.75},
			},
		},
	}

	r := Analyze(rr)
	if len(r.ByChecker) != 2 {
		t.Errorf("by_checker count = %d, want 2", len(r.ByChecker))
	}
	if len(r.ByModel) != 2 {
		t.Errorf("by_model count = %d, want 2", len(r.ByModel))
	}
	if r.ByChecker[0].SpentUSD < r.ByChecker[1].SpentUSD {
		t.Error("by_checker should be sorted by cost descending")
	}
}

func TestAnalyzeUsesEstimatedCostWhenSpentIsZero(t *testing.T) {
	rr := model.RunReport{
		RunID:        "budget-est",
		BudgetCapUSD: 10.0,
		Models: []model.ModelResult{
			{
				Ref:   model.ModelRef{Provider: "openai", Model: "gpt-4o", Agent: "recon"},
				Usage: model.Usage{InputTokens: 1000000, OutputTokens: 500000},
			},
		},
	}

	r := Analyze(rr)
	if r.SpentUSD <= 0 {
		t.Error("expected estimated cost > 0 when SpentUSD is zero")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := BudgetReport{}
	text := FormatText(r, 0)
	if !strings.Contains(text, "No budget data") {
		t.Error("expected empty message")
	}
}

func TestFormatTextWithLimit(t *testing.T) {
	r := BudgetReport{
		RunID:        "fmt-test",
		BudgetCapUSD: 10.0,
		SpentUSD:     5.0,
		RemainingUSD: 5.0,
		UsedPercent:  50.0,
		Status:       StatusUnderBudget,
		ByChecker: []CheckerBudget{
			{Checker: "recon", SpentUSD: 3.0, SharePercent: 60},
			{Checker: "probe", SpentUSD: 2.0, SharePercent: 40},
		},
		ByModel: []ModelBudget{
			{Model: "gpt-4o", SpentUSD: 3.0, SharePercent: 60},
			{Model: "claude-sonnet-4", SpentUSD: 2.0, SharePercent: 40},
		},
		TotalTokens: 1500000,
	}

	text := FormatText(r, 1)
	if !strings.Contains(text, "recon") {
		t.Error("expected first checker in output")
	}
	if strings.Contains(text, "probe") {
		t.Error("limit=1 should hide second checker")
	}
}
