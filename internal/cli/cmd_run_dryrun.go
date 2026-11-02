package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/joeyvictorino/assay/internal/config"
	"github.com/joeyvictorino/assay/internal/provider"
	"github.com/joeyvictorino/assay/internal/validate"
)

// DryRunEstimate holds the cost projection for a planned run.
type DryRunEstimate struct {
	Models        int
	Labs          int
	Checkers      int
	Tasks         int
	EstInputTok   int64
	EstOutputTok  int64
	EstCostMinUSD float64
	EstCostMaxUSD float64
	BudgetMaxUSD  float64
	PerModel      []ModelEstimate
}

// ModelEstimate is the per-model breakdown.
type ModelEstimate struct {
	Model        string
	Provider     string
	InputPerMTok float64
	OutputPerMTok float64
	EstInputTok  int64
	EstOutputTok int64
	EstCostUSD   float64
}

// EstimateAvgInputPerTurn is the assumed average input tokens per agent turn.
// This accounts for system prompt, tool definitions, conversation history and
// the lab context. Based on observed runs; the real number varies with lab
// complexity and conversation depth.
const EstimateAvgInputPerTurn = 3000

// EstimateDryRun projects cost from the config without calling any provider.
func EstimateDryRun(cfg *config.Config, labNames []string) DryRunEstimate {
	nModels := len(cfg.Models)
	nLabs := len(labNames)
	nCheckers := len(validate.Checkers())
	maxTurns := cfg.Agent.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 8
	}
	maxTokens := cfg.Agent.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 2048
	}

	// Each model runs: probe (maxTurns turns) + score (3 turns) per lab.
	// Each turn: ~EstimateAvgInputPerTurn input, ~maxTokens/2 output (models
	// rarely use the full budget).
	probeTurns := maxTurns
	scoreTurns := 3
	totalTurnsPerModelLab := probeTurns + scoreTurns
	avgOutputPerTurn := int64(maxTokens) / 2

	est := DryRunEstimate{
		Models:   nModels,
		Labs:     nLabs,
		Checkers: nCheckers,
		Tasks:    nModels * nLabs,
		BudgetMaxUSD: cfg.Budgets.PerRun.MaxUSD,
	}

	for _, m := range cfg.Models {
		cost, err := provider.Costs(cfg.CostPerMTok).Lookup(m.Model)
		if err != nil {
			continue
		}
		inputTok := int64(nLabs) * int64(totalTurnsPerModelLab) * EstimateAvgInputPerTurn
		outputTok := int64(nLabs) * int64(totalTurnsPerModelLab) * avgOutputPerTurn

		costUSD := float64(inputTok)*cost.InputUSD/1_000_000 +
			float64(outputTok)*cost.OutputUSD/1_000_000

		me := ModelEstimate{
			Model:         m.Model,
			Provider:      m.Provider,
			InputPerMTok:  cost.InputUSD,
			OutputPerMTok: cost.OutputUSD,
			EstInputTok:   inputTok,
			EstOutputTok:  outputTok,
			EstCostUSD:    costUSD,
		}
		est.PerModel = append(est.PerModel, me)
		est.EstInputTok += inputTok
		est.EstOutputTok += outputTok
	}

	// Cost range: 40% to 120% of point estimate to account for variance in
	// conversation depth, early stops, and cache hits.
	pointCost := 0.0
	for _, m := range est.PerModel {
		pointCost += m.EstCostUSD
	}
	est.EstCostMinUSD = pointCost * 0.4
	est.EstCostMaxUSD = pointCost * 1.2

	return est
}

// PrintDryRun writes the estimate to w in a human-readable table.
func PrintDryRun(w io.Writer, est DryRunEstimate) {
	fmt.Fprintln(w, "DRY RUN — cost estimate (no API calls made)")
	fmt.Fprintln(w, strings.Repeat("─", 56))
	fmt.Fprintf(w, "Models:       %d\n", est.Models)
	fmt.Fprintf(w, "Labs:         %d\n", est.Labs)
	fmt.Fprintf(w, "Checkers:     %d (deterministic, no token cost)\n", est.Checkers)
	fmt.Fprintf(w, "Tasks:        %d (model × lab)\n", est.Tasks)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Est. input:   %s tokens\n", fmtTok(est.EstInputTok))
	fmt.Fprintf(w, "Est. output:  %s tokens\n", fmtTok(est.EstOutputTok))
	fmt.Fprintln(w)

	if len(est.PerModel) > 0 {
		fmt.Fprintln(w, "Per-model breakdown:")
		fmt.Fprintf(w, "  %-30s %10s %10s %10s\n", "MODEL", "INPUT", "OUTPUT", "EST. USD")
		fmt.Fprintf(w, "  %-30s %10s %10s %10s\n",
			strings.Repeat("─", 30), strings.Repeat("─", 10),
			strings.Repeat("─", 10), strings.Repeat("─", 10))
		for _, m := range est.PerModel {
			fmt.Fprintf(w, "  %-30s %10s %10s %10s\n",
				m.Model,
				fmtTok(m.EstInputTok),
				fmtTok(m.EstOutputTok),
				fmtUSD(m.EstCostUSD))
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintf(w, "Estimated cost:   $%.2f – $%.2f\n", est.EstCostMinUSD, est.EstCostMaxUSD)
	fmt.Fprintf(w, "Budget cap:       $%.2f (per_run.max_usd)\n", est.BudgetMaxUSD)
	fmt.Fprintln(w, strings.Repeat("─", 56))
	fmt.Fprintln(w, "Estimates assume ~3K input tokens/turn and 50% output utilization.")
	fmt.Fprintln(w, "Actual cost depends on conversation depth, cache hits, and early stops.")
}

func fmtTok(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

func fmtUSD(v float64) string {
	if v < 0.01 {
		return "$0.00"
	}
	return fmt.Sprintf("$%.2f", v)
}
