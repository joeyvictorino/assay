package consensus

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type Dissent struct {
	Model   string  `json:"model"`
	Verdict bool    `json:"verdict"`
	Score   float64 `json:"score"`
	Reason  string  `json:"reason"`
}

type Result struct {
	Prompt     string             `json:"prompt"`
	Checker    string             `json:"checker"`
	Models     []string           `json:"models"`
	Verdicts   map[string]bool    `json:"verdicts"`
	Scores     map[string]float64 `json:"scores"`
	Agreement  float64            `json:"agreement"`
	Consensus  float64            `json:"consensus"`
	Majority   bool               `json:"majority"`
	Dissenters []Dissent          `json:"dissenters,omitempty"`
}

type groupKey struct {
	prompt  string
	checker string
}

func Analyze(findings []model.Finding, minModels int) []Result {
	if minModels < 2 {
		minModels = 2
	}

	type modelEntry struct {
		model   string
		pass    bool
		score   float64
		summary string
	}

	groups := make(map[groupKey][]modelEntry)
	for _, f := range findings {
		key := groupKey{
			prompt:  f.PromptHash,
			checker: checkerName(f),
		}
		pass := f.State == model.StateValidated
		score := severityScore(f.Severity)
		groups[key] = append(groups[key], modelEntry{
			model:   f.Model.Model,
			pass:    pass,
			score:   score,
			summary: f.Summary,
		})
	}

	var results []Result
	for key, entries := range groups {
		verdicts := make(map[string]bool)
		scores := make(map[string]float64)
		for _, e := range entries {
			verdicts[e.model] = e.pass
			scores[e.model] = e.score
		}

		models := uniqueModels(verdicts)
		if len(models) < minModels {
			continue
		}

		passCount := 0
		for _, v := range verdicts {
			if v {
				passCount++
			}
		}
		majority := passCount > len(verdicts)/2

		agreement := float64(max(passCount, len(verdicts)-passCount)) / float64(len(verdicts))

		totalWeight := 0.0
		agreeWeight := 0.0
		for m, v := range verdicts {
			w := scores[m]
			if w == 0 {
				w = 0.5
			}
			totalWeight += w
			if v == majority {
				agreeWeight += w
			}
		}
		consensus := 0.0
		if totalWeight > 0 {
			consensus = agreeWeight / totalWeight
		}

		var dissenters []Dissent
		for m, v := range verdicts {
			if v != majority {
				reason := "disagreed with majority"
				for _, e := range entries {
					if e.model == m {
						if e.summary != "" {
							reason = truncate(e.summary, 120)
						}
						break
					}
				}
				dissenters = append(dissenters, Dissent{
					Model:   m,
					Verdict: v,
					Score:   scores[m],
					Reason:  reason,
				})
			}
		}
		sort.Slice(dissenters, func(i, j int) bool {
			return dissenters[i].Model < dissenters[j].Model
		})

		sort.Strings(models)
		results = append(results, Result{
			Prompt:     key.prompt,
			Checker:    key.checker,
			Models:     models,
			Verdicts:   verdicts,
			Scores:     scores,
			Agreement:  round2(agreement),
			Consensus:  round2(consensus),
			Majority:   majority,
			Dissenters: dissenters,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Agreement != results[j].Agreement {
			return results[i].Agreement < results[j].Agreement
		}
		return results[i].Prompt < results[j].Prompt
	})

	return results
}

func FormatText(results []Result) string {
	var b strings.Builder
	b.WriteString("Multi-Model Consensus Analysis\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")

	if len(results) == 0 {
		b.WriteString("  No prompt/checker groups met the minimum model threshold.\n")
		return b.String()
	}

	totalAgreement := 0.0
	totalConsensus := 0.0
	dissentCount := 0
	for _, r := range results {
		totalAgreement += r.Agreement
		totalConsensus += r.Consensus
		dissentCount += len(r.Dissenters)
	}
	n := float64(len(results))
	fmt.Fprintf(&b, "  Groups analyzed:        %d\n", len(results))
	fmt.Fprintf(&b, "  Mean agreement:         %.1f%%\n", totalAgreement/n*100)
	fmt.Fprintf(&b, "  Mean consensus:         %.1f%%\n", totalConsensus/n*100)
	fmt.Fprintf(&b, "  Total dissenters:       %d\n\n", dissentCount)

	for i, r := range results {
		verdict := "FAIL"
		if r.Majority {
			verdict = "PASS"
		}
		fmt.Fprintf(&b, "  [%d] prompt=%s checker=%s\n", i+1, ellipsis(r.Prompt, 30), r.Checker)
		fmt.Fprintf(&b, "      Models: %s\n", strings.Join(r.Models, ", "))
		fmt.Fprintf(&b, "      Majority: %s  Agreement: %.0f%%  Consensus: %.0f%%\n",
			verdict, r.Agreement*100, r.Consensus*100)
		if len(r.Dissenters) > 0 {
			b.WriteString("      Dissenters:\n")
			for _, d := range r.Dissenters {
				dv := "fail"
				if d.Verdict {
					dv = "pass"
				}
				fmt.Fprintf(&b, "        - %s (%s, score=%.2f): %s\n", d.Model, dv, d.Score, d.Reason)
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(results []Result) string {
	b, _ := json.MarshalIndent(results, "", "  ")
	return string(b) + "\n"
}

func checkerName(f model.Finding) string {
	if f.Validation != nil && f.Validation.Checker != "" {
		return f.Validation.Checker
	}
	return string(f.Class)
}

func severityScore(s model.Severity) float64 {
	switch s {
	case model.SevCritical:
		return 1.0
	case model.SevHigh:
		return 0.8
	case model.SevMedium:
		return 0.6
	case model.SevLow:
		return 0.4
	default:
		return 0.2
	}
}

func uniqueModels(verdicts map[string]bool) []string {
	out := make([]string, 0, len(verdicts))
	for m := range verdicts {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

func ellipsis(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
