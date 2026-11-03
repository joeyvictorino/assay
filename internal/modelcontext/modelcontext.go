package modelcontext

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

type RunEntry struct {
	Model         string `json:"model"`
	PromptTokens  int    `json:"prompt_tokens"`
	OutputTokens  int    `json:"output_tokens"`
	MaxTokens     int    `json:"max_tokens"`
	ContextWindow int    `json:"context_window"`
	Truncated     bool   `json:"truncated"`
	Label         string `json:"label,omitempty"`
}

type ModelStats struct {
	Model              string  `json:"model"`
	Runs               int     `json:"runs"`
	AvgUtilization     float64 `json:"avg_utilization"`
	MaxUtilization     float64 `json:"max_utilization"`
	AvgInputOutputRatio float64 `json:"avg_io_ratio"`
	TruncatedCount     int     `json:"truncated_count"`
	TruncatedPct       float64 `json:"truncated_pct"`
	AvgPromptTokens    float64 `json:"avg_prompt_tokens"`
	AvgOutputTokens    float64 `json:"avg_output_tokens"`
}

type Finding struct {
	Severity    string `json:"severity"`
	Category    string `json:"category"`
	Model       string `json:"model"`
	Description string `json:"description"`
	RunIndex    int    `json:"run_index"`
}

type Report struct {
	TotalRuns    int          `json:"total_runs"`
	ModelStats   []ModelStats `json:"model_stats"`
	Findings     []Finding    `json:"findings"`
	OverallScore int          `json:"overall_score"`
	Grade        string       `json:"grade"`
}

func Analyze(entries []RunEntry, warnThreshold float64) Report {
	if len(entries) == 0 {
		return Report{OverallScore: 100, Grade: "A"}
	}

	byModel := map[string][]int{}
	for i, e := range entries {
		byModel[e.Model] = append(byModel[e.Model], i)
	}

	var stats []ModelStats
	var findings []Finding

	models := sortedKeys(byModel)
	for _, model := range models {
		indices := byModel[model]
		ms := ModelStats{Model: model, Runs: len(indices)}
		var totalUtil, totalRatio float64
		var maxUtil float64

		for _, idx := range indices {
			e := entries[idx]
			ctxWin := e.ContextWindow
			if ctxWin <= 0 {
				ctxWin = 128000
			}
			totalUsed := e.PromptTokens + e.OutputTokens
			util := float64(totalUsed) / float64(ctxWin)
			if util > maxUtil {
				maxUtil = util
			}
			totalUtil += util

			if e.OutputTokens > 0 {
				totalRatio += float64(e.PromptTokens) / float64(e.OutputTokens)
			}

			if e.Truncated {
				ms.TruncatedCount++
				findings = append(findings, Finding{
					Severity:    "high",
					Category:    "truncation",
					Model:       model,
					Description: fmt.Sprintf("Run %d was truncated (used %d/%d tokens)", idx, totalUsed, ctxWin),
					RunIndex:    idx,
				})
			}

			if util > warnThreshold {
				findings = append(findings, Finding{
					Severity:    severityForUtil(util),
					Category:    "high_utilization",
					Model:       model,
					Description: fmt.Sprintf("Run %d used %.1f%% of context window (%d/%d)", idx, util*100, totalUsed, ctxWin),
					RunIndex:    idx,
				})
			}

			if e.OutputTokens > 0 && e.PromptTokens > 0 {
				ratio := float64(e.PromptTokens) / float64(e.OutputTokens)
				if ratio > 50 {
					findings = append(findings, Finding{
						Severity:    "medium",
						Category:    "io_imbalance",
						Model:       model,
						Description: fmt.Sprintf("Run %d has extreme input/output ratio: %.1f:1 (%d prompt / %d output)", idx, ratio, e.PromptTokens, e.OutputTokens),
						RunIndex:    idx,
					})
				}
			}

			ms.AvgPromptTokens += float64(e.PromptTokens)
			ms.AvgOutputTokens += float64(e.OutputTokens)
		}

		n := float64(len(indices))
		ms.AvgUtilization = round4(totalUtil / n)
		ms.MaxUtilization = round4(maxUtil)
		ms.AvgInputOutputRatio = round4(totalRatio / n)
		ms.TruncatedPct = round4(float64(ms.TruncatedCount) / n)
		ms.AvgPromptTokens = math.Round(ms.AvgPromptTokens / n)
		ms.AvgOutputTokens = math.Round(ms.AvgOutputTokens / n)
		stats = append(stats, ms)
	}

	sort.Slice(findings, func(i, j int) bool {
		return sevOrder(findings[i].Severity) < sevOrder(findings[j].Severity)
	})

	score := computeScore(stats, findings)
	return Report{
		TotalRuns:    len(entries),
		ModelStats:   stats,
		Findings:     findings,
		OverallScore: score,
		Grade:        grade(score),
	}
}

func severityForUtil(util float64) string {
	if util >= 0.95 {
		return "critical"
	}
	if util >= 0.85 {
		return "high"
	}
	return "medium"
}

func computeScore(stats []ModelStats, findings []Finding) int {
	score := 100
	for _, f := range findings {
		switch f.Severity {
		case "critical":
			score -= 15
		case "high":
			score -= 10
		case "medium":
			score -= 5
		case "low":
			score -= 2
		}
	}
	if score < 0 {
		score = 0
	}
	return score
}

func grade(score int) string {
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

func sevOrder(s string) int {
	switch s {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	case "low":
		return 3
	default:
		return 4
	}
}

func round4(f float64) float64 {
	return math.Round(f*10000) / 10000
}

func sortedKeys(m map[string][]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func FormatText(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Context Window Utilization Report\n")
	fmt.Fprintf(&b, "%s\n\n", strings.Repeat("=", 62))

	if r.TotalRuns == 0 {
		fmt.Fprintln(&b, "  No runs to analyze.")
		return b.String()
	}

	fmt.Fprintf(&b, "  Total runs:      %d\n", r.TotalRuns)
	fmt.Fprintf(&b, "  Overall score:   %d/100 (%s)\n\n", r.OverallScore, r.Grade)

	for _, ms := range r.ModelStats {
		fmt.Fprintf(&b, "Model: %s (%d runs)\n", ms.Model, ms.Runs)
		fmt.Fprintf(&b, "%s\n", strings.Repeat("-", 62))
		fmt.Fprintf(&b, "  Avg utilization:    %.1f%%\n", ms.AvgUtilization*100)
		fmt.Fprintf(&b, "  Max utilization:    %.1f%%\n", ms.MaxUtilization*100)
		fmt.Fprintf(&b, "  Avg I/O ratio:      %.1f:1\n", ms.AvgInputOutputRatio)
		fmt.Fprintf(&b, "  Avg prompt tokens:  %.0f\n", ms.AvgPromptTokens)
		fmt.Fprintf(&b, "  Avg output tokens:  %.0f\n", ms.AvgOutputTokens)
		fmt.Fprintf(&b, "  Truncated:          %d (%.1f%%)\n\n", ms.TruncatedCount, ms.TruncatedPct*100)
	}

	if len(r.Findings) > 0 {
		fmt.Fprintf(&b, "Findings (%d)\n", len(r.Findings))
		fmt.Fprintf(&b, "%s\n", strings.Repeat("-", 62))
		for _, f := range r.Findings {
			fmt.Fprintf(&b, "  [%s] %s: %s\n", strings.ToUpper(f.Severity), f.Category, f.Description)
		}
		fmt.Fprintln(&b)
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data)
}
