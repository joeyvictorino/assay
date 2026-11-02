package reliability

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type ModelReliability struct {
	Model         string  `json:"model"`
	Runs          int     `json:"runs"`
	RefusalRate   float64 `json:"refusal_rate"`
	StateStability float64 `json:"state_stability"`
	SevAgreement  float64 `json:"severity_agreement"`
	Score         float64 `json:"score"`
}

type Report struct {
	Models       []ModelReliability `json:"models"`
	OverallScore float64            `json:"overall_score"`
}

func Analyze(findings []model.Finding) Report {
	byModel := make(map[string][]model.Finding)
	for _, f := range findings {
		byModel[f.Model.Model] = append(byModel[f.Model.Model], f)
	}

	var models []ModelReliability
	for m, fs := range byModel {
		mr := analyzeModel(m, fs)
		models = append(models, mr)
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].Model < models[j].Model
	})

	overall := 0.0
	if len(models) > 0 {
		sum := 0.0
		for _, m := range models {
			sum += m.Score
		}
		overall = sum / float64(len(models))
	}

	return Report{
		Models:       models,
		OverallScore: round2(overall),
	}
}

func analyzeModel(name string, findings []model.Finding) ModelReliability {
	total := len(findings)
	if total == 0 {
		return ModelReliability{Model: name}
	}

	refusals := 0
	stateCounts := make(map[model.FindingState]int)
	sevCounts := make(map[model.Severity]int)

	for _, f := range findings {
		if f.State == model.StateDeclined {
			refusals++
		}
		stateCounts[f.State]++
		sevCounts[f.Severity]++
	}

	refusalRate := float64(refusals) / float64(total)

	stateStability := dominance(stateCounts, total)

	byPrompt := make(map[string][]model.Finding)
	for _, f := range findings {
		if f.PromptHash != "" {
			byPrompt[f.PromptHash] = append(byPrompt[f.PromptHash], f)
		}
	}

	sevAgreement := 0.0
	counted := 0
	for _, group := range byPrompt {
		if len(group) < 2 {
			continue
		}
		sev := make(map[model.Severity]int)
		for _, f := range group {
			sev[f.Severity]++
		}
		maxCount := 0
		for _, c := range sev {
			if c > maxCount {
				maxCount = c
			}
		}
		sevAgreement += float64(maxCount) / float64(len(group))
		counted++
	}
	if counted > 0 {
		sevAgreement /= float64(counted)
	} else {
		sevAgreement = stateStability
	}

	score := (1-refusalRate)*0.3 + stateStability*0.35 + sevAgreement*0.35

	return ModelReliability{
		Model:         name,
		Runs:          total,
		RefusalRate:   round2(refusalRate),
		StateStability: round2(stateStability),
		SevAgreement:  round2(sevAgreement),
		Score:         round2(score),
	}
}

func dominance(counts map[model.FindingState]int, total int) float64 {
	if total == 0 {
		return 0
	}
	maxCount := 0
	for _, c := range counts {
		if c > maxCount {
			maxCount = c
		}
	}
	return float64(maxCount) / float64(total)
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Model Reliability Analysis\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No findings to analyze.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models analyzed:      %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Overall reliability:  %.0f%%\n\n", r.OverallScore*100)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  %s\n", m.Model)
		fmt.Fprintf(&b, "    Runs: %d  Refusal: %.0f%%  State stability: %.0f%%  Sev agreement: %.0f%%  Score: %.0f%%\n\n",
			m.Runs, m.RefusalRate*100, m.StateStability*100, m.SevAgreement*100, m.Score*100)
	}

	return b.String()
}

func FormatJSON(r Report) string {
	b, _ := json.MarshalIndent(r, "", "  ")
	return string(b) + "\n"
}
