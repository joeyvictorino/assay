package tokencount

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

const wordsToTokenRatio = 1.3

type PromptStats struct {
	PromptHash     string  `json:"prompt_hash"`
	Checker        string  `json:"checker"`
	Model          string  `json:"model"`
	PromptTokens   int     `json:"prompt_tokens"`
	ResponseTokens int     `json:"response_tokens"`
	TotalTokens    int     `json:"total_tokens"`
	FindingCount   int     `json:"finding_count"`
	TokensPerFind  float64 `json:"tokens_per_finding"`
}

type ModelStats struct {
	Model             string  `json:"model"`
	TotalPromptTok    int     `json:"total_prompt_tokens"`
	TotalResponseTok  int     `json:"total_response_tokens"`
	TotalTokens       int     `json:"total_tokens"`
	PromptCount       int     `json:"prompt_count"`
	FindingCount      int     `json:"finding_count"`
	AvgTokensPerPrompt float64 `json:"avg_tokens_per_prompt"`
	FindingsPer1K     float64 `json:"findings_per_1k_tokens"`
}

type CheckerStats struct {
	Checker        string  `json:"checker"`
	TotalTokens    int     `json:"total_tokens"`
	FindingCount   int     `json:"finding_count"`
	FindingsPer1K  float64 `json:"findings_per_1k_tokens"`
	PromptCount    int     `json:"prompt_count"`
}

type Report struct {
	ByPrompt       []PromptStats  `json:"by_prompt"`
	ByModel        []ModelStats   `json:"by_model"`
	ByChecker      []CheckerStats `json:"by_checker"`
	TotalTokens    int            `json:"total_tokens"`
	TotalFindings  int            `json:"total_findings"`
	TotalPrompts   int            `json:"total_prompts"`
	OverallFPer1K  float64        `json:"overall_findings_per_1k_tokens"`
}

func estimateTokens(text string) int {
	if text == "" {
		return 0
	}
	words := len(strings.Fields(text))
	return int(math.Ceil(float64(words) * wordsToTokenRatio))
}

type promptKey struct {
	hash    string
	checker string
	mdl     string
}

func Analyze(findings []model.Finding) Report {
	if len(findings) == 0 {
		return Report{}
	}

	promptMap := make(map[promptKey]*PromptStats)
	modelMap := make(map[string]*ModelStats)
	checkerMap := make(map[string]*CheckerStats)

	seenPrompts := make(map[promptKey]bool)

	for _, f := range findings {
		checker := checkerName(f)
		mdl := f.Model.Model
		ph := f.PromptHash
		if ph == "" {
			ph = "unknown"
		}

		promptTok := estimateTokens(f.Summary)
		responseTok := estimateTokens(f.Summary)
		totalTok := promptTok + responseTok

		pk := promptKey{hash: ph, checker: checker, mdl: mdl}
		ps, ok := promptMap[pk]
		if !ok {
			ps = &PromptStats{
				PromptHash: ph,
				Checker:    checker,
				Model:      mdl,
			}
			promptMap[pk] = ps
		}
		ps.PromptTokens += promptTok
		ps.ResponseTokens += responseTok
		ps.TotalTokens += totalTok
		ps.FindingCount++

		ms, ok := modelMap[mdl]
		if !ok {
			ms = &ModelStats{Model: mdl}
			modelMap[mdl] = ms
		}
		ms.TotalPromptTok += promptTok
		ms.TotalResponseTok += responseTok
		ms.TotalTokens += totalTok
		ms.FindingCount++
		if !seenPrompts[pk] {
			ms.PromptCount++
			seenPrompts[pk] = true
		}

		cs, ok := checkerMap[checker]
		if !ok {
			cs = &CheckerStats{Checker: checker}
			checkerMap[checker] = cs
		}
		cs.TotalTokens += totalTok
		cs.FindingCount++
	}

	var prompts []PromptStats
	for _, ps := range promptMap {
		if ps.FindingCount > 0 {
			ps.TokensPerFind = float64(ps.TotalTokens) / float64(ps.FindingCount)
		}
		prompts = append(prompts, *ps)
	}
	sort.Slice(prompts, func(i, j int) bool {
		return prompts[i].TotalTokens > prompts[j].TotalTokens
	})

	var models []ModelStats
	for _, ms := range modelMap {
		if ms.PromptCount > 0 {
			ms.AvgTokensPerPrompt = float64(ms.TotalTokens) / float64(ms.PromptCount)
		}
		if ms.TotalTokens > 0 {
			ms.FindingsPer1K = float64(ms.FindingCount) * 1000.0 / float64(ms.TotalTokens)
		}
		models = append(models, *ms)
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].TotalTokens > models[j].TotalTokens
	})

	checkerKeys := make(map[string]bool)
	var checkers []CheckerStats
	for _, cs := range checkerMap {
		if cs.TotalTokens > 0 {
			cs.FindingsPer1K = float64(cs.FindingCount) * 1000.0 / float64(cs.TotalTokens)
		}
		cs.PromptCount = countCheckerPrompts(prompts, cs.Checker)
		checkers = append(checkers, *cs)
		checkerKeys[cs.Checker] = true
	}
	sort.Slice(checkers, func(i, j int) bool {
		return checkers[i].TotalTokens > checkers[j].TotalTokens
	})

	var totalTok, totalFindings int
	for _, ms := range models {
		totalTok += ms.TotalTokens
		totalFindings += ms.FindingCount
	}

	var overallFPer1K float64
	if totalTok > 0 {
		overallFPer1K = float64(totalFindings) * 1000.0 / float64(totalTok)
	}

	return Report{
		ByPrompt:      prompts,
		ByModel:       models,
		ByChecker:     checkers,
		TotalTokens:   totalTok,
		TotalFindings: totalFindings,
		TotalPrompts:  len(prompts),
		OverallFPer1K: overallFPer1K,
	}
}

