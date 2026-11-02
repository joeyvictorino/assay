package dashdata

import (
	"sort"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
)

type DashboardData struct {
	GeneratedAt    string           `json:"generated_at"`
	RunSummary     RunSummary       `json:"run_summary"`
	ModelScores    []ModelScoreCard `json:"model_scores"`
	CheckerResults []CheckerCard    `json:"checker_results"`
	TopFindings    []FindingSummary `json:"top_findings"`
	RiskGauge      RiskGauge        `json:"risk_gauge"`
}

type RunSummary struct {
	TotalModels   int     `json:"total_models"`
	TotalCheckers int     `json:"total_checkers"`
	TotalFindings int     `json:"total_findings"`
	OverallScore  float64 `json:"overall_score"`
	Duration      string  `json:"duration"`
}

type ModelScoreCard struct {
	Model string  `json:"model"`
	Score float64 `json:"score"`
	Rank  int     `json:"rank"`
	Delta float64 `json:"delta"`
}

type CheckerCard struct {
	Checker  string  `json:"checker"`
	PassRate float64 `json:"pass_rate"`
	Findings int     `json:"findings"`
}

type FindingSummary struct {
	Checker  string `json:"checker"`
	Model    string `json:"model"`
	Severity string `json:"severity"`
	Summary  string `json:"summary"`
}

type RiskGauge struct {
	Score float64 `json:"score"`
	Level string  `json:"level"`
	Trend string  `json:"trend"`
}

func Generate(rr model.RunReport, prev *model.RunReport) DashboardData {
	d := DashboardData{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}

	d.RunSummary = buildRunSummary(rr)
	d.ModelScores = buildModelScores(rr, prev)
	d.CheckerResults = buildCheckerCards(rr)
	d.TopFindings = buildTopFindings(rr)
	d.RiskGauge = buildRiskGauge(rr, prev)

	return d
}

func buildRunSummary(rr model.RunReport) RunSummary {
	checkerSet := map[string]bool{}
	totalFindings := 0
	for _, ref := range rr.Reflection {
		checkerSet[ref.Reviewer] = true
		totalFindings += ref.Validated + ref.Refuted + ref.Theorized
	}

	dur := rr.Finished.Sub(rr.Started)
	durStr := dur.Truncate(time.Second).String()
	if dur == 0 {
		durStr = "0s"
	}

	return RunSummary{
		TotalModels:   len(rr.Models),
		TotalCheckers: len(checkerSet),
		TotalFindings: totalFindings,
		OverallScore:  rr.WeightedScore,
		Duration:      durStr,
	}
}

func buildModelScores(rr model.RunReport, prev *model.RunReport) []ModelScoreCard {
	type modelAcc struct {
		total float64
		count int
	}
	scores := map[string]*modelAcc{}
	for _, ref := range rr.Reflection {
		if !ref.Scored {
			continue
		}
		acc, ok := scores[ref.Model]
		if !ok {
			acc = &modelAcc{}
			scores[ref.Model] = acc
		}
		acc.total += float64(ref.Final)
		acc.count++
	}

	var prevScores map[string]float64
	if prev != nil {
		prevScores = map[string]float64{}
		prevAcc := map[string]*modelAcc{}
		for _, ref := range prev.Reflection {
			if !ref.Scored {
				continue
			}
			acc, ok := prevAcc[ref.Model]
			if !ok {
				acc = &modelAcc{}
				prevAcc[ref.Model] = acc
			}
			acc.total += float64(ref.Final)
			acc.count++
		}
		for m, acc := range prevAcc {
			if acc.count > 0 {
				prevScores[m] = acc.total / float64(acc.count)
			}
		}
	}

	var cards []ModelScoreCard
	for m, acc := range scores {
		avg := 0.0
		if acc.count > 0 {
			avg = acc.total / float64(acc.count)
		}
		card := ModelScoreCard{Model: m, Score: avg}
		if prevScores != nil {
			if ps, ok := prevScores[m]; ok {
				card.Delta = avg - ps
			}
		}
		cards = append(cards, card)
	}
	sort.Slice(cards, func(i, j int) bool {
		return cards[i].Score > cards[j].Score
	})
	for i := range cards {
		cards[i].Rank = i + 1
	}
	return cards
}

func buildCheckerCards(rr model.RunReport) []CheckerCard {
	type checkerAcc struct {
		total    int
		passed   int
		findings int
	}
	accs := map[string]*checkerAcc{}
	for _, ref := range rr.Reflection {
		key := ref.Reviewer
		if key == "" {
			key = ref.Model
		}
		acc, ok := accs[key]
		if !ok {
			acc = &checkerAcc{}
			accs[key] = acc
		}
		acc.total++
		if ref.Passed {
			acc.passed++
		}
		acc.findings += ref.Validated + ref.Refuted + ref.Theorized
	}

	var cards []CheckerCard
	for name, acc := range accs {
		pr := 0.0
		if acc.total > 0 {
			pr = float64(acc.passed) / float64(acc.total)
		}
		cards = append(cards, CheckerCard{
			Checker:  name,
			PassRate: pr,
			Findings: acc.findings,
		})
	}
	sort.Slice(cards, func(i, j int) bool {
		return cards[i].Checker < cards[j].Checker
	})
	return cards
}

func buildTopFindings(rr model.RunReport) []FindingSummary {
	var findings []FindingSummary
	for _, ref := range rr.Reflection {
		if ref.Validated > 0 {
			sev := "high"
			if ref.Final >= 8 {
				sev = "low"
			} else if ref.Final >= 5 {
				sev = "medium"
			}
			findings = append(findings, FindingSummary{
				Checker:  ref.Reviewer,
				Model:    ref.Model,
				Severity: sev,
				Summary:  ref.Lab,
			})
		}
	}
	if len(findings) > 10 {
		findings = findings[:10]
	}
	return findings
}

func buildRiskGauge(rr model.RunReport, prev *model.RunReport) RiskGauge {
	score := rr.WeightedScore * 10
	if score > 100 {
		score = 100
	}

	level := "low"
	switch {
	case score >= 75:
		level = "critical"
	case score >= 50:
		level = "high"
	case score >= 25:
		level = "medium"
	}

	trend := "stable"
	if prev != nil {
		prevScore := prev.WeightedScore * 10
		delta := score - prevScore
		if delta > 5 {
			trend = "degrading"
		} else if delta < -5 {
			trend = "improving"
		}
	}

	return RiskGauge{Score: score, Level: level, Trend: trend}
}
