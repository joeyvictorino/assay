package structure

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type StructureDetail struct {
	HeadingCount    int     `json:"heading_count"`
	ListCount       int     `json:"list_count"`
	CodeBlockCount  int     `json:"code_block_count"`
	SectionCount    int     `json:"section_count"`
	TransitionCount int     `json:"transition_count"`
	RunOnCount      int     `json:"run_on_count"`
	WordCount       int     `json:"word_count"`
	StructureRatio  float64 `json:"structure_ratio"`
}

type ModelStructure struct {
	Model       string          `json:"model"`
	Score       float64         `json:"score"`
	Grade       string          `json:"grade"`
	Detail      StructureDetail `json:"detail"`
	SampleCount int             `json:"sample_count"`
	LabCount    int             `json:"lab_count"`
}

type Report struct {
	Models     []ModelStructure `json:"models"`
	BestModel  string           `json:"best_model"`
	WorstModel string           `json:"worst_model"`
	MeanScore  float64          `json:"mean_score"`
}

var headingPatterns = regexp.MustCompile(`(?m)^#{1,6}\s+\S|^\*\*[^*]+\*\*:?\s*$`)

var listPatterns = regexp.MustCompile(`(?m)^[\s]*[-*+]\s+\S|^[\s]*\d+\.\s+\S`)

var codeBlockPattern = regexp.MustCompile("(?m)^```")

var sectionPhrases = []string{
	"step 1", "step 2", "step 3", "step 4",
	"first,", "second,", "third,", "finally,",
	"to begin", "next,", "then,", "lastly,",
	"in summary", "in conclusion", "to summarize",
	"overview:", "background:", "solution:", "approach:",
}

var transitionPhrases = []string{
	"however", "furthermore", "additionally", "moreover",
	"in contrast", "on the other hand", "nevertheless",
	"as a result", "for example", "specifically",
	"in particular", "to illustrate", "that said",
	"building on this", "with that in mind",
}

var runOnIndicators = []string{
	" and then and ", " and also and ", " but also but ",
	" which is which ", " that is that ",
}

func countHeadings(text string) int {
	return len(headingPatterns.FindAllString(text, -1))
}

func countListItems(text string) int {
	return len(listPatterns.FindAllString(text, -1))
}

func countCodeBlocks(text string) int {
	n := len(codeBlockPattern.FindAllString(text, -1)) / 2
	if n < 0 {
		n = 0
	}
	return n
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func AnalyzeText(text string) (float64, StructureDetail) {
	words := strings.Fields(text)
	wordCount := len(words)

	if wordCount == 0 {
		return 50.0, StructureDetail{}
	}

	headings := len(headingPatterns.FindAllString(text, -1))
	lists := len(listPatterns.FindAllString(text, -1))
	codeBlocks := len(codeBlockPattern.FindAllString(text, -1)) / 2
	if codeBlocks < 0 {
		codeBlocks = 0
	}
	sections := countPhrases(text, sectionPhrases)
	transitions := countPhrases(text, transitionPhrases)
	runOns := countPhrases(text, runOnIndicators)

	lines := strings.Split(text, "\n")
	longLineCount := 0
	for _, line := range lines {
		lineWords := strings.Fields(line)
		if len(lineWords) > 60 {
			longLineCount++
		}
	}

	positiveSignals := float64(headings + lists + codeBlocks + sections + transitions)
	negativeSignals := float64(runOns + longLineCount)
	totalSignals := positiveSignals + negativeSignals

	ratio := 0.5
	if totalSignals > 0 {
		ratio = positiveSignals / totalSignals
	}

	score := 50.0
	score += float64(headings) * 4.0
	score += float64(lists) * 2.5
	score += float64(codeBlocks) * 3.5
	score += float64(sections) * 3.0
	score += float64(transitions) * 2.0
	score -= float64(runOns) * 5.0
	score -= float64(longLineCount) * 3.0

	density := positiveSignals / float64(wordCount) * 100
	score += density * 2.0

	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	detail := StructureDetail{
		HeadingCount:    headings,
		ListCount:       lists,
		CodeBlockCount:  codeBlocks,
		SectionCount:    sections,
		TransitionCount: transitions,
		RunOnCount:      runOns + longLineCount,
		WordCount:       wordCount,
		StructureRatio:  math.Round(ratio*100) / 100,
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

	var models []ModelStructure
	for mn, entries := range modelEntries {
		var totalScore float64
		var aggDetail StructureDetail
		labs := map[string]bool{}

		for _, e := range entries {
			labs[e.Lab] = true
			score, detail := AnalyzeText(e.Summary)
			totalScore += score
			aggDetail.HeadingCount += detail.HeadingCount
			aggDetail.ListCount += detail.ListCount
			aggDetail.CodeBlockCount += detail.CodeBlockCount
			aggDetail.SectionCount += detail.SectionCount
			aggDetail.TransitionCount += detail.TransitionCount
			aggDetail.RunOnCount += detail.RunOnCount
			aggDetail.WordCount += detail.WordCount
			aggDetail.StructureRatio += detail.StructureRatio
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		aggDetail.StructureRatio = math.Round(aggDetail.StructureRatio/n*100) / 100

		models = append(models, ModelStructure{
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
	b.WriteString("Response Structure Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No structure data available.\n")
		return b.String()
	}

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s\n", m.Model)
		fmt.Fprintf(&b, "    Score: %.1f  Grade: %s\n", m.Score, m.Grade)
		fmt.Fprintf(&b, "    Headings: %d  Lists: %d  Code blocks: %d\n",
			m.Detail.HeadingCount, m.Detail.ListCount, m.Detail.CodeBlockCount)
		fmt.Fprintf(&b, "    Sections: %d  Transitions: %d  Run-ons: %d  Ratio: %.2f\n",
			m.Detail.SectionCount, m.Detail.TransitionCount, m.Detail.RunOnCount, m.Detail.StructureRatio)
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