func countCheckerPrompts(prompts []PromptStats, checker string) int {
	n := 0
	for _, p := range prompts {
		if p.Checker == checker {
			n++
		}
	}
	return n
}

func checkerName(f model.Finding) string {
	if f.Validation != nil && f.Validation.Checker != "" {
		return f.Validation.Checker
	}
	return "unknown"
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Token Count Analysis\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")

	fmt.Fprintf(&b, "  Total tokens (estimated): %d\n", r.TotalTokens)
	fmt.Fprintf(&b, "  Total findings:           %d\n", r.TotalFindings)
	fmt.Fprintf(&b, "  Total prompts:            %d\n", r.TotalPrompts)
	fmt.Fprintf(&b, "  Findings per 1K tokens:   %.2f\n\n", r.OverallFPer1K)

	if len(r.ByModel) > 0 {
		b.WriteString("  By Model:\n")
		b.WriteString("  " + strings.Repeat("-", 60) + "\n")
		for _, ms := range r.ByModel {
			fmt.Fprintf(&b, "    %-30s %8d tokens, %d findings (%.2f/1K)\n",
				ms.Model, ms.TotalTokens, ms.FindingCount, ms.FindingsPer1K)
		}
		b.WriteString("\n")
	}

	if len(r.ByChecker) > 0 {
		b.WriteString("  By Checker:\n")
		b.WriteString("  " + strings.Repeat("-", 60) + "\n")
		for _, cs := range r.ByChecker {
			fmt.Fprintf(&b, "    %-30s %8d tokens, %d findings (%.2f/1K)\n",
				cs.Checker, cs.TotalTokens, cs.FindingCount, cs.FindingsPer1K)
		}
		b.WriteString("\n")
	}

	if len(r.ByPrompt) > 0 {
		limit := len(r.ByPrompt)
		if limit > 10 {
			limit = 10
		}
		b.WriteString("  Top Prompts (by token usage):\n")
		b.WriteString("  " + strings.Repeat("-", 60) + "\n")
		for _, ps := range r.ByPrompt[:limit] {
			fmt.Fprintf(&b, "    %s [%s/%s]: %d tokens, %d findings\n",
				ps.PromptHash, ps.Model, ps.Checker, ps.TotalTokens, ps.FindingCount)
		}
		if len(r.ByPrompt) > 10 {
			fmt.Fprintf(&b, "    ... and %d more\n", len(r.ByPrompt)-10)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
