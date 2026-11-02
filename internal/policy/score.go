package policy

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type ScoreResult struct {
	OverallScore   float64            `json:"overall_score"`
	Grade          string             `json:"grade"`
	AgentScores    map[string]float64 `json:"agent_scores"`
	CategoryScores map[string]float64 `json:"category_scores"`
	Deductions     []Deduction        `json:"deductions"`
	Summary        string             `json:"summary"`
}

type Deduction struct {
	Reason string  `json:"reason"`
	Points float64 `json:"points"`
}

var severityWeights = map[model.RiskTier]float64{
	model.TierCritical: 25.0,
	model.TierHigh:     15.0,
	model.TierMedium:   8.0,
	model.TierLow:      3.0,
}

func Score(e *Engine) ScoreResult {
	p := e.p
	score := 100.0
	var deductions []Deduction
	agentScores := make(map[string]float64)
	categoryScores := map[string]float64{
		"boundary_strength":   100.0,
		"rule_coverage":       100.0,
		"combination_safety":  100.0,
		"delegation_control":  100.0,
	}

	for name, ab := range p.Agents {
		agentScore := 100.0

		if ab.MaxTier == model.TierCritical {
			d := 15.0
			agentScore -= d
			deductions = append(deductions, Deduction{
				Reason: fmt.Sprintf("agent %q allows critical-tier tools", name),
				Points: d,
			})
		} else if ab.MaxTier == model.TierHigh {
			d := 5.0
			agentScore -= d
			deductions = append(deductions, Deduction{
				Reason: fmt.Sprintf("agent %q allows high-tier tools", name),
				Points: d,
			})
		}

		for _, t := range ab.AllowedTools {
			if t == Wildcard {
				d := 20.0
				agentScore -= d
				deductions = append(deductions, Deduction{
					Reason: fmt.Sprintf("agent %q uses wildcard tool access", name),
					Points: d,
				})
				categoryScores["boundary_strength"] -= 20.0
				break
			}
		}

		if ab.MaxDelegationDepth > 3 {
			d := float64(ab.MaxDelegationDepth-3) * 3.0
			agentScore -= d
			deductions = append(deductions, Deduction{
				Reason: fmt.Sprintf("agent %q allows deep delegation (depth %d)", name, ab.MaxDelegationDepth),
				Points: d,
			})
			categoryScores["delegation_control"] -= d
		}

		if ab.Budget.MaxUSD > 100 {
			d := math.Min(10.0, (ab.Budget.MaxUSD-100)/100*5)
			agentScore -= d
			deductions = append(deductions, Deduction{
				Reason: fmt.Sprintf("agent %q has high budget ($%.0f)", name, ab.Budget.MaxUSD),
				Points: d,
			})
		}

		agentScores[name] = math.Max(0, agentScore)
	}

	if p.DefaultEffect == model.EffectAllow {
		d := 20.0
		deductions = append(deductions, Deduction{
			Reason: "default effect is allow (deny recommended)",
			Points: d,
		})
		categoryScores["rule_coverage"] -= 20.0
	}

	denyRules := 0
	for _, r := range p.Rules {
		if r.Effect == model.EffectDeny {
			denyRules++
		}
	}
	if denyRules == 0 && len(p.Rules) > 0 {
		d := 10.0
		deductions = append(deductions, Deduction{
			Reason: "no explicit deny rules defined",
			Points: d,
		})
		categoryScores["rule_coverage"] -= 10.0
	}

	if len(p.DangerousCombos) == 0 {
		d := 5.0
		deductions = append(deductions, Deduction{
			Reason: "no dangerous tool combinations defined",
			Points: d,
		})
		categoryScores["combination_safety"] -= 5.0
	}

	for _, ded := range deductions {
		score -= ded.Points
	}
	score = math.Max(0, math.Min(100, score))

	for k, v := range categoryScores {
		categoryScores[k] = math.Max(0, math.Min(100, v))
	}

	grade := scoreToGrade(score)

	sort.Slice(deductions, func(i, j int) bool {
		return deductions[i].Points > deductions[j].Points
	})

	return ScoreResult{
		OverallScore:   math.Round(score*10) / 10,
		Grade:          grade,
		AgentScores:    agentScores,
		CategoryScores: categoryScores,
		Deductions:     deductions,
		Summary:        scoreSummary(score, grade, len(p.Agents), len(p.Rules)),
	}
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

func scoreSummary(score float64, grade string, agents, rules int) string {
	return fmt.Sprintf("Security posture score: %.1f/100 (Grade: %s) across %d agents and %d rules",
		score, grade, agents, rules)
}

func FormatScoreText(r ScoreResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Security Posture Score: %.1f / 100  (Grade: %s)\n", r.OverallScore, r.Grade)
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	b.WriteString("Category Scores:\n")
	cats := make([]string, 0, len(r.CategoryScores))
	for k := range r.CategoryScores {
		cats = append(cats, k)
	}
	sort.Strings(cats)
	for _, k := range cats {
		fmt.Fprintf(&b, "  %-25s %.1f\n", k, r.CategoryScores[k])
	}

	if len(r.AgentScores) > 0 {
		b.WriteString("\nAgent Scores:\n")
		agents := make([]string, 0, len(r.AgentScores))
		for k := range r.AgentScores {
			agents = append(agents, k)
		}
		sort.Strings(agents)
		for _, a := range agents {
			fmt.Fprintf(&b, "  %-25s %.1f\n", a, r.AgentScores[a])
		}
	}

	if len(r.Deductions) > 0 {
		b.WriteString("\nDeductions:\n")
		for _, d := range r.Deductions {
			fmt.Fprintf(&b, "  -%.1f  %s\n", d.Points, d.Reason)
		}
	}

	b.WriteString("\n" + r.Summary + "\n")
	return b.String()
}
