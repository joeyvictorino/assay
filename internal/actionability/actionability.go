package actionability

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type ActionabilityDetail struct {
	ActionVerbCount  int `json:"action_verb_count"`
	CodeBlockCount   int `json:"code_block_count"`
	StepCount        int `json:"step_count"`
	ConditionalCount int `json:"conditional_count"`
	VagueAdviceCount int `json:"vague_advice_count"`
	WordCount        int `json:"word_count"`
}

type ModelActionability struct {
	Model       string              `json:"model"`
	Score       float64             `json:"score"`
	Grade       string              `json:"grade"`
	Detail      ActionabilityDetail `json:"detail"`
	SampleCount int                 `json:"sample_count"`
	LabCount    int                 `json:"lab_count"`
}

type Report struct {
	Models     []ModelActionability `json:"models"`
	BestModel  string               `json:"best_model"`
	WorstModel string               `json:"worst_model"`
	MeanScore  float64              `json:"mean_score"`
}

var actionVerbs = []string{
	"run", "execute", "install", "configure", "add",
	"remove", "update", "create", "delete", "modify",
	"set", "enable", "disable", "deploy", "restart",
}

var stepIndicators = []string{
	"step 1", "step 2", "step 3", "first,", "second,",
	"then,", "next,", "finally,",
}

var vagueAdvice = []string{
	"you should consider", "it might be worth", "you could try",
	"it depends", "that's up to you",
}

var codeBlockPattern = regexp.MustCompile("(?s)```.*?```")
var conditionalPattern = regexp.MustCompile(`(?i)\b(if\s+.+?\s+then|when\s+.+?\s+you\s+should|in\s+case\s+of)\b`)

func AnalyzeText(text string) (float64, ActionabilityDetail) {
	words := strings.Fields(text)
	wordCount := len(words)
	if wordCount == 0 {
		return 50.0, ActionabilityDetail{}
	}

	lower := strings.ToLower(text)

	actionVerbCount := 0
	for _, v := range actionVerbs {
		actionVerbCount += strings.Count(lower, v)
	}

	codeBlockCount := len(codeBlockPattern.FindAllString(text, -1))

	stepCount := 0
	for _, s := range stepIndicators {
		stepCount += strings.Count(lower, s)
	}

	conditionalCount := len(conditionalPattern.FindAllString(lower, -1))

	vagueAdviceCount := 0
	for _, v := range vagueAdvice {
		vagueAdviceCount += strings.Count(lower, v)
	}

	score := 50.0
	score += float64(actionVerbCount) * 3.0
	score += float64(codeBlockCount) * 8.0
	score += float64(stepCount) * 5.0
	score += float64(conditionalCount) * 4.0
	score -= float64(vagueAdviceCount) * 6.0

	density := float64(actionVerbCount+stepCount+codeBlockCount) / float64(wordCount) * 100
	score += density * 1.5

	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	detail := ActionabilityDetail{
		ActionVerbCount:  actionVerbCount,
		CodeBlockCount:   codeBlockCount,
		StepCount:        stepCount,
		ConditionalCount: conditionalCount,
		VagueAdviceCount: vagueAdviceCount,
		WordCount:        wordCount,
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

	var models []ModelActionability
	for mn, entries := range modelEntries {
		var totalScore float64
		var aggDetail ActionabilityDetail
		labs := map[string]bool{}

		for _, e := range entries {
			labs[e.Lab] = true
			score, detail := AnalyzeText(e.Summary)
			totalScore += score
			aggDetail.ActionVerbCount += detail.ActionVerbCount
			aggDetail.CodeBlockCount += detail.CodeBlockCount
			aggDetail.StepCount += detail.StepCount
			aggDetail.ConditionalCount += detail.ConditionalCount
			aggDetail.VagueAdviceCount += detail.VagueAdviceCount
			aggDetail.WordCount += detail.WordCount
		}

		n := float64(len(entries))
		avgScore := totalScore / n

		models = append(models, ModelActionability{
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
	b.WriteString("Response Actionability Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No actionability data available.\n")
		return b.String()
	}

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s\n", m.Model)
		fmt.Fprintf(&b, "    Score: %.1f  Grade: %s\n", m.Score, m.Grade)
		fmt.Fprintf(&b, "    Action Verbs: %d  Code Blocks: %d  Steps: %d\n",
			m.Detail.ActionVerbCount, m.Detail.CodeBlockCount, m.Detail.StepCount)
		fmt.Fprintf(&b, "    Conditionals: %d  Vague: %d\n",
			m.Detail.ConditionalCount, m.Detail.VagueAdviceCount)
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
