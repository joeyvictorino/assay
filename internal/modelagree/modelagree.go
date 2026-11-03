package modelagree

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type PairAgreement struct {
	ModelA    string  `json:"model_a"`
	ModelB    string  `json:"model_b"`
	Shared    int     `json:"shared_prompts"`
	Agreement float64 `json:"agreement_rate"`
	SevCorr   float64 `json:"severity_correlation"`
}

type ModelSummary struct {
	Model       string  `json:"model"`
	Prompts     int     `json:"prompt_count"`
	AvgAgreement float64 `json:"avg_agreement"`
}

type Report struct {
	Pairs     []PairAgreement `json:"pairs"`
	Summaries []ModelSummary  `json:"summaries"`
	Overall   float64         `json:"overall_agreement"`
}

type promptModel struct {
	state    model.FindingState
	severity model.Severity
}

func Analyze(findings []model.Finding) *Report {
	index := make(map[string]map[string]promptModel)
	for _, f := range findings {
		h := f.PromptHash
		if h == "" {
			continue
		}
		m := f.Model.Model
		if _, ok := index[h]; !ok {
			index[h] = make(map[string]promptModel)
		}
		index[h][m] = promptModel{state: f.State, severity: f.Severity}
	}

	modelSet := make(map[string]bool)
	for _, f := range findings {
		modelSet[f.Model.Model] = true
	}
	models := sortedKeys(modelSet)

	var pairs []PairAgreement
	for i := 0; i < len(models); i++ {
		for j := i + 1; j < len(models); j++ {
			shared := 0
			agree := 0
			var sevsA, sevsB []float64
			for _, pm := range index {
				a, okA := pm[models[i]]
				b, okB := pm[models[j]]
				if !okA || !okB {
					continue
				}
				shared++
				if stateCategory(a.state) == stateCategory(b.state) {
					agree++
				}
				sevsA = append(sevsA, sevNum(a.severity))
				sevsB = append(sevsB, sevNum(b.severity))
			}
			if shared == 0 {
				continue
			}
			pairs = append(pairs, PairAgreement{
				ModelA:    models[i],
				ModelB:    models[j],
				Shared:    shared,
				Agreement: round2(float64(agree) / float64(shared) * 100),
				SevCorr:   round2(pearson(sevsA, sevsB)),
			})
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].Agreement > pairs[j].Agreement
	})

	agreeByModel := make(map[string][]float64)
	for _, p := range pairs {
		agreeByModel[p.ModelA] = append(agreeByModel[p.ModelA], p.Agreement)
		agreeByModel[p.ModelB] = append(agreeByModel[p.ModelB], p.Agreement)
	}

	promptsByModel := make(map[string]int)
	for _, pm := range index {
		for m := range pm {
			promptsByModel[m]++
		}
	}

	var summaries []ModelSummary
	for _, m := range models {
		avg := 0.0
		if vals := agreeByModel[m]; len(vals) > 0 {
			sum := 0.0
			for _, v := range vals {
				sum += v
			}
			avg = round2(sum / float64(len(vals)))
		}
		summaries = append(summaries, ModelSummary{
			Model:        m,
			Prompts:      promptsByModel[m],
			AvgAgreement: avg,
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].AvgAgreement > summaries[j].AvgAgreement
	})

	overall := 0.0
	if len(pairs) > 0 {
		sum := 0.0
		for _, p := range pairs {
			sum += p.Agreement
		}
		overall = round2(sum / float64(len(pairs)))
	}

	return &Report{Pairs: pairs, Summaries: summaries, Overall: overall}
}

func stateCategory(s model.FindingState) string {
	switch s {
	case model.StateValidated:
		return "pass"
	case model.StateRefuted:
		return "fail"
	default:
		return "other"
	}
}

func sevNum(s model.Severity) float64 {
	switch s {
	case model.SevCritical:
		return 4
	case model.SevHigh:
		return 3
	case model.SevMedium:
		return 2
	case model.SevLow:
		return 1
	default:
		return 0
	}
}

func pearson(x, y []float64) float64 {
	if len(x) != len(y) || len(x) < 2 {
		return 0
	}
	n := float64(len(x))
	var sumX, sumY, sumXY, sumX2, sumY2 float64
	for i := range x {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
		sumY2 += y[i] * y[i]
	}
	num := n*sumXY - sumX*sumY
	den := math.Sqrt((n*sumX2 - sumX*sumX) * (n*sumY2 - sumY*sumY))
	if den == 0 {
		return 0
	}
	return num / den
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func FormatText(r *Report) string {
	var b strings.Builder
	b.WriteString("Inter-Model Agreement Analysis\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")

	if len(r.Pairs) == 0 {
		b.WriteString("  No shared prompts found between models.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Overall agreement: %.1f%%\n\n", r.Overall)

	b.WriteString("  Model Summaries\n")
	b.WriteString("  " + strings.Repeat("-", 50) + "\n")
	for _, s := range r.Summaries {
		fmt.Fprintf(&b, "    %s: %d prompts, avg agreement %.1f%%\n",
			s.Model, s.Prompts, s.AvgAgreement)
	}
	b.WriteString("\n")

	b.WriteString("  Pairwise Agreements\n")
	b.WriteString("  " + strings.Repeat("-", 50) + "\n")
	for _, p := range r.Pairs {
		fmt.Fprintf(&b, "    %s vs %s: %d shared, %.1f%% agree, sev-corr=%.2f\n",
			p.ModelA, p.ModelB, p.Shared, p.Agreement, p.SevCorr)
	}
	b.WriteString("\n")

	return b.String()
}

func FormatJSON(r *Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
