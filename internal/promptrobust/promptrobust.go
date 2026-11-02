package promptrobust

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type PromptGroup struct {
	PromptHash   string `json:"prompt_hash"`
	FindingCount int    `json:"finding_count"`
	Classes      []string `json:"classes"`
}

type ModelRobustness struct {
	Model            string        `json:"model"`
	TotalFindings    int           `json:"total_findings"`
	UniquePrompts    int           `json:"unique_prompts"`
	ConsistencyScore float64       `json:"consistency_score"`
	StabilityScore   float64       `json:"stability_score"`
	OverallScore     float64       `json:"overall_score"`
	Grade            string        `json:"grade"`
	PromptGroups     []PromptGroup `json:"prompt_groups"`
	ClassVariance    float64       `json:"class_variance"`
	SeverityVariance float64       `json:"severity_variance"`
}

type Report struct {
	Models       []ModelRobustness `json:"models"`
	MostRobust   string            `json:"most_robust"`
	LeastRobust  string            `json:"least_robust"`
	MeanScore    float64           `json:"mean_score"`
	TotalPrompts int               `json:"total_prompts"`
}

var sevRank = map[model.Severity]float64{
	model.SevCritical: 5.0,
	model.SevHigh:     4.0,
	model.SevMedium:   3.0,
	model.SevLow:      2.0,
	model.SevInfo:     1.0,
}

func Analyze(findings []model.Finding) Report {
	byModel := make(map[string][]model.Finding)
	for _, f := range findings {
		m := f.Model.Model
		if m == "" {
			m = "unknown"
		}
		byModel[m] = append(byModel[m], f)
	}

	allPrompts := make(map[string]bool)
	for _, f := range findings {
		if f.PromptHash != "" {
			allPrompts[f.PromptHash] = true
		}
	}

	var models []ModelRobustness
	for name, mf := range byModel {
		mr := scoreModel(name, mf)
		models = append(models, mr)
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].OverallScore > models[j].OverallScore
	})

	var best, worst string
	var total float64
	if len(models) > 0 {
		best = models[0].Model
		worst = models[len(models)-1].Model
		for _, m := range models {
			total += m.OverallScore
		}
	}

	var mean float64
	if len(models) > 0 {
		mean = total / float64(len(models))
	}

	return Report{
		Models:       models,
		MostRobust:   best,
		LeastRobust:  worst,
		MeanScore:    math.Round(mean*10) / 10,
		TotalPrompts: len(allPrompts),
	}
}

func scoreModel(name string, findings []model.Finding) ModelRobustness {
	byPrompt := make(map[string][]model.Finding)
	for _, f := range findings {
		h := f.PromptHash
		if h == "" {
			h = "no-hash"
		}
		byPrompt[h] = append(byPrompt[h], f)
	}

	var groups []PromptGroup
	for hash, pf := range byPrompt {
		classSet := make(map[string]bool)
		for _, f := range pf {
			classSet[string(f.Class)] = true
		}
		classes := make([]string, 0, len(classSet))
		for c := range classSet {
			classes = append(classes, c)
		}
		sort.Strings(classes)
		groups = append(groups, PromptGroup{
			PromptHash:   hash,
			FindingCount: len(pf),
			Classes:      classes,
		})
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].PromptHash < groups[j].PromptHash
	})

	consistency := computeConsistency(groups)
	classVar := computeClassVariance(findings)
	sevVar := computeSeverityVariance(findings)

	stability := 100.0
	if sevVar > 0 {
		stability = math.Max(0, 100-sevVar*20)
	}
	if classVar > 0 {
		stability = math.Max(0, stability-classVar*10)
	}

	overall := (consistency*0.6 + stability*0.4)
	overall = math.Round(overall*10) / 10

	return ModelRobustness{
		Model:            name,
		TotalFindings:    len(findings),
		UniquePrompts:    len(byPrompt),
		ConsistencyScore: math.Round(consistency*10) / 10,
		StabilityScore:   math.Round(stability*10) / 10,
		OverallScore:     overall,
		Grade:            scoreToGrade(overall),
		PromptGroups:     groups,
		ClassVariance:    math.Round(classVar*100) / 100,
		SeverityVariance: math.Round(sevVar*100) / 100,
	}
}

func computeConsistency(groups []PromptGroup) float64 {
	if len(groups) <= 1 {
		return 100.0
	}
	counts := make([]float64, len(groups))
	var sum float64
	for i, g := range groups {
		counts[i] = float64(g.FindingCount)
		sum += counts[i]
	}
	mean := sum / float64(len(counts))
	if mean == 0 {
		return 100.0
	}
	var variance float64
	for _, c := range counts {
		d := c - mean
		variance += d * d
	}
	variance /= float64(len(counts))
	cv := math.Sqrt(variance) / mean
	score := math.Max(0, 100-cv*50)
	return score
}

func computeClassVariance(findings []model.Finding) float64 {
	if len(findings) == 0 {
		return 0
	}
	classCounts := make(map[string]int)
	for _, f := range findings {
		classCounts[string(f.Class)]++
	}
	if len(classCounts) <= 1 {
		return 0
	}
	var sum float64
	for _, c := range classCounts {
		sum += float64(c)
	}
	mean := sum / float64(len(classCounts))
	var variance float64
	for _, c := range classCounts {
		d := float64(c) - mean
		variance += d * d
	}
	variance /= float64(len(classCounts))
	return math.Sqrt(variance)
}

func computeSeverityVariance(findings []model.Finding) float64 {
	if len(findings) == 0 {
		return 0
	}
	var ranks []float64
	for _, f := range findings {
		r := sevRank[f.Severity]
		if r == 0 {
			r = 1.0
		}
		ranks = append(ranks, r)
	}
	var sum float64
	for _, r := range ranks {
		sum += r
	}
	mean := sum / float64(len(ranks))
	var variance float64
	for _, r := range ranks {
		d := r - mean
		variance += d * d
	}
	variance /= float64(len(ranks))
	return math.Sqrt(variance)
}

func scoreToGrade(score float64) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "F"
	}
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Prompt Robustness Report\n")
	b.WriteString(strings.Repeat("=", 55) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No robustness data to analyze.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated: %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Total prompts:    %d\n", r.TotalPrompts)
	fmt.Fprintf(&b, "  Mean score:       %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Most robust:      %s\n", r.MostRobust)
	fmt.Fprintf(&b, "  Least robust:     %s\n\n", r.LeastRobust)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s [%s] (%.1f/100)\n", m.Model, m.Grade, m.OverallScore)
		fmt.Fprintf(&b, "    Total findings:    %d\n", m.TotalFindings)
		fmt.Fprintf(&b, "    Unique prompts:    %d\n", m.UniquePrompts)
		fmt.Fprintf(&b, "    Consistency:       %.1f\n", m.ConsistencyScore)
		fmt.Fprintf(&b, "    Stability:         %.1f\n", m.StabilityScore)
		fmt.Fprintf(&b, "    Class variance:    %.2f\n", m.ClassVariance)
		fmt.Fprintf(&b, "    Severity variance: %.2f\n", m.SeverityVariance)
		for _, g := range m.PromptGroups {
			fmt.Fprintf(&b, "    Prompt %s: %d findings [%s]\n",
				g.PromptHash, g.FindingCount, strings.Join(g.Classes, ", "))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
