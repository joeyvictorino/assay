package injdefense

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type DefenseType string

const (
	DefInputFilter   DefenseType = "input-filter"
	DefOutputFilter  DefenseType = "output-filter"
	DefSystemPrompt  DefenseType = "system-prompt"
	DefRateLimiting  DefenseType = "rate-limiting"
	DefContentPolicy DefenseType = "content-policy"
)

var AllDefenseTypes = []DefenseType{
	DefInputFilter, DefOutputFilter, DefSystemPrompt,
	DefRateLimiting, DefContentPolicy,
}

type AttackOutcome struct {
	AttackClass string `json:"attack_class"`
	Blocked     int    `json:"blocked"`
	Bypassed    int    `json:"bypassed"`
	Total       int    `json:"total"`
	BlockRate   float64 `json:"block_rate"`
}

type DefenseResult struct {
	Defense       DefenseType     `json:"defense"`
	Attacks       []AttackOutcome `json:"attacks"`
	TotalBlocked  int             `json:"total_blocked"`
	TotalBypassed int             `json:"total_bypassed"`
	TotalAttempts int             `json:"total_attempts"`
	BlockRate     float64         `json:"block_rate"`
	Grade         string          `json:"grade"`
}

type Summary struct {
	TotalDefenses     int     `json:"total_defenses"`
	TotalAttempts     int     `json:"total_attempts"`
	OverallBlockRate  float64 `json:"overall_block_rate"`
	WeakestDefense    string  `json:"weakest_defense"`
	StrongestDefense  string  `json:"strongest_defense"`
	CriticalBypasses  int     `json:"critical_bypasses"`
}

type Report struct {
	Defenses []DefenseResult `json:"defenses"`
	Summary  Summary         `json:"summary"`
}

func Analyze(withDefense, withoutDefense []model.Finding) Report {
	defenseFindings := groupByClass(withDefense)
	baselineFindings := groupByClass(withoutDefense)

	defenseResults := make(map[DefenseType]map[string]*AttackOutcome)

	for class, baseline := range baselineFindings {
		defended := defenseFindings[class]
		defType := classifyDefense(class)

		if _, ok := defenseResults[defType]; !ok {
			defenseResults[defType] = make(map[string]*AttackOutcome)
		}

		baseCount := len(baseline)
		defCount := len(defended)
		blocked := baseCount - defCount
		if blocked < 0 {
			blocked = 0
		}
		bypassed := defCount

		ao := defenseResults[defType][class]
		if ao == nil {
			ao = &AttackOutcome{AttackClass: class}
			defenseResults[defType][class] = ao
		}
		ao.Blocked += blocked
		ao.Bypassed += bypassed
		ao.Total += baseCount
	}

	for class, defended := range defenseFindings {
		if _, exists := baselineFindings[class]; exists {
			continue
		}
		defType := classifyDefense(class)
		if _, ok := defenseResults[defType]; !ok {
			defenseResults[defType] = make(map[string]*AttackOutcome)
		}
		ao := defenseResults[defType][class]
		if ao == nil {
			ao = &AttackOutcome{AttackClass: class}
			defenseResults[defType][class] = ao
		}
		ao.Bypassed += len(defended)
		ao.Total += len(defended)
	}

	var results []DefenseResult
	for defType, attacks := range defenseResults {
		dr := DefenseResult{Defense: defType}
		for _, ao := range attacks {
			if ao.Total > 0 {
				ao.BlockRate = float64(ao.Blocked) / float64(ao.Total) * 100
			}
			dr.Attacks = append(dr.Attacks, *ao)
			dr.TotalBlocked += ao.Blocked
			dr.TotalBypassed += ao.Bypassed
			dr.TotalAttempts += ao.Total
		}
		sort.Slice(dr.Attacks, func(i, j int) bool {
			return dr.Attacks[i].AttackClass < dr.Attacks[j].AttackClass
		})
		if dr.TotalAttempts > 0 {
			dr.BlockRate = float64(dr.TotalBlocked) / float64(dr.TotalAttempts) * 100
		}
		dr.Grade = rateToGrade(dr.BlockRate)
		results = append(results, dr)
	}

	sort.Slice(results, func(i, j int) bool {
		return string(results[i].Defense) < string(results[j].Defense)
	})

	summary := Summary{TotalDefenses: len(results)}
	var totalBlocked, totalAttempts, critBypasses int
	var weakest, strongest string
	weakestRate := math.MaxFloat64
	strongestRate := -1.0

	for _, dr := range results {
		totalBlocked += dr.TotalBlocked
		totalAttempts += dr.TotalAttempts
		if dr.BlockRate < weakestRate {
			weakestRate = dr.BlockRate
			weakest = string(dr.Defense)
		}
		if dr.BlockRate > strongestRate {
			strongestRate = dr.BlockRate
			strongest = string(dr.Defense)
		}
		for _, a := range dr.Attacks {
			if a.Bypassed > 0 && isCriticalClass(a.AttackClass) {
				critBypasses += a.Bypassed
			}
		}
	}

	if totalAttempts > 0 {
		summary.OverallBlockRate = float64(totalBlocked) / float64(totalAttempts) * 100
	}
	summary.TotalAttempts = totalAttempts
	summary.WeakestDefense = weakest
	summary.StrongestDefense = strongest
	summary.CriticalBypasses = critBypasses

	return Report{Defenses: results, Summary: summary}
}

