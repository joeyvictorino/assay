package costcompare

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type ModelCost struct {
	Model         string  `json:"model"`
	FindingCount  int     `json:"finding_count"`
	ValidCount    int     `json:"validated_count"`
	CostPerFind   float64 `json:"cost_per_finding"`
	CostPerValid  float64 `json:"cost_per_validated"`
	TotalTokens   int64   `json:"total_tokens"`
	Efficiency    float64 `json:"efficiency_score"`
}

type Comparison struct {
	Better string  `json:"better_value"`
	Worse  string  `json:"worse_value"`
	Metric string  `json:"metric"`
	Ratio  float64 `json:"ratio"`
}

type Report struct {
	Models      []ModelCost  `json:"models"`
	Comparisons []Comparison `json:"comparisons,omitempty"`
}

type bucket struct {
	total     int
	validated int
	tokens    int64
}

func Analyze(findings []model.Finding) *Report {
	buckets := make(map[string]*bucket)
	for _, f := range findings {
		m := f.Model.Model
		b, ok := buckets[m]
		if !ok {
			b = &bucket{}
			buckets[m] = b
		}
		b.total++
		if f.State == model.StateValidated {
			b.validated++
		}
		if f.MaxTokens > 0 {
			b.tokens += int64(f.MaxTokens)
		}
	}

	var models []ModelCost
	for m, b := range buckets {
		costPerFind := 0.0
		costPerValid := 0.0
		if b.total > 0 && b.tokens > 0 {
			costPerFind = round2(float64(b.tokens) / float64(b.total))
		}
		if b.validated > 0 && b.tokens > 0 {
			costPerValid = round2(float64(b.tokens) / float64(b.validated))
		}
		eff := efficiency(b)
		models = append(models, ModelCost{
			Model:        m,
			FindingCount: b.total,
			ValidCount:   b.validated,
			CostPerFind:  costPerFind,
			CostPerValid: costPerValid,
			TotalTokens:  b.tokens,
			Efficiency:   eff,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		if models[i].Efficiency != models[j].Efficiency {
			return models[i].Efficiency > models[j].Efficiency
		}
		return models[i].Model < models[j].Model
	})

	var comps []Comparison
	if len(models) >= 2 {
		best := models[0]
		worst := models[len(models)-1]
		if best.CostPerFind > 0 && worst.CostPerFind > 0 {
			comps = append(comps, Comparison{
				Better: best.Model,
				Worse:  worst.Model,
				Metric: "cost_per_finding",
				Ratio:  round2(worst.CostPerFind / best.CostPerFind),
			})
		}
		if best.CostPerValid > 0 && worst.CostPerValid > 0 {
			comps = append(comps, Comparison{
				Better: best.Model,
				Worse:  worst.Model,
				Metric: "cost_per_validated",
				Ratio:  round2(worst.CostPerValid / best.CostPerValid),
			})
		}
	}

	return &Report{Models: models, Comparisons: comps}
}

func efficiency(b *bucket) float64 {
	if b.total == 0 || b.tokens == 0 {
		return 0
	}
	validRatio := float64(b.validated) / float64(b.total)
	tokenEff := 1.0 / (1.0 + math.Log1p(float64(b.tokens)/float64(b.total)/1000.0))
	return round2(validRatio*70 + tokenEff*30)
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func FormatText(r *Report) string {
	var b strings.Builder
	b.WriteString("Model Cost Comparison\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No model data available.\n")
		return b.String()
	}

	for i, m := range r.Models {
		fmt.Fprintf(&b, "  [%d] %s\n", i+1, m.Model)
		fmt.Fprintf(&b, "      Findings: %d total, %d validated\n", m.FindingCount, m.ValidCount)
		fmt.Fprintf(&b, "      Tokens: %d  Cost/finding: %.0f  Cost/validated: %.0f\n",
			m.TotalTokens, m.CostPerFind, m.CostPerValid)
		fmt.Fprintf(&b, "      Efficiency: %.1f%%\n\n", m.Efficiency)
	}

	if len(r.Comparisons) > 0 {
		b.WriteString("  Best vs Worst\n")
		b.WriteString("  " + strings.Repeat("-", 50) + "\n")
		for _, c := range r.Comparisons {
			fmt.Fprintf(&b, "    %s: %s is %.1fx more efficient than %s\n",
				c.Metric, c.Better, c.Ratio, c.Worse)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r *Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
