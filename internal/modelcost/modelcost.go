package modelcost

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type ModelCost struct {
	Model          string  `json:"model"`
	Provider       string  `json:"provider"`
	FindingCount   int     `json:"finding_count"`
	EstTokens      int     `json:"estimated_tokens"`
	EstCost        float64 `json:"estimated_cost_usd"`
	CostPerFinding float64 `json:"cost_per_finding"`
}

type CostSummary struct {
	TotalModels       int     `json:"total_models"`
	TotalFindings     int     `json:"total_findings"`
	TotalEstCost      float64 `json:"total_estimated_cost_usd"`
	AvgCostPerFinding float64 `json:"avg_cost_per_finding"`
	BestValue         string  `json:"best_value_model"`
	WorstValue        string  `json:"worst_value_model"`
}

type CostReport struct {
	Models  []ModelCost `json:"models"`
	Summary CostSummary `json:"summary"`
}

var pricePer1KTokens = map[string]float64{
	"gpt-4":          0.06,
	"gpt-4o":         0.005,
	"gpt-4o-mini":    0.00015,
	"gpt-4-turbo":    0.02,
	"gpt-3.5-turbo":  0.0015,
	"claude-3-opus":  0.075,
	"claude-3-sonnet": 0.015,
	"claude-3-haiku": 0.00125,
	"claude-3.5-sonnet": 0.015,
	"claude-3.5-haiku": 0.004,
	"gemini-pro":     0.00125,
	"gemini-1.5-pro": 0.00125,
	"gemini-1.5-flash": 0.000075,
}

func estimateTokens(f model.Finding) int {
	tokens := len(f.Summary) / 4
	tokens += len(string(f.Class)) / 4
	tokens += len(f.Evidence) * 200
	if tokens < 500 {
		tokens = 500
	}
	return tokens
}

func priceForModel(name string) float64 {
	lower := strings.ToLower(name)
	bestKey := ""
	bestPrice := 0.01
	for k, v := range pricePer1KTokens {
		kl := strings.ToLower(k)
		if strings.Contains(lower, kl) && len(kl) > len(bestKey) {
			bestKey = kl
			bestPrice = v
		}
	}
	return bestPrice
}

func Analyze(findings []model.Finding) CostReport {
	type bucket struct {
		provider string
		count    int
		tokens   int
	}
	byModel := make(map[string]*bucket)

	for _, f := range findings {
		mdl := f.Model.Model
		if mdl == "" {
			mdl = "unknown"
		}
		b, ok := byModel[mdl]
		if !ok {
			b = &bucket{provider: f.Model.Provider}
			byModel[mdl] = b
		}
		b.count++
		b.tokens += estimateTokens(f)
	}

	var models []ModelCost
	totalCost := 0.0
	totalFindings := 0

	for mdl, b := range byModel {
		price := priceForModel(mdl)
		cost := float64(b.tokens) / 1000.0 * price
		cpf := 0.0
		if b.count > 0 {
			cpf = cost / float64(b.count)
		}
		models = append(models, ModelCost{
			Model:          mdl,
			Provider:       b.provider,
			FindingCount:   b.count,
			EstTokens:      b.tokens,
			EstCost:        math.Round(cost*10000) / 10000,
			CostPerFinding: math.Round(cpf*10000) / 10000,
		})
		totalCost += cost
		totalFindings += b.count
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].CostPerFinding < models[j].CostPerFinding
	})

	best, worst := "", ""
	if len(models) > 0 {
		best = models[0].Model
		worst = models[len(models)-1].Model
	}

	avgCPF := 0.0
	if totalFindings > 0 {
		avgCPF = totalCost / float64(totalFindings)
	}

	return CostReport{
		Models: models,
		Summary: CostSummary{
			TotalModels:       len(models),
			TotalFindings:     totalFindings,
			TotalEstCost:      math.Round(totalCost*10000) / 10000,
			AvgCostPerFinding: math.Round(avgCPF*10000) / 10000,
			BestValue:         best,
			WorstValue:        worst,
		},
	}
}

func FormatText(r CostReport) string {
	var sb strings.Builder
	sb.WriteString("Model Cost Analysis\n")
	sb.WriteString(strings.Repeat("=", 50) + "\n\n")
	fmt.Fprintf(&sb, "  Models:             %d\n", r.Summary.TotalModels)
	fmt.Fprintf(&sb, "  Total findings:     %d\n", r.Summary.TotalFindings)
	fmt.Fprintf(&sb, "  Estimated cost:     $%.4f\n", r.Summary.TotalEstCost)
	fmt.Fprintf(&sb, "  Avg cost/finding:   $%.4f\n", r.Summary.AvgCostPerFinding)
	fmt.Fprintf(&sb, "  Best value:         %s\n", r.Summary.BestValue)
	fmt.Fprintf(&sb, "  Worst value:        %s\n\n", r.Summary.WorstValue)

	if len(r.Models) > 0 {
		sb.WriteString("Per-Model Breakdown:\n")
		for i, m := range r.Models {
			fmt.Fprintf(&sb, "  %d. %s (%s)\n", i+1, m.Model, m.Provider)
			fmt.Fprintf(&sb, "     Findings: %d  Tokens: ~%d  Cost: $%.4f  Per-finding: $%.4f\n\n",
				m.FindingCount, m.EstTokens, m.EstCost, m.CostPerFinding)
		}
	}

	return sb.String()
}

func FormatJSON(r CostReport) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
