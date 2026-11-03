package empowerment

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type EmpowermentDetail struct {
	ActionableCount  int     `json:"actionable_count"`
	StepCount        int     `json:"step_count"`
	CodeRefCount     int     `json:"code_ref_count"`
	SelfHelpCount    int     `json:"self_help_count"`
	DependencyCount  int     `json:"dependency_count"`
	WordCount        int     `json:"word_count"`
	EmpowermentRatio float64 `json:"empowerment_ratio"`
}

type ModelEmpowerment struct {
	Model       string            `json:"model"`
	Score       float64           `json:"score"`
	Grade       string            `json:"grade"`
	Detail      EmpowermentDetail `json:"detail"`
	SampleCount int               `json:"sample_count"`
	LabCount    int               `json:"lab_count"`
}

type Report struct {
	Models     []ModelEmpowerment `json:"models"`
	BestModel  string             `json:"best_model"`
	WorstModel string             `json:"worst_model"`
	MeanScore  float64            `json:"mean_score"`
}

var actionablePhrases = []string{
	"you can", "try running", "run the", "use the", "execute",
	"implement", "configure", "set up", "install", "create",
	"open the", "navigate to", "click on", "type in", "enter the",
	"modify the", "update the", "add the", "remove the", "replace",
}

var stepPhrases = []string{
	"step 1", "step 2", "step 3", "step 4", "step 5",
	"first,", "second,", "third,", "then,", "next,",
	"finally,", "after that", "once you", "start by", "begin with",
}

var selfHelpPhrases = []string{
	"documentation", "reference", "learn more", "for more details",
	"see the docs", "check the manual", "refer to", "read more",
	"official guide", "tutorial", "the docs explain",
}

var dependencyPhrases = []string{
	"contact support", "ask your admin", "you'll need help",
	"reach out to", "have someone", "get assistance",
	"request access from", "your team lead", "your manager",
	"submit a ticket", "open a support case",
}

var codeBlockPattern = regexp.MustCompile("`[^`]+`")

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func countCodeRefs(text string) int {
	return len(codeBlockPattern.FindAllString(text, -1))
}

func AnalyzeText(text string) (float64, EmpowermentDetail) {
	words := strings.Fields(text)
	wordCount := len(words)

	if wordCount == 0 {
		return 50.0, EmpowermentDetail{}
	}

	actionable := countPhrases(text, actionablePhrases)
	steps := countPhrases(text, stepPhrases)
	codeRefs := countCodeRefs(text)
	selfHelp := countPhrases(text, selfHelpPhrases)
	dependency := countPhrases(text, dependencyPhrases)

	positiveSignals := float64(actionable + steps + codeRefs + selfHelp)
	negativeSignals := float64(dependency)
	totalSignals := positiveSignals + negativeSignals

	empowermentRatio := 0.5
	if totalSignals > 0 {
		empowermentRatio = positiveSignals / totalSignals
	}

	score := 50.0
	score += float64(actionable) * 3.0
	score += float64(steps) * 4.0
	score += float64(codeRefs) * 2.5
	score += float64(selfHelp) * 3.0
	score -= float64(dependency) * 6.0

	density := positiveSignals / float64(wordCount) * 100
	score += density * 1.5

	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	detail := EmpowermentDetail{
		ActionableCount:  actionable,
		StepCount:        steps,
		CodeRefCount:     codeRefs,
		SelfHelpCount:    selfHelp,
		DependencyCount:  dependency,
		WordCount:        wordCount,
		EmpowermentRatio: math.Round(empowermentRatio*100) / 100,
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

	var models []ModelEmpowerment
	for mn, entries := range modelEntries {
		var totalScore float64
		var aggDetail EmpowermentDetail
		labs := map[string]bool{}

		for _, e := range entries {
			labs[e.Lab] = true
			score, detail := AnalyzeText(e.Summary)
			totalScore += score
			aggDetail.ActionableCount += detail.ActionableCount
			aggDetail.StepCount += detail.StepCount
			aggDetail.CodeRefCount += detail.CodeRefCount
			aggDetail.SelfHelpCount += detail.SelfHelpCount
			aggDetail.DependencyCount += detail.DependencyCount
			aggDetail.WordCount += detail.WordCount
			aggDetail.EmpowermentRatio += detail.EmpowermentRatio
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		aggDetail.EmpowermentRatio = math.Round(aggDetail.EmpowermentRatio/n*100) / 100

		models = append(models, ModelEmpowerment{
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
	b.WriteString("Response Empowerment Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No empowerment data available.\n")
		return b.String()
	}

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s\n", m.Model)
		fmt.Fprintf(&b, "    Score: %.1f  Grade: %s\n", m.Score, m.Grade)
		fmt.Fprintf(&b, "    Actionable: %d  Steps: %d  Code Refs: %d\n",
			m.Detail.ActionableCount, m.Detail.StepCount, m.Detail.CodeRefCount)
		fmt.Fprintf(&b, "    Self-Help: %d  Dependencies: %d  Ratio: %.2f\n",
			m.Detail.SelfHelpCount, m.Detail.DependencyCount, m.Detail.EmpowermentRatio)
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
