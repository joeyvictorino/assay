package modelrank

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type ModelScore struct {
	Model         string             `json:"model"`
	OverallScore  float64            `json:"overall_score"`
	CheckerScores map[string]float64 `json:"checker_scores"`
	RunCount      int                `json:"run_count"`
	Rank          int                `json:"rank"`
}

type Leaderboard struct {
	Models   []ModelScore `json:"models"`
	Checkers []string     `json:"checkers"`
}

type scoreAccum struct {
	sum   float64
	count int
}

func BuildLeaderboard(rr model.RunReport, findings []model.Finding) Leaderboard {
	// Accumulate scores per model per checker (lab).
	type key struct{ model, checker string }
	accum := map[key]*scoreAccum{}
	checkerSet := map[string]bool{}

	for _, f := range findings {
		m := f.Model.Model
		c := f.Lab
		if m == "" || c == "" {
			continue
		}
		checkerSet[c] = true
		k := key{m, c}
		if accum[k] == nil {
			accum[k] = &scoreAccum{}
		}
		accum[k].sum += severityScore(f.Severity)
		accum[k].count++
	}

	// Also incorporate reflection scores when available.
	for _, r := range rr.Reflection {
		if r.Model == "" || r.Lab == "" || !r.Scored {
			continue
		}
		checkerSet[r.Lab] = true
		k := key{r.Model, r.Lab}
		if accum[k] == nil {
			accum[k] = &scoreAccum{}
		}
		accum[k].sum += float64(r.Final)
		accum[k].count++
	}

	checkers := make([]string, 0, len(checkerSet))
	for c := range checkerSet {
		checkers = append(checkers, c)
	}
	sort.Strings(checkers)

	// Build per-model scores.
	modelSet := map[string]bool{}
	for k := range accum {
		modelSet[k.model] = true
	}

	models := make([]ModelScore, 0, len(modelSet))
	for m := range modelSet {
		ms := ModelScore{
			Model:         m,
			CheckerScores: map[string]float64{},
		}
		var totalSum float64
		var totalCount int
		for _, c := range checkers {
			k := key{m, c}
			if a, ok := accum[k]; ok {
				avg := a.sum / float64(a.count)
				ms.CheckerScores[c] = math.Round(avg*100) / 100
				totalSum += avg
				totalCount++
				ms.RunCount += a.count
			}
		}
		if totalCount > 0 {
			ms.OverallScore = math.Round(totalSum/float64(totalCount)*100) / 100
		}
		models = append(models, ms)
	}

	sort.Slice(models, func(i, j int) bool {
		if models[i].OverallScore != models[j].OverallScore {
			return models[i].OverallScore > models[j].OverallScore
		}
		return models[i].Model < models[j].Model
	})
	for i := range models {
		models[i].Rank = i + 1
	}

	return Leaderboard{Models: models, Checkers: checkers}
}

func severityScore(sev model.Severity) float64 {
	switch sev {
	case model.SevCritical:
		return 4.0
	case model.SevHigh:
		return 3.0
	case model.SevMedium:
		return 2.0
	case model.SevLow:
		return 1.0
	default:
		return 0.0
	}
}

func FormatText(lb Leaderboard) string {
	if len(lb.Models) == 0 {
		return "No models found in run data.\n"
	}

	var b strings.Builder
	b.WriteString("Model Leaderboard\n")
	b.WriteString(strings.Repeat("=", 60) + "\n\n")

	// Header
	fmt.Fprintf(&b, "%-4s %-25s %-10s %-8s", "Rank", "Model", "Score", "Runs")
	for _, c := range lb.Checkers {
		name := c
		if len(name) > 10 {
			name = name[:10]
		}
		fmt.Fprintf(&b, " %-10s", name)
	}
	b.WriteString("\n")
	b.WriteString(strings.Repeat("-", 48+11*len(lb.Checkers)) + "\n")

	for _, m := range lb.Models {
		fmt.Fprintf(&b, "%-4d %-25s %-10.2f %-8d", m.Rank, m.Model, m.OverallScore, m.RunCount)
		for _, c := range lb.Checkers {
			if s, ok := m.CheckerScores[c]; ok {
				fmt.Fprintf(&b, " %-10.2f", s)
			} else {
				fmt.Fprintf(&b, " %-10s", "-")
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

func FilterByChecker(lb Leaderboard, checker string) Leaderboard {
	if checker == "" {
		return lb
	}
	found := false
	for _, c := range lb.Checkers {
		if c == checker {
			found = true
			break
		}
	}
	if !found {
		return Leaderboard{Checkers: []string{checker}}
	}

	filtered := make([]ModelScore, 0, len(lb.Models))
	for _, m := range lb.Models {
		if s, ok := m.CheckerScores[checker]; ok {
			filtered = append(filtered, ModelScore{
				Model:         m.Model,
				OverallScore:  s,
				CheckerScores: map[string]float64{checker: s},
				RunCount:      m.RunCount,
			})
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].OverallScore != filtered[j].OverallScore {
			return filtered[i].OverallScore > filtered[j].OverallScore
		}
		return filtered[i].Model < filtered[j].Model
	})
	for i := range filtered {
		filtered[i].Rank = i + 1
	}
	return Leaderboard{Models: filtered, Checkers: []string{checker}}
}

func TopN(lb Leaderboard, n int) Leaderboard {
	if n <= 0 || n >= len(lb.Models) {
		return lb
	}
	return Leaderboard{Models: lb.Models[:n], Checkers: lb.Checkers}
}
