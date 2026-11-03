package proactivity

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type ProactivityDetail struct {
	WarningCount     int     `json:"warning_count"`
	PreventionCount  int     `json:"prevention_count"`
	AlternativeCount int     `json:"alternative_count"`
	FutureLookCount  int     `json:"future_look_count"`
	ReactiveCount    int     `json:"reactive_count"`
	WordCount        int     `json:"word_count"`
	ProactiveRatio   float64 `json:"proactive_ratio"`
}

type ModelProactivity struct {
	Model       string            `json:"model"`
	Score       float64           `json:"score"`
	Grade       string            `json:"grade"`
	Detail      ProactivityDetail `json:"detail"`
	SampleCount int               `json:"sample_count"`
	LabCount    int               `json:"lab_count"`
}

type Report struct {
	Models     []ModelProactivity `json:"models"`
	BestModel  string             `json:"best_model"`
	WorstModel string             `json:"worst_model"`
	MeanScore  float64            `json:"mean_score"`
}

var warningPhrases = []string{
	"be careful", "watch out", "note that", "be aware",
	"warning:", "caution:", "important:", "beware of",
	"keep in mind", "don't forget", "make sure to",
	"be cautious", "pay attention to", "heads up",
}

var preventionPhrases = []string{
	"to prevent", "to avoid", "before you", "make sure",
	"ensure that", "always check", "never use",
	"protect against", "safeguard", "mitigate",
	"to reduce the risk", "as a precaution", "proactively",
	"before deploying", "before running",
}

var alternativePhrases = []string{
	"alternatively", "another approach", "you could also",
	"a better approach", "instead of", "rather than",
	"consider using", "you might prefer", "an alternative",
	"a safer option", "a more secure", "if that doesn't work",
}

var futureLookPhrases = []string{
	"in the future", "going forward", "long term",
	"when you scale", "as your system grows", "eventually",
	"plan for", "prepare for", "anticipate",
	"down the road", "in production", "at scale",
	"when traffic increases", "as usage grows",
}

var reactivePhrases = []string{
	"if it breaks", "when it fails", "after the crash",
	"once the error occurs", "after the damage",
	"to fix the issue", "to resolve the problem",
	"as a workaround", "the fix is to", "to recover from",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func AnalyzeText(text string) (float64, ProactivityDetail) {
	words := strings.Fields(text)
	wordCount := len(words)

	if wordCount == 0 {
		return 50.0, ProactivityDetail{}
	}

	warnings := countPhrases(text, warningPhrases)
	preventions := countPhrases(text, preventionPhrases)
	alternatives := countPhrases(text, alternativePhrases)
	futureLooks := countPhrases(text, futureLookPhrases)
	reactives := countPhrases(text, reactivePhrases)

	positiveSignals := float64(warnings + preventions + alternatives + futureLooks)
	negativeSignals := float64(reactives)
	totalSignals := positiveSignals + negativeSignals

	proactiveRatio := 0.5
	if totalSignals > 0 {
		proactiveRatio = positiveSignals / totalSignals
	}

	score := 50.0
	score += float64(warnings) * 3.0
	score += float64(preventions) * 3.5
	score += float64(alternatives) * 3.0
	score += float64(futureLooks) * 2.5
	score -= float64(reactives) * 5.0

	density := positiveSignals / float64(wordCount) * 100
	score += density * 2.0

	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	detail := ProactivityDetail{
		WarningCount:     warnings,
		PreventionCount:  preventions,
		AlternativeCount: alternatives,
		FutureLookCount:  futureLooks,
		ReactiveCount:    reactives,
		WordCount:        wordCount,
		ProactiveRatio:   math.Round(proactiveRatio*100) / 100,
	}

	return math.Round(score*10) / 10, detail
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

func Analyze(findings []model.Finding) Report {
	if len(findings) == 0 {
		return Report{}
	}

	modelEntries := map[string][]model.Finding{}
	for _, f := range findings {
		mn := f.Model.Model
		if mn == "" {
			mn = "unknown"
		}
		modelEntries[mn] = append(modelEntries[mn], f)
	}

	var models []ModelProactivity
	for mn, entries := range modelEntries {
		var totalScore float64
		var aggDetail ProactivityDetail
		labs := map[string]bool{}

		for _, e := range entries {
			labs[e.Lab] = true
			score, detail := AnalyzeText(e.Summary)
			totalScore += score
			aggDetail.WarningCount += detail.WarningCount
			aggDetail.PreventionCount += detail.PreventionCount
			aggDetail.AlternativeCount += detail.AlternativeCount
			aggDetail.FutureLookCount += detail.FutureLookCount
			aggDetail.ReactiveCount += detail.ReactiveCount
			aggDetail.WordCount += detail.WordCount
			aggDetail.ProactiveRatio += detail.ProactiveRatio
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		aggDetail.ProactiveRatio = math.Round(aggDetail.ProactiveRatio/n*100) / 100

		models = append(models, ModelProactivity{
			Model:       mn,
			Score:       math.Round(avgScore*10) / 10,
			Grade:       scoreToGrade(avgScore),
			Detail:      aggDetail,
			SampleCount: len(entries),
			LabCount:    len(labs),
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].Score > models[j].Score
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.Score
	}

	r := Report{Models: models}
	if len(models) > 0 {
		r.BestModel = models[0].Model
		r.WorstModel = models[len(models)-1].Model
		r.MeanScore = math.Round(sumScore/float64(len(models))*10) / 10
	}
	return r
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Response Proactivity Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No proactivity data available.\n")
		return b.String()
	}

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s\n", m.Model)
		fmt.Fprintf(&b, "    Score: %.1f  Grade: %s\n", m.Score, m.Grade)
		fmt.Fprintf(&b, "    Warnings: %d  Preventions: %d  Alternatives: %d\n",
			m.Detail.WarningCount, m.Detail.PreventionCount, m.Detail.AlternativeCount)
		fmt.Fprintf(&b, "    Future: %d  Reactive: %d  Ratio: %.2f\n",
			m.Detail.FutureLookCount, m.Detail.ReactiveCount, m.Detail.ProactiveRatio)
		fmt.Fprintf(&b, "    Samples: %d  Labs: %d\n\n",
			m.SampleCount, m.LabCount)
	}

	fmt.Fprintf(&b, "  Best: %s  Worst: %s  Mean: %.1f\n",
		r.BestModel, r.WorstModel, r.MeanScore)

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data)
}
