package modelhealth

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type Grade string

const (
	GradeA Grade = "A"
	GradeB Grade = "B"
	GradeC Grade = "C"
	GradeD Grade = "D"
	GradeF Grade = "F"
)

type Dimension struct {
	Name  string  `json:"name"`
	Score float64 `json:"score"`
	Grade Grade   `json:"grade"`
	Note  string  `json:"note,omitempty"`
}

type ModelHealth struct {
	Model      string      `json:"model"`
	Provider   string      `json:"provider"`
	Overall    float64     `json:"overall_score"`
	Grade      Grade       `json:"grade"`
	Dimensions []Dimension `json:"dimensions"`
	Flags      []string    `json:"flags,omitempty"`
}

type Report struct {
	RunID  string        `json:"run_id"`
	Models []ModelHealth `json:"models"`
}

func Analyze(rr model.RunReport) Report {
	var models []ModelHealth
	for _, mr := range rr.Models {
		mh := analyzeModel(mr, rr)
		models = append(models, mh)
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].Overall > models[j].Overall
	})
	return Report{RunID: rr.RunID, Models: models}
}

func analyzeModel(mr model.ModelResult, rr model.RunReport) ModelHealth {
	dims := []Dimension{
		tokenEfficiency(mr),
		latencyHealth(mr),
		refusalHealth(mr),
		failoverHealth(mr),
		findingQuality(mr),
		reflectionHealth(mr, rr),
	}

	var sum float64
	for _, d := range dims {
		sum += d.Score
	}
	overall := sum / float64(len(dims))

	var flags []string
	if mr.Refusals > 5 {
		flags = append(flags, "high-refusal-rate")
	}
	if mr.Failovers > 3 {
		flags = append(flags, "frequent-failovers")
	}
	if mr.Usage.CostUSD > rr.BudgetCapUSD*0.5 && rr.BudgetCapUSD > 0 {
		flags = append(flags, "high-cost-share")
	}
	if mr.P95MS > 30000 {
		flags = append(flags, "slow-p95")
	}

	return ModelHealth{
		Model:      mr.Ref.Model,
		Provider:   mr.Ref.Provider,
		Overall:    math.Round(overall*100) / 100,
		Grade:      scoreToGrade(overall),
		Dimensions: dims,
		Flags:      flags,
	}
}

func tokenEfficiency(mr model.ModelResult) Dimension {
	total := mr.Usage.InputTokens + mr.Usage.OutputTokens
	if total == 0 {
		return Dimension{Name: "token_efficiency", Score: 50, Grade: GradeC, Note: "no token data"}
	}
	cached := mr.Usage.CacheReadTokens
	cacheRatio := float64(cached) / float64(total)
	outputRatio := float64(mr.Usage.OutputTokens) / float64(total)

	score := 50.0
	score += cacheRatio * 30
	if outputRatio > 0.1 && outputRatio < 0.5 {
		score += 20
	} else if outputRatio <= 0.1 {
		score += 5
	} else {
		score += 10
	}

	score = clamp(score, 0, 100)
	note := fmt.Sprintf("cache_ratio=%.1f%% output_ratio=%.1f%%", cacheRatio*100, outputRatio*100)
	return Dimension{Name: "token_efficiency", Score: score, Grade: scoreToGrade(score), Note: note}
}

func latencyHealth(mr model.ModelResult) Dimension {
	if mr.P50MS == 0 && mr.P95MS == 0 {
		return Dimension{Name: "latency", Score: 50, Grade: GradeC, Note: "no latency data"}
	}
	score := 100.0
	if mr.P50MS > 5000 {
		score -= float64(mr.P50MS-5000) / 200
	}
	if mr.P95MS > 15000 {
		score -= float64(mr.P95MS-15000) / 500
	}
	if mr.P95MS > 0 && mr.P50MS > 0 {
		jitter := float64(mr.P95MS) / float64(mr.P50MS)
		if jitter > 5 {
			score -= (jitter - 5) * 3
		}
	}

	score = clamp(score, 0, 100)
	note := fmt.Sprintf("p50=%dms p95=%dms", mr.P50MS, mr.P95MS)
	return Dimension{Name: "latency", Score: score, Grade: scoreToGrade(score), Note: note}
}

