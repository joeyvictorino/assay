package fleet

import (
	"fmt"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type ModelResult struct {
	Model        string             `json:"model"`
	OverallScore float64            `json:"overall_score"`
	PassRate     float64            `json:"pass_rate"`
	FindingCount int                `json:"finding_count"`
	ByChecker    map[string]float64 `json:"by_checker"`
	Duration     string             `json:"duration"`
	TokensUsed   int                `json:"tokens_used"`
}

type FleetReport struct {
	Models    []ModelResult `json:"models"`
	BestModel  string       `json:"best_model"`
	WorstModel string       `json:"worst_model"`
	Consensus  []string     `json:"consensus_findings"`
	Divergent  []string     `json:"divergent_findings"`
	Agreement  float64      `json:"agreement_rate"`
}

func AnalyzeFleet(rr model.RunReport, findings []model.Finding) FleetReport {
	modelScores := map[string]*ModelResult{}
	checkers := map[string]bool{}

	for _, ref := range rr.Reflection {
		mr, ok := modelScores[ref.Model]
		if !ok {
			mr = &ModelResult{
				Model:     ref.Model,
				ByChecker: map[string]float64{},
			}
			modelScores[ref.Model] = mr
		}
		lab := ref.Lab
		checkers[lab] = true
		if ref.Scored && ref.Final > 0 {
			mr.ByChecker[lab] = float64(ref.Final)
		}
		if ref.Passed {
			mr.PassRate++
		}
	}

	modelFindings := map[string]map[string]bool{}
	for _, f := range findings {
		m := f.Model.Model
		if m == "" {
			continue
		}
		if _, ok := modelScores[m]; !ok {
			modelScores[m] = &ModelResult{
				Model:     m,
				ByChecker: map[string]float64{},
			}
		}
		modelScores[m].FindingCount++
		if modelFindings[m] == nil {
			modelFindings[m] = map[string]bool{}
		}
		modelFindings[m][f.DedupKey] = true
	}

	for _, mr := range rr.Models {
		m := mr.Ref.Model
		if ms, ok := modelScores[m]; ok {
			ms.TokensUsed = int(mr.Usage.InputTokens + mr.Usage.OutputTokens)
		}
	}

	var results []ModelResult
	for _, ms := range modelScores {
		totalCheckers := 0
		totalScore := 0.0
		for _, s := range ms.ByChecker {
			totalScore += s
			totalCheckers++
		}
		if totalCheckers > 0 {
			ms.OverallScore = totalScore / float64(totalCheckers)
		}

		totalReflections := 0
		passedReflections := 0.0
		for _, ref := range rr.Reflection {
			if ref.Model == ms.Model {
				totalReflections++
				if ref.Passed {
					passedReflections++
				}
			}
		}
		if totalReflections > 0 {
			ms.PassRate = passedReflections / float64(totalReflections)
		}

		results = append(results, *ms)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].OverallScore > results[j].OverallScore
	})

	report := FleetReport{Models: results}

	if len(results) > 0 {
		report.BestModel = results[0].Model
		report.WorstModel = results[len(results)-1].Model
	}

	if len(modelFindings) > 1 {
		allKeys := map[string]int{}
		for _, fset := range modelFindings {
			for k := range fset {
				allKeys[k]++
			}
		}
		modelCount := len(modelFindings)
		for k, count := range allKeys {
			if count == modelCount {
				report.Consensus = append(report.Consensus, k)
			} else {
				report.Divergent = append(report.Divergent, k)
			}
		}
		sort.Strings(report.Consensus)
		sort.Strings(report.Divergent)

		total := len(report.Consensus) + len(report.Divergent)
		if total > 0 {
			report.Agreement = float64(len(report.Consensus)) / float64(total)
		}
	}

	return report
}

func SortBy(r FleetReport, field string) FleetReport {
	switch field {
	case "findings":
		sort.Slice(r.Models, func(i, j int) bool {
			return r.Models[i].FindingCount > r.Models[j].FindingCount
		})
	case "pass-rate":
		sort.Slice(r.Models, func(i, j int) bool {
			return r.Models[i].PassRate > r.Models[j].PassRate
		})
	default:
		sort.Slice(r.Models, func(i, j int) bool {
			return r.Models[i].OverallScore > r.Models[j].OverallScore
		})
	}
	return r
}

func FilterModel(r FleetReport, model string) FleetReport {
	var filtered []ModelResult
	for _, m := range r.Models {
		if m.Model == model {
			filtered = append(filtered, m)
		}
	}
	r.Models = filtered
	return r
}

func FormatText(r FleetReport) string {
	if len(r.Models) == 0 {
		return "No models found in run.\n"
	}

	var sb strings.Builder
	sb.WriteString("Fleet Assessment\n")
	sb.WriteString(strings.Repeat("=", 70) + "\n\n")

	sb.WriteString(fmt.Sprintf("  %-30s %8s %10s %8s %8s\n",
		"Model", "Score", "Pass Rate", "Findings", "Tokens"))
	sb.WriteString(fmt.Sprintf("  %-30s %8s %10s %8s %8s\n",
		strings.Repeat("-", 30), "--------", "----------", "--------", "--------"))

	for _, m := range r.Models {
		sb.WriteString(fmt.Sprintf("  %-30s %8.2f %9.0f%% %8d %8d\n",
			truncate(m.Model, 30), m.OverallScore, m.PassRate*100, m.FindingCount, m.TokensUsed))
	}

	sb.WriteString("\n")
	if r.BestModel != "" {
		sb.WriteString(fmt.Sprintf("  Best:  %s\n", r.BestModel))
		sb.WriteString(fmt.Sprintf("  Worst: %s\n", r.WorstModel))
	}

	sb.WriteString(fmt.Sprintf("\n  Agreement: %.0f%%  (%d consensus, %d divergent)\n",
		r.Agreement*100, len(r.Consensus), len(r.Divergent)))

	return sb.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
