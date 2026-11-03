package refusal

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

var refusalPatterns = []string{
	"i cannot",
	"i can't",
	"i'm sorry, i can't",
	"i'm not able to",
	"as an ai",
	"i don't have the ability",
	"i'm unable to",
	"i must decline",
	"i cannot assist",
	"i'm not allowed",
}

type ModelRate struct {
	Model       string  `json:"model"`
	Total       int     `json:"total"`
	Refusals    int     `json:"refusals"`
	Rate        float64 `json:"rate"`
	TopPatterns []string `json:"top_patterns,omitempty"`
}

type PromptBreakdown struct {
	PromptHash    string             `json:"prompt_hash"`
	Total         int                `json:"total"`
	Refusals      int                `json:"refusals"`
	Rate          float64            `json:"rate"`
	RefusingModels []string          `json:"refusing_models,omitempty"`
	AlwaysRefused bool               `json:"always_refused"`
}

type CheckerBreakdown struct {
	Checker  string  `json:"checker"`
	Total    int     `json:"total"`
	Refusals int     `json:"refusals"`
	Rate     float64 `json:"rate"`
}

type Report struct {
	TotalFindings   int                `json:"total_findings"`
	TotalRefusals   int                `json:"total_refusals"`
	OverallRate     float64            `json:"overall_rate"`
	ByModel         []ModelRate        `json:"by_model"`
	ByPrompt        []PromptBreakdown  `json:"by_prompt"`
	ByChecker       []CheckerBreakdown `json:"by_checker"`
	AlwaysRefused   []string           `json:"always_refused"`
}

func IsRefusal(f model.Finding) bool {
	if f.State == model.StateDeclined {
		return true
	}
	if f.Summary == "" {
		return true
	}
	lower := strings.ToLower(f.Summary)
	for _, pat := range refusalPatterns {
		if strings.Contains(lower, pat) {
			return true
		}
	}
	return false
}

func matchedPattern(f model.Finding) string {
	if f.State == model.StateDeclined {
		return "state:declined"
	}
	if f.Summary == "" {
		return "empty-response"
	}
	lower := strings.ToLower(f.Summary)
	for _, pat := range refusalPatterns {
		if strings.Contains(lower, pat) {
			return pat
		}
	}
	return "unknown"
}

