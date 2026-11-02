package stability

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type Result struct {
	Prompt        string    `json:"prompt"`
	Model         string    `json:"model"`
	Checker       string    `json:"checker"`
	RunCount      int       `json:"run_count"`
	Verdicts      []bool    `json:"verdicts"`
	Scores        []float64 `json:"scores"`
	Consistency   float64   `json:"consistency"`
	ScoreVariance float64   `json:"score_variance"`
	FlipCount     int       `json:"flip_count"`
	Grade         string    `json:"grade"`
}

type groupKey struct {
	prompt  string
	model   string
	checker string
}

func Analyze(findings []model.Finding, minRuns int) []Result {
	if minRuns < 2 {
		minRuns = 2
	}

	type runEntry struct {
		pass  bool
		score float64
	}

	groups := make(map[groupKey][]runEntry)
	for _, f := range findings {
		key := groupKey{
			prompt:  f.PromptHash,
			model:   f.Model.Model,
			checker: checkerName(f),
		}
		pass := f.State == model.StateValidated
		score := severityScore(f.Severity)
		groups[key] = append(groups[key], runEntry{pass: pass, score: score})
	}

	var results []Result
	for key, entries := range groups {
		if len(entries) < minRuns {
			continue
		}

		verdicts := make([]bool, len(entries))
		scores := make([]float64, len(entries))
		for i, e := range entries {
			verdicts[i] = e.pass
			scores[i] = e.score
		}

		passCount := 0
		for _, v := range verdicts {
			if v {
				passCount++
			}
		}
		majority := float64(max(passCount, len(verdicts)-passCount)) / float64(len(verdicts))

		variance := scoreVariance(scores)
		flips := countFlips(verdicts)
		grade := gradeFromMetrics(majority, variance, flips, len(verdicts))

		results = append(results, Result{
			Prompt:        key.prompt,
			Model:         key.model,
			Checker:       key.checker,
			RunCount:      len(entries),
			Verdicts:      verdicts,
			Scores:        roundSlice(scores),
			Consistency:   round2(majority),
			ScoreVariance: round4(variance),
			FlipCount:     flips,
			Grade:         grade,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		gi := gradeRank(results[i].Grade)
		gj := gradeRank(results[j].Grade)
		if gi != gj {
			return gi > gj
		}
		return results[i].Prompt < results[j].Prompt
	})

	return results
}

func FormatText(results []Result) string {
	var b strings.Builder
	b.WriteString("Prompt Stability Analysis\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")

	if len(results) == 0 {
		b.WriteString("  No prompt/model/checker groups met the minimum run threshold.\n")
		return b.String()
	}

	gradeCounts := make(map[string]int)
	for _, r := range results {
		gradeCounts[r.Grade]++
	}

	fmt.Fprintf(&b, "  Groups analyzed:        %d\n", len(results))
	fmt.Fprintf(&b, "  Grade distribution:     ")
	for _, g := range []string{"A", "B", "C", "D", "F"} {
		if c := gradeCounts[g]; c > 0 {
			fmt.Fprintf(&b, "%s=%d ", g, c)
		}
	}
	b.WriteString("\n\n")

	for i, r := range results {
		fmt.Fprintf(&b, "  [%d] prompt=%s model=%s checker=%s\n",
			i+1, ellipsis(r.Prompt, 24), r.Model, r.Checker)
		fmt.Fprintf(&b, "      Runs: %d  Consistency: %.0f%%  Variance: %.4f  Flips: %d  Grade: %s\n",
			r.RunCount, r.Consistency*100, r.ScoreVariance, r.FlipCount, r.Grade)
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

func scoreVariance(scores []float64) float64 {
	if len(scores) < 2 {
		return 0
	}
	mean := 0.0
	for _, s := range scores {
		mean += s
	}
	mean /= float64(len(scores))

	sumSq := 0.0
	for _, s := range scores {
		d := s - mean
		sumSq += d * d
	}
	return sumSq / float64(len(scores)-1)
}

func countFlips(verdicts []bool) int {
	flips := 0
	for i := 1; i < len(verdicts); i++ {
		if verdicts[i] != verdicts[i-1] {
			flips++
		}
	}
	return flips
}

func gradeFromMetrics(consistency, variance float64, flips, runs int) string {
	flipRate := 0.0
	if runs > 1 {
		flipRate = float64(flips) / float64(runs-1)
	}

	if consistency >= 0.95 && variance < 0.01 && flipRate == 0 {
		return "A"
	}
	if consistency >= 0.85 && variance < 0.05 && flipRate <= 0.1 {
		return "B"
	}
	if consistency >= 0.70 && variance < 0.10 {
		return "C"
	}
	if consistency >= 0.55 {
		return "D"
	}
	return "F"
}

func gradeRank(g string) int {
	switch g {
	case "A":
		return 0
	case "B":
		return 1
	case "C":
		return 2
	case "D":
		return 3
	case "F":
		return 4
	}
	return 5
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}

func roundSlice(s []float64) []float64 {
	out := make([]float64, len(s))
	for i, v := range s {
		out[i] = round2(v)
	}
	return out
}

func ellipsis(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
