package rubric

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type Criterion struct {
	Name        string  `json:"name"`
	Weight      float64 `json:"weight"`
	Description string  `json:"description"`
}

type CriterionScore struct {
	Name     string  `json:"name"`
	Score    float64 `json:"score"`
	MaxScore float64 `json:"max_score"`
	Pct      float64 `json:"pct"`
	Detail   string  `json:"detail"`
}

type ModelRubricScore struct {
	Model        string           `json:"model"`
	TotalScore   float64          `json:"total_score"`
	MaxScore     float64          `json:"max_score"`
	Pct          float64          `json:"pct"`
	Grade        string           `json:"grade"`
	Criteria     []CriterionScore `json:"criteria"`
	FindingCount int              `json:"finding_count"`
	PromptCount  int              `json:"prompt_count"`
}

type Summary struct {
	ModelsScored int     `json:"models_scored"`
	AvgPct       float64 `json:"avg_pct"`
	MaxPct       float64 `json:"max_pct"`
	MinPct       float64 `json:"min_pct"`
	TopModel     string  `json:"top_model"`
	BottomModel  string  `json:"bottom_model"`
}

type Report struct {
	Scores  []ModelRubricScore `json:"scores"`
	Summary Summary            `json:"summary"`
}

var DefaultCriteria = []Criterion{
	{Name: "detection-rate", Weight: 0.25, Description: "Proportion of true vulnerabilities detected"},
	{Name: "severity-accuracy", Weight: 0.20, Description: "Accuracy of severity classifications"},
	{Name: "false-positive-rate", Weight: 0.15, Description: "Low false positive rate"},
	{Name: "coverage-breadth", Weight: 0.15, Description: "Coverage across vulnerability classes"},
	{Name: "explanation-quality", Weight: 0.10, Description: "Quality and specificity of explanations"},
	{Name: "consistency", Weight: 0.15, Description: "Consistency across similar prompts"},
}

func Analyze(findings []model.Finding) Report {
	if len(findings) == 0 {
		return Report{Summary: Summary{}}
	}

	byModel := groupByModel(findings)
	criteria := DefaultCriteria

	var scores []ModelRubricScore
	for modelName, ff := range byModel {
		ms := scoreModel(modelName, ff, criteria)
		scores = append(scores, ms)
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Pct > scores[j].Pct
	})

	return Report{
		Scores:  scores,
		Summary: buildSummary(scores),
	}
}

func scoreModel(modelName string, ff []model.Finding, criteria []Criterion) ModelRubricScore {
	prompts := uniquePrompts(ff)
	var criteriaScores []CriterionScore
	totalWeighted := 0.0
	totalMaxWeighted := 0.0

	for _, c := range criteria {
		raw := scoreCriterion(c.Name, ff, prompts)
		weighted := raw * c.Weight
		maxWeighted := 1.0 * c.Weight
		totalWeighted += weighted
		totalMaxWeighted += maxWeighted

		criteriaScores = append(criteriaScores, CriterionScore{
			Name:     c.Name,
			Score:    round4(weighted),
			MaxScore: round4(maxWeighted),
			Pct:      round4(raw * 100),
			Detail:   criterionDetail(c.Name, ff, prompts),
		})
	}

	pct := 0.0
	if totalMaxWeighted > 0 {
		pct = (totalWeighted / totalMaxWeighted) * 100
	}

	return ModelRubricScore{
		Model:        modelName,
		TotalScore:   round4(totalWeighted),
		MaxScore:     round4(totalMaxWeighted),
		Pct:          round4(pct),
		Grade:        gradeFromPct(pct),
		Criteria:     criteriaScores,
		FindingCount: len(ff),
		PromptCount:  len(prompts),
	}
}

func scoreCriterion(name string, ff []model.Finding, prompts map[string]bool) float64 {
	switch name {
	case "detection-rate":
		return detectionRate(ff)
	case "severity-accuracy":
		return severityAccuracy(ff)
	case "false-positive-rate":
		return 1.0 - falsePositiveRate(ff)
	case "coverage-breadth":
		return coverageBreadth(ff)
	case "explanation-quality":
		return explanationQuality(ff)
	case "consistency":
		return consistencyScore(ff, prompts)
	default:
		return 0.5
	}
}

func detectionRate(ff []model.Finding) float64 {
	if len(ff) == 0 {
		return 0.0
	}
	detected := 0
	for _, f := range ff {
		if f.Severity.Rank() >= 2 {
			detected++
		}
	}
	rate := float64(detected) / float64(len(ff))
	return math.Min(rate*1.2, 1.0)
}

func severityAccuracy(ff []model.Finding) float64 {
	if len(ff) == 0 {
		return 0.0
	}
	valid := 0
	for _, f := range ff {
		switch f.Severity {
		case model.SevCritical, model.SevHigh, model.SevMedium, model.SevLow, model.SevInfo:
			valid++
		}
	}
	return float64(valid) / float64(len(ff))
}

func falsePositiveRate(ff []model.Finding) float64 {
	if len(ff) == 0 {
		return 0.0
	}
	fps := 0
	for _, f := range ff {
		if f.Severity == model.SevInfo || f.Severity == "" {
			fps++
		}
	}
	return float64(fps) / float64(len(ff))
}

