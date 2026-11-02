package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/config"
	"github.com/joeyvictorino/assay/internal/provider"
)

func TestEstimateDryRunBasic(t *testing.T) {
	cfg := &config.Config{
		Models: []config.ModelConfig{
			{Provider: "anthropic", Model: "claude-sonnet-5-5"},
			{Provider: "anthropic", Model: "claude-haiku-4-5"},
		},
		CostPerMTok: map[string]provider.Cost{
			"claude-sonnet-5-5": {InputUSD: 2, OutputUSD: 10},
			"claude-haiku-4-5":  {InputUSD: 1, OutputUSD: 5},
		},
		Agent:   config.AgentConfig{MaxTurns: 8, MaxTokens: 2048},
		Budgets: config.Budgets{PerRun: config.Budget{MaxUSD: 15}},
	}
	labs := []string{"synthetic-ops", "juice-shop", "dvwa"}

	est := EstimateDryRun(cfg, labs)

	if est.Models != 2 {
		t.Errorf("Models = %d, want 2", est.Models)
	}
	if est.Labs != 3 {
		t.Errorf("Labs = %d, want 3", est.Labs)
	}
	if est.Tasks != 6 {
		t.Errorf("Tasks = %d, want 6", est.Tasks)
	}
	if est.Checkers <= 0 {
		t.Error("Checkers should be positive")
	}
	if est.EstInputTok <= 0 {
		t.Error("EstInputTok should be positive")
	}
	if est.EstOutputTok <= 0 {
		t.Error("EstOutputTok should be positive")
	}
	if est.EstCostMinUSD <= 0 || est.EstCostMaxUSD <= 0 {
		t.Error("cost estimates should be positive")
	}
	if est.EstCostMinUSD >= est.EstCostMaxUSD {
		t.Error("min cost should be less than max cost")
	}
	if est.BudgetMaxUSD != 15 {
		t.Errorf("BudgetMaxUSD = %f, want 15", est.BudgetMaxUSD)
	}
	if len(est.PerModel) != 2 {
		t.Fatalf("PerModel = %d, want 2", len(est.PerModel))
	}

	// Sonnet should cost more than Haiku with the same token volume.
	if est.PerModel[0].EstCostUSD <= est.PerModel[1].EstCostUSD {
		t.Errorf("sonnet (%f) should cost more than haiku (%f)",
			est.PerModel[0].EstCostUSD, est.PerModel[1].EstCostUSD)
	}
}

func TestEstimateDryRunZeroCostModel(t *testing.T) {
	cfg := &config.Config{
		Models: []config.ModelConfig{
			{Provider: "local", Model: "qwen2.5-3b"},
		},
		CostPerMTok: map[string]provider.Cost{
			"qwen2.5-3b": {InputUSD: 0, OutputUSD: 0},
		},
		Agent:   config.AgentConfig{MaxTurns: 8, MaxTokens: 2048},
		Budgets: config.Budgets{PerRun: config.Budget{MaxUSD: 15}},
	}
	est := EstimateDryRun(cfg, []string{"synthetic-ops"})

	if est.EstCostMinUSD != 0 {
		t.Errorf("zero-cost model should have $0 estimate, got %f", est.EstCostMinUSD)
	}
	if est.EstInputTok <= 0 {
		t.Error("token estimate should still be positive even with zero cost")
	}
}

func TestEstimateDryRunMissingCostEntry(t *testing.T) {
	cfg := &config.Config{
		Models: []config.ModelConfig{
			{Provider: "anthropic", Model: "claude-sonnet-5-5"},
			{Provider: "local", Model: "unknown-model"},
		},
		CostPerMTok: map[string]provider.Cost{
			"claude-sonnet-5-5": {InputUSD: 2, OutputUSD: 10},
		},
		Agent:   config.AgentConfig{MaxTurns: 8, MaxTokens: 2048},
		Budgets: config.Budgets{PerRun: config.Budget{MaxUSD: 15}},
	}
	est := EstimateDryRun(cfg, []string{"synthetic-ops"})

	if len(est.PerModel) != 1 {
		t.Errorf("should skip model without cost entry, got %d", len(est.PerModel))
	}
}

func TestPrintDryRun(t *testing.T) {
	cfg := &config.Config{
		Models: []config.ModelConfig{
			{Provider: "anthropic", Model: "claude-sonnet-5-5"},
		},
		CostPerMTok: map[string]provider.Cost{
			"claude-sonnet-5-5": {InputUSD: 2, OutputUSD: 10},
		},
		Agent:   config.AgentConfig{MaxTurns: 8, MaxTokens: 2048},
		Budgets: config.Budgets{PerRun: config.Budget{MaxUSD: 15}},
	}
	est := EstimateDryRun(cfg, []string{"synthetic-ops"})

	var buf bytes.Buffer
	PrintDryRun(&buf, est)
	out := buf.String()

	for _, want := range []string{
		"DRY RUN",
		"Models:",
		"Labs:",
		"Checkers:",
		"claude-sonnet-5-5",
		"Estimated cost:",
		"Budget cap:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestFmtTok(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{500, "500"},
		{1500, "1.5K"},
		{33000, "33.0K"},
		{1_500_000, "1.5M"},
	}
	for _, tc := range cases {
		if got := fmtTok(tc.in); got != tc.want {
			t.Errorf("fmtTok(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
