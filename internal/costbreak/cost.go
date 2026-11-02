package costbreak

import (
	"fmt"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type CostEntry struct {
	Checker          string  `json:"checker"`
	Model            string  `json:"model"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	EstCostUSD       float64 `json:"est_cost_usd"`
}

type CostReport struct {
	Entries      []CostEntry        `json:"entries"`
	TotalTokens  int64              `json:"total_tokens"`
	TotalCostUSD float64            `json:"total_cost_usd"`
	ByChecker    map[string]float64 `json:"by_checker"`
	ByModel      map[string]float64 `json:"by_model"`
}

type modelPricing struct {
	inputPerMTok  float64
	outputPerMTok float64
}

var pricing = map[string]modelPricing{
	"claude-3-5-sonnet":     {3.00, 15.00},
	"claude-3-5-haiku":      {0.80, 4.00},
	"claude-3-opus":         {15.00, 75.00},
	"claude-sonnet-4":       {3.00, 15.00},
	"claude-opus-4":         {15.00, 75.00},
	"gpt-4o":                {2.50, 10.00},
	"gpt-4o-mini":           {0.15, 0.60},
	"gpt-4-turbo":           {10.00, 30.00},
	"gpt-4":                 {30.00, 60.00},
	"gpt-3.5-turbo":         {0.50, 1.50},
	"gemini-1.5-pro":        {3.50, 10.50},
	"gemini-1.5-flash":      {0.075, 0.30},
	"gemini-2.0-flash":      {0.10, 0.40},
}

func lookupPricing(modelName string) modelPricing {
	lower := strings.ToLower(modelName)
	for prefix, p := range pricing {
		if strings.HasPrefix(lower, prefix) || strings.Contains(lower, prefix) {
			return p
		}
	}
	return modelPricing{5.00, 15.00}
}

func AnalyzeCost(rr model.RunReport) CostReport {
	var entries []CostEntry
	byChecker := map[string]float64{}
	byModel := map[string]float64{}

	for _, mr := range rr.Models {
		checker := mr.Ref.Agent
		mdl := mr.Ref.Model
		u := mr.Usage

		cost := u.CostUSD
		if cost == 0 {
			p := lookupPricing(mdl)
			cost = float64(u.InputTokens)/1e6*p.inputPerMTok +
				float64(u.OutputTokens)/1e6*p.outputPerMTok
		}

		e := CostEntry{
			Checker:          checker,
			Model:            mdl,
			PromptTokens:     u.InputTokens,
			CompletionTokens: u.OutputTokens,
			EstCostUSD:       cost,
		}
		entries = append(entries, e)
		byChecker[checker] += cost
		byModel[mdl] += cost
	}

	var totalTokens int64
	var totalCost float64
	for _, e := range entries {
		totalTokens += e.PromptTokens + e.CompletionTokens
		totalCost += e.EstCostUSD
	}

	return CostReport{
		Entries:      entries,
		TotalTokens:  totalTokens,
		TotalCostUSD: totalCost,
		ByChecker:    byChecker,
		ByModel:      byModel,
	}
}

func FormatText(r CostReport, groupBy string) string {
	if len(r.Entries) == 0 {
		return "No cost data available.\n"
	}

	s := "Cost Breakdown\n"
	s += "══════════════════════════════════════════════════════════════\n\n"

	switch groupBy {
	case "checker":
		s += formatGrouped("Checker", r.ByChecker)
	case "model":
		s += formatGrouped("Model", r.ByModel)
	default:
		s += fmt.Sprintf("  %-20s %-25s %12s %12s %10s\n",
			"Checker", "Model", "In Tokens", "Out Tokens", "Cost USD")
		s += "  ────────────────────────────────────────────────────────────────────────────\n"
		for _, e := range r.Entries {
			s += fmt.Sprintf("  %-20s %-25s %12d %12d $%9.4f\n",
				e.Checker, e.Model, e.PromptTokens, e.CompletionTokens, e.EstCostUSD)
		}
	}

	s += fmt.Sprintf("\n  Total: %d tokens, $%.4f\n", r.TotalTokens, r.TotalCostUSD)
	return s
}

func formatGrouped(label string, m map[string]float64) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	s := fmt.Sprintf("  %-30s %10s\n", label, "Cost USD")
	s += "  ──────────────────────────────────────────\n"
	for _, k := range keys {
		s += fmt.Sprintf("  %-30s $%9.4f\n", k, m[k])
	}
	return s
}