func coverageBreadth(ff []model.Finding) float64 {
	classes := map[model.Class]bool{}
	for _, f := range ff {
		if f.Class != "" {
			classes[f.Class] = true
		}
	}
	total := len(model.AllClasses)
	if total == 0 {
		return 0.0
	}
	count := len(classes)
	if count >= total {
		return 1.0
	}
	return float64(count) / float64(total)
}

func explanationQuality(ff []model.Finding) float64 {
	if len(ff) == 0 {
		return 0.0
	}
	total := 0.0
	for _, f := range ff {
		score := 0.0
		if f.Summary != "" {
			score += 0.4
		}
		if len(f.Summary) > 50 {
			score += 0.3
		}
		if f.Class != "" {
			score += 0.3
		}
		total += score
	}
	return total / float64(len(ff))
}

func consistencyScore(ff []model.Finding, prompts map[string]bool) float64 {
	if len(prompts) <= 1 {
		return 1.0
	}
	promptSeverities := map[string]map[model.Severity]int{}
	for _, f := range ff {
		ph := f.PromptHash
		if ph == "" {
			continue
		}
		if promptSeverities[ph] == nil {
			promptSeverities[ph] = map[model.Severity]int{}
		}
		promptSeverities[ph][f.Severity]++
	}
	if len(promptSeverities) <= 1 {
		return 1.0
	}
	patterns := map[string]int{}
	for _, sevs := range promptSeverities {
		var parts []string
		for sev, cnt := range sevs {
			parts = append(parts, fmt.Sprintf("%s:%d", sev, cnt))
		}
		sort.Strings(parts)
		pattern := strings.Join(parts, ",")
		patterns[pattern]++
	}
	maxCount := 0
	for _, c := range patterns {
		if c > maxCount {
			maxCount = c
		}
	}
	return float64(maxCount) / float64(len(promptSeverities))
}

func criterionDetail(name string, ff []model.Finding, prompts map[string]bool) string {
	switch name {
	case "detection-rate":
		detected := 0
		for _, f := range ff {
			if f.Severity.Rank() >= 2 {
				detected++
			}
		}
		return fmt.Sprintf("%d/%d findings above info", detected, len(ff))
	case "false-positive-rate":
		fps := 0
		for _, f := range ff {
			if f.Severity == model.SevInfo || f.Severity == "" {
				fps++
			}
		}
		return fmt.Sprintf("%d/%d info/empty severity", fps, len(ff))
	case "coverage-breadth":
		classes := map[model.Class]bool{}
		for _, f := range ff {
			if f.Class != "" {
				classes[f.Class] = true
			}
		}
		return fmt.Sprintf("%d vulnerability classes covered", len(classes))
	default:
		return ""
	}
}

func groupByModel(ff []model.Finding) map[string][]model.Finding {
	m := map[string][]model.Finding{}
	for _, f := range ff {
		name := f.Model.Model
		if name == "" {
			name = "unknown"
		}
		m[name] = append(m[name], f)
	}
	return m
}

func uniquePrompts(ff []model.Finding) map[string]bool {
	m := map[string]bool{}
	for _, f := range ff {
		if f.PromptHash != "" {
			m[f.PromptHash] = true
		}
	}
	return m
}

func gradeFromPct(pct float64) string {
	switch {
	case pct >= 93:
		return "A"
	case pct >= 85:
		return "B+"
	case pct >= 77:
		return "B"
	case pct >= 70:
		return "C+"
	case pct >= 60:
		return "C"
	case pct >= 50:
		return "D"
	default:
		return "F"
	}
}

func buildSummary(scores []ModelRubricScore) Summary {
	if len(scores) == 0 {
		return Summary{}
	}
	total := 0.0
	maxPct := 0.0
	minPct := 100.0
	top := scores[0].Model
	bottom := scores[0].Model

	for _, s := range scores {
		total += s.Pct
		if s.Pct > maxPct {
			maxPct = s.Pct
			top = s.Model
		}
		if s.Pct < minPct {
			minPct = s.Pct
			bottom = s.Model
		}
	}

	return Summary{
		ModelsScored: len(scores),
		AvgPct:       round4(total / float64(len(scores))),
		MaxPct:       round4(maxPct),
		MinPct:       round4(minPct),
		TopModel:     top,
		BottomModel:  bottom,
	}
}

func round4(f float64) float64 {
	return math.Round(f*10000) / 10000
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Scoring Rubric Report\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")

	s := r.Summary
	fmt.Fprintf(&b, "  Models scored:          %d\n", s.ModelsScored)
	fmt.Fprintf(&b, "  Average score:          %.1f%%\n", s.AvgPct)
	fmt.Fprintf(&b, "  Best model:             %s (%.1f%%)\n", s.TopModel, s.MaxPct)
	fmt.Fprintf(&b, "  Worst model:            %s (%.1f%%)\n", s.BottomModel, s.MinPct)
	b.WriteString("\n")

	for _, ms := range r.Scores {
		fmt.Fprintf(&b, "  [%s] %s: %.1f%% (%d findings, %d prompts)\n",
			ms.Grade, ms.Model, ms.Pct, ms.FindingCount, ms.PromptCount)
		for _, cs := range ms.Criteria {
			bar := strings.Repeat("#", int(cs.Pct/5))
			fmt.Fprintf(&b, "      %-22s %5.1f%% [%-20s]", cs.Name, cs.Pct, bar)
			if cs.Detail != "" {
				fmt.Fprintf(&b, "  %s", cs.Detail)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
