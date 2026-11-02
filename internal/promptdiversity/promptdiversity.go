package promptdiversity

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type ModelDiversity struct {
	Model         string  `json:"model"`
	UniquePrompts int     `json:"unique_prompts"`
	TotalFindings int     `json:"total_findings"`
	Entropy       float64 `json:"entropy"`
	Ratio         float64 `json:"ratio"`
}

type Report struct {
	Models         []ModelDiversity `json:"models"`
	OverallEntropy float64          `json:"overall_entropy"`
	OverallRatio   float64          `json:"overall_ratio"`
}

func Analyze(findings []model.Finding) Report {
	byModel := make(map[string][]model.Finding)
	for _, f := range findings {
		byModel[f.Model.Model] = append(byModel[f.Model.Model], f)
	}

	var models []ModelDiversity
	for m, fs := range byModel {
		md := analyzeModel(m, fs)
		models = append(models, md)
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].Model < models[j].Model
	})

	allPrompts := make(map[string]int)
	for _, f := range findings {
		if f.PromptHash != "" {
			allPrompts[f.PromptHash]++
		}
	}

	overallEntropy := shannon(allPrompts, len(findings))
	overallRatio := 0.0
	if len(findings) > 0 {
		overallRatio = float64(len(allPrompts)) / float64(len(findings))
		if overallRatio > 1.0 {
			overallRatio = 1.0
		}
	}

	return Report{
		Models:         models,
		OverallEntropy: round2(overallEntropy),
		OverallRatio:   round2(overallRatio),
	}
}

func analyzeModel(name string, findings []model.Finding) ModelDiversity {
	prompts := make(map[string]int)
	for _, f := range findings {
		if f.PromptHash != "" {
			prompts[f.PromptHash]++
		}
	}

	entropy := shannon(prompts, len(findings))
	ratio := 0.0
	if len(findings) > 0 {
		ratio = float64(len(prompts)) / float64(len(findings))
		if ratio > 1.0 {
			ratio = 1.0
		}
	}

	return ModelDiversity{
		Model:         name,
		UniquePrompts: len(prompts),
		TotalFindings: len(findings),
		Entropy:       round2(entropy),
		Ratio:         round2(ratio),
	}
}

func shannon(counts map[string]int, total int) float64 {
	if total == 0 || len(counts) == 0 {
		return 0
	}
	h := 0.0
	for _, c := range counts {
		if c > 0 {
			p := float64(c) / float64(total)
			h -= p * math.Log2(p)
		}
	}
	return h
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Prompt Template Diversity Analysis\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No findings to analyze.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models analyzed:      %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Overall entropy:      %.2f bits\n", r.OverallEntropy)
	fmt.Fprintf(&b, "  Overall unique ratio: %.0f%%\n\n", r.OverallRatio*100)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  %s\n", m.Model)
		fmt.Fprintf(&b, "    Findings: %d  Unique prompts: %d  Ratio: %.0f%%  Entropy: %.2f bits\n\n",
			m.TotalFindings, m.UniquePrompts, m.Ratio*100, m.Entropy)
	}

	return b.String()
}

func FormatJSON(r Report) string {
	b, _ := json.MarshalIndent(r, "", "  ")
	return string(b) + "\n"
}