func Analyze(findings []model.Finding) Report {
	type modelStats struct {
		total    int
		refusals int
		patterns map[string]int
	}
	type promptStats struct {
		total         int
		refusals      int
		refusingModels map[string]bool
	}
	type checkerStats struct {
		total    int
		refusals int
	}

	byModel := make(map[string]*modelStats)
	byPrompt := make(map[string]*promptStats)
	byChecker := make(map[string]*checkerStats)
	totalRefusals := 0

	for _, f := range findings {
		m := f.Model.Model
		if byModel[m] == nil {
			byModel[m] = &modelStats{patterns: make(map[string]int)}
		}
		byModel[m].total++

		ph := f.PromptHash
		if ph == "" {
			ph = "(no-hash)"
		}
		if byPrompt[ph] == nil {
			byPrompt[ph] = &promptStats{refusingModels: make(map[string]bool)}
		}
		byPrompt[ph].total++

		ck := checkerName(f)
		if byChecker[ck] == nil {
			byChecker[ck] = &checkerStats{}
		}
		byChecker[ck].total++

		if IsRefusal(f) {
			totalRefusals++
			byModel[m].refusals++
			byModel[m].patterns[matchedPattern(f)]++
			byPrompt[ph].refusals++
			byPrompt[ph].refusingModels[m] = true
			byChecker[ck].refusals++
		}
	}

	var modelRates []ModelRate
	for m, s := range byModel {
		rate := 0.0
		if s.total > 0 {
			rate = float64(s.refusals) / float64(s.total)
		}
		var topPats []string
		type kv struct {
			k string
			v int
		}
		var pats []kv
		for k, v := range s.patterns {
			pats = append(pats, kv{k, v})
		}
		sort.Slice(pats, func(i, j int) bool { return pats[i].v > pats[j].v })
		for i, p := range pats {
			if i >= 3 {
				break
			}
			topPats = append(topPats, p.k)
		}
		modelRates = append(modelRates, ModelRate{
			Model:       m,
			Total:       s.total,
			Refusals:    s.refusals,
			Rate:        round3(rate),
			TopPatterns: topPats,
		})
	}
	sort.Slice(modelRates, func(i, j int) bool {
		if modelRates[i].Rate != modelRates[j].Rate {
			return modelRates[i].Rate > modelRates[j].Rate
		}
		return modelRates[i].Model < modelRates[j].Model
	})

	var promptBreaks []PromptBreakdown
	var alwaysRefused []string
	for ph, s := range byPrompt {
		rate := 0.0
		if s.total > 0 {
			rate = float64(s.refusals) / float64(s.total)
		}
		always := s.total > 0 && s.refusals == s.total
		var models []string
		for m := range s.refusingModels {
			models = append(models, m)
		}
		sort.Strings(models)
		promptBreaks = append(promptBreaks, PromptBreakdown{
			PromptHash:     ph,
			Total:          s.total,
			Refusals:       s.refusals,
			Rate:           round3(rate),
			RefusingModels: models,
			AlwaysRefused:  always,
		})
		if always {
			alwaysRefused = append(alwaysRefused, ph)
		}
	}
	sort.Slice(promptBreaks, func(i, j int) bool {
		if promptBreaks[i].Rate != promptBreaks[j].Rate {
			return promptBreaks[i].Rate > promptBreaks[j].Rate
		}
		return promptBreaks[i].PromptHash < promptBreaks[j].PromptHash
	})
	sort.Strings(alwaysRefused)

	var checkerBreaks []CheckerBreakdown
	for ck, s := range byChecker {
		rate := 0.0
		if s.total > 0 {
			rate = float64(s.refusals) / float64(s.total)
		}
		checkerBreaks = append(checkerBreaks, CheckerBreakdown{
			Checker:  ck,
			Total:    s.total,
			Refusals: s.refusals,
			Rate:     round3(rate),
		})
	}
	sort.Slice(checkerBreaks, func(i, j int) bool {
		if checkerBreaks[i].Rate != checkerBreaks[j].Rate {
			return checkerBreaks[i].Rate > checkerBreaks[j].Rate
		}
		return checkerBreaks[i].Checker < checkerBreaks[j].Checker
	})

	overallRate := 0.0
	if len(findings) > 0 {
		overallRate = float64(totalRefusals) / float64(len(findings))
	}

	return Report{
		TotalFindings: len(findings),
		TotalRefusals: totalRefusals,
		OverallRate:   round3(overallRate),
		ByModel:       modelRates,
		ByPrompt:      promptBreaks,
		ByChecker:     checkerBreaks,
		AlwaysRefused: alwaysRefused,
	}
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Model Refusal Rate Analysis\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")

	fmt.Fprintf(&b, "  Total findings:         %d\n", r.TotalFindings)
	fmt.Fprintf(&b, "  Total refusals:         %d\n", r.TotalRefusals)
	fmt.Fprintf(&b, "  Overall refusal rate:   %.1f%%\n\n", r.OverallRate*100)

	if len(r.ByModel) > 0 {
		b.WriteString("  By Model:\n")
		b.WriteString("  " + strings.Repeat("-", 58) + "\n")
		for _, m := range r.ByModel {
			fmt.Fprintf(&b, "    %-30s %3d/%3d  (%.1f%%)\n", m.Model, m.Refusals, m.Total, m.Rate*100)
			if len(m.TopPatterns) > 0 {
				fmt.Fprintf(&b, "      patterns: %s\n", strings.Join(m.TopPatterns, ", "))
			}
		}
		b.WriteString("\n")
	}

	if len(r.ByChecker) > 0 {
		b.WriteString("  By Checker:\n")
		b.WriteString("  " + strings.Repeat("-", 58) + "\n")
		for _, c := range r.ByChecker {
			fmt.Fprintf(&b, "    %-30s %3d/%3d  (%.1f%%)\n", c.Checker, c.Refusals, c.Total, c.Rate*100)
		}
		b.WriteString("\n")
	}

	if len(r.AlwaysRefused) > 0 {
		fmt.Fprintf(&b, "  Always-Refused Prompts: %d\n", len(r.AlwaysRefused))
		b.WriteString("  " + strings.Repeat("-", 58) + "\n")
		for _, p := range r.AlwaysRefused {
			fmt.Fprintf(&b, "    %s\n", ellipsis(p, 56))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	b, _ := json.MarshalIndent(r, "", "  ")
	return string(b) + "\n"
}

func checkerName(f model.Finding) string {
	if f.Validation != nil && f.Validation.Checker != "" {
		return f.Validation.Checker
	}
	return string(f.Class)
}

func round3(v float64) float64 {
	return float64(int(v*1000+0.5)) / 1000
}

func ellipsis(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