func refusalHealth(mr model.ModelResult) Dimension {
	totalFindings := 0
	for _, n := range mr.Findings {
		totalFindings += n
	}
	total := totalFindings + mr.Refusals
	if total == 0 {
		return Dimension{Name: "refusal_rate", Score: 100, Grade: GradeA, Note: "no calls"}
	}
	rate := float64(mr.Refusals) / float64(total)
	score := 100 - rate*200
	score = clamp(score, 0, 100)
	note := fmt.Sprintf("%d refusals / %d total (%.1f%%)", mr.Refusals, total, rate*100)
	return Dimension{Name: "refusal_rate", Score: score, Grade: scoreToGrade(score), Note: note}
}

func failoverHealth(mr model.ModelResult) Dimension {
	if mr.Failovers == 0 {
		return Dimension{Name: "failover_rate", Score: 100, Grade: GradeA, Note: "no failovers"}
	}
	score := 100 - float64(mr.Failovers)*15
	score = clamp(score, 0, 100)
	note := fmt.Sprintf("%d failovers", mr.Failovers)
	return Dimension{Name: "failover_rate", Score: score, Grade: scoreToGrade(score), Note: note}
}

func findingQuality(mr model.ModelResult) Dimension {
	validated := mr.Findings[model.StateValidated]
	refuted := mr.Findings[model.StateRefuted]
	theorized := mr.Findings[model.StateTheorized]
	total := validated + refuted + theorized
	if total == 0 {
		return Dimension{Name: "finding_quality", Score: 50, Grade: GradeC, Note: "no findings"}
	}
	validatedRatio := float64(validated) / float64(total)
	refutedRatio := float64(refuted) / float64(total)
	score := validatedRatio*100 - refutedRatio*50
	if theorized > 0 && validated == 0 && refuted == 0 {
		score = 50
	}
	score = clamp(score, 0, 100)
	note := fmt.Sprintf("validated=%d refuted=%d theorized=%d", validated, refuted, theorized)
	return Dimension{Name: "finding_quality", Score: score, Grade: scoreToGrade(score), Note: note}
}

func reflectionHealth(mr model.ModelResult, rr model.RunReport) Dimension {
	var scores []int
	for _, r := range rr.Reflection {
		if r.Model == mr.Ref.Model && r.Scored && r.Final > 0 {
			scores = append(scores, r.Final)
		}
	}
	if len(scores) == 0 {
		return Dimension{Name: "reflection", Score: 50, Grade: GradeC, Note: "no reflection data"}
	}
	var sum int
	for _, s := range scores {
		sum += s
	}
	avg := float64(sum) / float64(len(scores))
	score := avg * 10
	score = clamp(score, 0, 100)
	note := fmt.Sprintf("avg_score=%.1f across %d labs", avg, len(scores))
	return Dimension{Name: "reflection", Score: score, Grade: scoreToGrade(score), Note: note}
}

func scoreToGrade(score float64) Grade {
	switch {
	case score >= 90:
		return GradeA
	case score >= 80:
		return GradeB
	case score >= 70:
		return GradeC
	case score >= 60:
		return GradeD
	default:
		return GradeF
	}
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func FormatText(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Model Health Report  (run %s)\n", r.RunID)
	b.WriteString(strings.Repeat("=", 60) + "\n\n")

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  %s (%s)  [%s]  overall=%.0f\n", m.Model, m.Provider, m.Grade, m.Overall)
		for _, d := range m.Dimensions {
			fmt.Fprintf(&b, "    %-20s %5.0f [%s]  %s\n", d.Name, d.Score, d.Grade, d.Note)
		}
		if len(m.Flags) > 0 {
			fmt.Fprintf(&b, "    flags: %s\n", strings.Join(m.Flags, ", "))
		}
		b.WriteString("\n")
	}
	return b.String()
}