func groupByClass(findings []model.Finding) map[string][]model.Finding {
	m := make(map[string][]model.Finding)
	for _, f := range findings {
		m[string(f.Class)] = append(m[string(f.Class)], f)
	}
	return m
}

func classifyDefense(class string) DefenseType {
	lower := strings.ToLower(class)
	switch {
	case strings.Contains(lower, "injection") || strings.Contains(lower, "inject"):
		return DefInputFilter
	case strings.Contains(lower, "exfil") || strings.Contains(lower, "leak"):
		return DefOutputFilter
	case strings.Contains(lower, "prompt") || strings.Contains(lower, "system"):
		return DefSystemPrompt
	case strings.Contains(lower, "rate") || strings.Contains(lower, "dos"):
		return DefRateLimiting
	default:
		return DefContentPolicy
	}
}

func isCriticalClass(class string) bool {
	lower := strings.ToLower(class)
	return strings.Contains(lower, "injection") ||
		strings.Contains(lower, "exfil") ||
		strings.Contains(lower, "rce") ||
		strings.Contains(lower, "ssrf")
}

func rateToGrade(rate float64) string {
	switch {
	case rate >= 95:
		return "A"
	case rate >= 85:
		return "B"
	case rate >= 70:
		return "C"
	case rate >= 50:
		return "D"
	default:
		return "F"
	}
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Prompt Injection Defense Effectiveness\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")

	fmt.Fprintf(&b, "  Defenses evaluated:    %d\n", r.Summary.TotalDefenses)
	fmt.Fprintf(&b, "  Total attack attempts: %d\n", r.Summary.TotalAttempts)
	fmt.Fprintf(&b, "  Overall block rate:    %.1f%%\n", r.Summary.OverallBlockRate)
	fmt.Fprintf(&b, "  Critical bypasses:     %d\n", r.Summary.CriticalBypasses)
	if r.Summary.StrongestDefense != "" {
		fmt.Fprintf(&b, "  Strongest defense:     %s\n", r.Summary.StrongestDefense)
	}
	if r.Summary.WeakestDefense != "" {
		fmt.Fprintf(&b, "  Weakest defense:       %s\n", r.Summary.WeakestDefense)
	}
	b.WriteString("\n")

	if len(r.Defenses) == 0 {
		b.WriteString("  No defense data to analyze.\n")
		return b.String()
	}

	for _, d := range r.Defenses {
		fmt.Fprintf(&b, "  Defense: %s [%s]\n", d.Defense, d.Grade)
		fmt.Fprintf(&b, "    Block rate: %.1f%% (%d/%d)\n", d.BlockRate, d.TotalBlocked, d.TotalAttempts)
		for _, a := range d.Attacks {
			fmt.Fprintf(&b, "    %s: %d blocked, %d bypassed (%.1f%%)\n",
				a.AttackClass, a.Blocked, a.Bypassed, a.BlockRate)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
