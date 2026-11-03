package tokeneff

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type ModelEfficiency struct {
	Agent            string  `json:"agent"`
	Model            string  `json:"model"`
	Provider         string  `json:"provider"`
	InputTokens      int64   `json:"input_tokens"`
	OutputTokens     int64   `json:"output_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	CostUSD          float64 `json:"cost_usd"`
	LatencyMS        int64   `json:"latency_ms"`
	FindingsTotal    int     `json:"findings_total"`
	FindingsValid    int     `json:"findings_validated"`
	FindingsRefuted  int     `json:"findings_refuted"`
	Refusals         int     `json:"refusals"`
	TokensPerFinding float64 `json:"tokens_per_finding"`
	CostPerFinding   float64 `json:"cost_per_finding"`
	ValidationRate   float64 `json:"validation_rate"`
	EfficiencyScore  float64 `json:"efficiency_score"`
	Grade            string  `json:"grade"`
}

type Report struct {
	RunID       string            `json:"run_id"`
	Models      []ModelEfficiency `json:"models"`
	BestOverall string            `json:"best_overall"`
	BestValue   string            `json:"best_value"`
	Summary     Summary           `json:"summary"`
}

type Summary struct {
	TotalModels     int     `json:"total_models"`
	TotalTokens     int64   `json:"total_tokens"`
	TotalCostUSD    float64 `json:"total_cost_usd"`
	TotalFindings   int     `json:"total_findings"`
	AvgEfficiency   float64 `json:"avg_efficiency"`
	BudgetCapUSD    float64 `json:"budget_cap_usd"`
	BudgetUsedPct   float64 `json:"budget_used_pct"`
}

func Analyze(rr model.RunReport) Report {
	var models []ModelEfficiency

	for _, mr := range rr.Models {
		total := mr.Usage.InputTokens + mr.Usage.OutputTokens
		findingsTotal := countFindings(mr.Findings)
		validated := mr.Findings[model.StateValidated]
		refuted := mr.Findings[model.StateRefuted]

		tokPerFinding := 0.0
		if findingsTotal > 0 {
			tokPerFinding = float64(total) / float64(findingsTotal)
		}

		costPerFinding := 0.0
		if findingsTotal > 0 {
			costPerFinding = mr.Usage.CostUSD / float64(findingsTotal)
		}

		validRate := 0.0
		if findingsTotal > 0 {
			validRate = float64(validated) / float64(findingsTotal)
		}

		eff := computeEfficiency(total, findingsTotal, validated, refuted, mr.Refusals, mr.Usage.CostUSD)

		models = append(models, ModelEfficiency{
			Agent:            mr.Ref.Agent,
			Model:            mr.Ref.Model,
			Provider:         mr.Ref.Provider,
			InputTokens:      mr.Usage.InputTokens,
			OutputTokens:     mr.Usage.OutputTokens,
			TotalTokens:      total,
			CostUSD:          mr.Usage.CostUSD,
			LatencyMS:        mr.Usage.LatencyMS,
			FindingsTotal:    findingsTotal,
			FindingsValid:    validated,
			FindingsRefuted:  refuted,
			Refusals:         mr.Refusals,
			TokensPerFinding: tokPerFinding,
			CostPerFinding:   costPerFinding,
			ValidationRate:   validRate,
			EfficiencyScore:  eff,
			Grade:            gradeScore(eff),
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].EfficiencyScore > models[j].EfficiencyScore
	})

	bestOverall := ""
	bestValue := ""
	if len(models) > 0 {
		bestOverall = models[0].Model
		bestVal := models[0]
		for _, m := range models[1:] {
			if m.CostPerFinding > 0 && (bestVal.CostPerFinding == 0 || m.CostPerFinding < bestVal.CostPerFinding) {
				bestVal = m
			}
		}
		bestValue = bestVal.Model
	}

	return Report{
		RunID:       rr.RunID,
		Models:      models,
		BestOverall: bestOverall,
		BestValue:   bestValue,
		Summary:     buildSummary(rr, models),
	}
}

func countFindings(m map[model.FindingState]int) int {
	total := 0
	for _, v := range m {
		total += v
	}
	return total
}

func computeEfficiency(totalTokens int64, findings, validated, refuted, refusals int, costUSD float64) float64 {
	if findings == 0 {
		if refusals > 0 {
			return 0
		}
		return 10
	}

	validRate := float64(validated) / float64(findings)
	refutedPenalty := float64(refuted) / float64(findings)
	refusalPenalty := 0.0
	if refusals > 0 {
		refusalPenalty = math.Min(float64(refusals)*0.1, 0.5)
	}

	tokEff := 1.0
	if totalTokens > 0 {
		tokEff = math.Min(1.0, 50000.0/float64(totalTokens)*float64(findings))
	}

	costEff := 1.0
	if costUSD > 0 {
		costEff = math.Min(1.0, 0.50/costUSD*float64(findings))
	}

	score := (validRate*40 + tokEff*25 + costEff*25 + (1-refutedPenalty)*10) * (1 - refusalPenalty)

	return math.Round(score*100) / 100
}

func gradeScore(score float64) string {
	switch {
	case score >= 80:
		return "A"
	case score >= 65:
		return "B"
	case score >= 50:
		return "C"
	case score >= 35:
		return "D"
	default:
		return "F"
	}
}

func buildSummary(rr model.RunReport, models []ModelEfficiency) Summary {
	s := Summary{
		TotalModels:  len(models),
		BudgetCapUSD: rr.BudgetCapUSD,
		TotalCostUSD: rr.SpentUSD,
	}

	effSum := 0.0
	for _, m := range models {
		s.TotalTokens += m.TotalTokens
		s.TotalFindings += m.FindingsTotal
		effSum += m.EfficiencyScore
	}

	if len(models) > 0 {
		s.AvgEfficiency = math.Round(effSum/float64(len(models))*100) / 100
	}
	if s.BudgetCapUSD > 0 {
		s.BudgetUsedPct = math.Round(s.TotalCostUSD/s.BudgetCapUSD*10000) / 100
	}

	return s
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Token Efficiency Analysis\n")
	b.WriteString("══════════════════════════════════════════════════════════════\n\n")

	b.WriteString(fmt.Sprintf("  Run:           %s\n", r.RunID))
	b.WriteString(fmt.Sprintf("  Models:        %d\n", r.Summary.TotalModels))
	b.WriteString(fmt.Sprintf("  Total tokens:  %d\n", r.Summary.TotalTokens))
	b.WriteString(fmt.Sprintf("  Total cost:    $%.4f\n", r.Summary.TotalCostUSD))
	b.WriteString(fmt.Sprintf("  Total findings:%d\n", r.Summary.TotalFindings))
	b.WriteString(fmt.Sprintf("  Avg efficiency:%.1f\n", r.Summary.AvgEfficiency))
	if r.Summary.BudgetCapUSD > 0 {
		b.WriteString(fmt.Sprintf("  Budget used:   %.1f%%\n", r.Summary.BudgetUsedPct))
	}
	b.WriteString(fmt.Sprintf("  Best overall:  %s\n", r.BestOverall))
	b.WriteString(fmt.Sprintf("  Best value:    %s\n\n", r.BestValue))

	if len(r.Models) > 0 {
		b.WriteString(fmt.Sprintf("  %-15s %-20s %8s %8s %10s %8s %5s\n",
			"Agent", "Model", "Tokens", "Findings", "Tok/Find", "Score", "Grade"))
		b.WriteString("  ──────────────────────────────────────────────────────────────────────────────\n")
		for _, m := range r.Models {
			tokPerF := "-"
			if m.TokensPerFinding > 0 {
				tokPerF = fmt.Sprintf("%.0f", m.TokensPerFinding)
			}
			b.WriteString(fmt.Sprintf("  %-15s %-20s %8d %8d %10s %8.1f %5s\n",
				m.Agent, m.Model, m.TotalTokens, m.FindingsTotal, tokPerF, m.EfficiencyScore, m.Grade))
		}
		b.WriteByte('\n')
	}

	return b.String()
}
