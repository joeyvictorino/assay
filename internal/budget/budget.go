package budget

import (
	"fmt"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/costbreak"
	"github.com/joeyvictorino/assay/internal/model"
)

type BudgetStatus string

const (
	StatusUnderBudget BudgetStatus = "UNDER_BUDGET"
	StatusAtLimit     BudgetStatus = "AT_LIMIT"
	StatusOverBudget  BudgetStatus = "OVER_BUDGET"
	StatusNoBudget    BudgetStatus = "NO_BUDGET"
)

type CheckerBudget struct {
	Checker      string  `json:"checker"`
	SpentUSD     float64 `json:"spent_usd"`
	SharePercent float64 `json:"share_percent"`
}

type ModelBudget struct {
	Model        string  `json:"model"`
	SpentUSD     float64 `json:"spent_usd"`
	SharePercent float64 `json:"share_percent"`
}

type BudgetReport struct {
	RunID        string          `json:"run_id"`
	BudgetCapUSD float64         `json:"budget_cap_usd"`
	SpentUSD     float64         `json:"spent_usd"`
	RemainingUSD float64         `json:"remaining_usd"`
	UsedPercent  float64         `json:"used_percent"`
	Status       BudgetStatus    `json:"status"`
	ByChecker    []CheckerBudget `json:"by_checker"`
	ByModel      []ModelBudget   `json:"by_model"`
	TotalTokens  int64           `json:"total_tokens"`
}

func Analyze(rr model.RunReport) BudgetReport {
	cr := costbreak.AnalyzeCost(rr)

	spent := rr.SpentUSD
	if spent == 0 {
		spent = cr.TotalCostUSD
	}

	cap := rr.BudgetCapUSD
	remaining := cap - spent
	if remaining < 0 {
		remaining = 0
	}

	var usedPct float64
	var status BudgetStatus
	switch {
	case cap <= 0:
		status = StatusNoBudget
	case spent >= cap:
		status = StatusOverBudget
		usedPct = 100
		if cap > 0 {
			usedPct = (spent / cap) * 100
		}
	case spent >= cap*0.9:
		status = StatusAtLimit
		usedPct = (spent / cap) * 100
	default:
		status = StatusUnderBudget
		usedPct = (spent / cap) * 100
	}

	var byChecker []CheckerBudget
	for checker, cost := range cr.ByChecker {
		share := 0.0
		if spent > 0 {
			share = (cost / spent) * 100
		}
		byChecker = append(byChecker, CheckerBudget{
			Checker:      checker,
			SpentUSD:     cost,
			SharePercent: share,
		})
	}
	sort.Slice(byChecker, func(i, j int) bool {
		return byChecker[i].SpentUSD > byChecker[j].SpentUSD
	})

	var byModel []ModelBudget
	for mdl, cost := range cr.ByModel {
		share := 0.0
		if spent > 0 {
			share = (cost / spent) * 100
		}
		byModel = append(byModel, ModelBudget{
			Model:        mdl,
			SpentUSD:     cost,
			SharePercent: share,
		})
	}
	sort.Slice(byModel, func(i, j int) bool {
		return byModel[i].SpentUSD > byModel[j].SpentUSD
	})

	return BudgetReport{
		RunID:        rr.RunID,
		BudgetCapUSD: cap,
		SpentUSD:     spent,
		RemainingUSD: remaining,
		UsedPercent:  usedPct,
		Status:       status,
		ByChecker:    byChecker,
		ByModel:      byModel,
		TotalTokens:  cr.TotalTokens,
	}
}

func FormatText(r BudgetReport, limit int) string {
	if len(r.ByChecker) == 0 && len(r.ByModel) == 0 {
		return "No budget data available.\n"
	}

	var b strings.Builder
	b.WriteString("Budget Report\n")
	b.WriteString("══════════════════════════════════════════════════════════════\n\n")

	b.WriteString(fmt.Sprintf("  Run:       %s\n", r.RunID))
	b.WriteString(fmt.Sprintf("  Status:    %s\n", r.Status))
	if r.BudgetCapUSD > 0 {
		b.WriteString(fmt.Sprintf("  Budget:    $%.4f\n", r.BudgetCapUSD))
		b.WriteString(fmt.Sprintf("  Spent:     $%.4f (%.1f%%)\n", r.SpentUSD, r.UsedPercent))
		b.WriteString(fmt.Sprintf("  Remaining: $%.4f\n", r.RemainingUSD))
	} else {
		b.WriteString(fmt.Sprintf("  Spent:     $%.4f (no cap set)\n", r.SpentUSD))
	}
	b.WriteString(fmt.Sprintf("  Tokens:    %d\n\n", r.TotalTokens))

	b.WriteString("  By Checker:\n")
	b.WriteString(fmt.Sprintf("  %-25s %10s %8s\n", "Checker", "Cost USD", "Share"))
	b.WriteString("  ─────────────────────────────────────────────\n")
	checkers := r.ByChecker
	if limit > 0 && limit < len(checkers) {
		checkers = checkers[:limit]
	}
	for _, c := range checkers {
		b.WriteString(fmt.Sprintf("  %-25s $%9.4f %7.1f%%\n", c.Checker, c.SpentUSD, c.SharePercent))
	}

	b.WriteString("\n  By Model:\n")
	b.WriteString(fmt.Sprintf("  %-25s %10s %8s\n", "Model", "Cost USD", "Share"))
	b.WriteString("  ─────────────────────────────────────────────\n")
	models := r.ByModel
	if limit > 0 && limit < len(models) {
		models = models[:limit]
	}
	for _, m := range models {
		b.WriteString(fmt.Sprintf("  %-25s $%9.4f %7.1f%%\n", m.Model, m.SpentUSD, m.SharePercent))
	}
	b.WriteByte('\n')

	return b.String()
}
