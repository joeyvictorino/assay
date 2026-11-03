package depth

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type DepthDetail struct {
	DetailCount    int     `json:"detail_count"`
	ExampleCount   int     `json:"example_count"`
	CaveatCount    int     `json:"caveat_count"`
	BreadthCount   int     `json:"breadth_count"`
	ShallowCount   int     `json:"shallow_count"`
	CodeRefCount   int     `json:"code_ref_count"`
	WordCount      int     `json:"word_count"`
	CoverageRatio  float64 `json:"coverage_ratio"`
}

type ModelDepth struct {
	Model       string      `json:"model"`
	Score       float64     `json:"score"`
	Grade       string      `json:"grade"`
	Detail      DepthDetail `json:"detail"`
	SampleCount int         `json:"sample_count"`
	LabCount    int         `json:"lab_count"`
}

type Report struct {
	Models     []ModelDepth `json:"models"`
	BestModel  string       `json:"best_model"`
	WorstModel string       `json:"worst_model"`
	MeanScore  float64      `json:"mean_score"`
}

var detailPhrases = []string{
	"specifically", "in detail", "more precisely",
	"breaking this down", "looking deeper", "examining closely",
	"the root cause", "under the hood", "at a lower level",
	"the mechanism is", "this works by", "internally",
	"the implementation", "the underlying", "fundamentally",
}

var examplePhrases = []string{
	"for example", "for instance", "such as",
	"consider the case", "to illustrate", "as an example",
	"e.g.", "one scenario", "a concrete case",
}

var caveatPhrases = []string{
	"however, note that", "one caveat", "be aware that",
	"an edge case", "the exception is", "keep in mind",
	"a limitation", "with the caveat", "important to note",
	"a subtle point", "a corner case", "watch out for",
}

var breadthPhrases = []string{
	"another approach", "alternatively", "a related concept",
	"in a different context", "from another angle", "by comparison",
	"a complementary", "on a related note", "additionally",
	"beyond this", "extending further", "broadening the scope",
}

var shallowPhrases = []string{
	"just do", "simply", "easy", "straightforward",
	"obvious", "clearly", "of course", "naturally",
	"trivial", "no-brainer", "self-explanatory",
}

var codeRefPattern = regexp.MustCompile("(`[^`]+`|\\b\\w+\\.\\w+\\(|\\b(func|class|def|var|const|type|struct|interface)\\b)")

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func countCodeRefs(text string) int {
	return len(codeRefPattern.FindAllString(text, -1))
}

func AnalyzeText(text string) (float64, DepthDetail) {
	words := strings.Fields(text)
	wordCount := len(words)
	if wordCount == 0 {
		return 50.0, DepthDetail{}
	}

	details := countPhrases(text, detailPhrases)
	examples := countPhrases(text, examplePhrases)
	caveats := countPhrases(text, caveatPhrases)
	breadth := countPhrases(text, breadthPhrases)
	shallow := countPhrases(text, shallowPhrases)
	codeRefs := countCodeRefs(text)

	positiveSignals := float64(details + examples + caveats + breadth + codeRefs)
	negativeSignals := float64(shallow)
	totalSignals := positiveSignals + negativeSignals

	coverageRatio := 0.5
	if totalSignals > 0 {
		coverageRatio = positiveSignals / totalSignals
	}

	score := 50.0
	score += float64(details) * 3.5
	score += float64(examples) * 3.0
	score += float64(caveats) * 4.0
	score += float64(breadth) * 2.5
	score += float64(codeRefs) * 2.0
	score -= float64(shallow) * 4.0

	density := positiveSignals / float64(wordCount) * 100
	score += density * 2.0

	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	detail := DepthDetail{
		DetailCount:   details,
		ExampleCount:  examples,
		CaveatCount:   caveats,
		BreadthCount:  breadth,
		ShallowCount:  shallow,
		CodeRefCount:  codeRefs,
		WordCount:     wordCount,
		CoverageRatio: math.Round(coverageRatio*100) / 100,
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

	var models []ModelDepth
	for mn, entries := range modelEntries {
		var totalScore float64
		var aggDetail DepthDetail
		labs := map[string]bool{}

		for _, e := range entries {
			labs[e.Lab] = true
			score, detail := AnalyzeText(e.Summary)
			totalScore += score
			aggDetail.DetailCount += detail.DetailCount
			aggDetail.ExampleCount += detail.ExampleCount
			aggDetail.CaveatCount += detail.CaveatCount
			aggDetail.BreadthCount += detail.BreadthCount
			aggDetail.ShallowCount += detail.ShallowCount
			aggDetail.CodeRefCount += detail.CodeRefCount
			aggDetail.WordCount += detail.WordCount
			aggDetail.CoverageRatio += detail.CoverageRatio
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		aggDetail.CoverageRatio = math.Round(aggDetail.CoverageRatio/n*100) / 100

		models = append(models, ModelDepth{
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
	b.WriteString("Response Depth Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No depth data available.\n")
		return b.String()
	}

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s\n", m.Model)
		fmt.Fprintf(&b, "    Score: %.1f  Grade: %s\n", m.Score, m.Grade)
		fmt.Fprintf(&b, "    Detail: %d  Examples: %d  Caveats: %d\n",
			m.Detail.DetailCount, m.Detail.ExampleCount, m.Detail.CaveatCount)
		fmt.Fprintf(&b, "    Breadth: %d  Shallow: %d  Code Refs: %d\n",
			m.Detail.BreadthCount, m.Detail.ShallowCount, m.Detail.CodeRefCount)
		fmt.Fprintf(&b, "    Coverage: %.2f  Samples: %d  Labs: %d\n\n",
			m.Detail.CoverageRatio, m.SampleCount, m.LabCount)
	}

	fmt.Fprintf(&b, "  Best: %s  Worst: %s  Mean: %.1f\n",
		r.BestModel, r.WorstModel, r.MeanScore)

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data)
}
