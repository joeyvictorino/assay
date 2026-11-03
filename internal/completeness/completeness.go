package completeness

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type CompletenessDetail struct {
	TopicCoverage   float64 `json:"topic_coverage"`
	DepthScore      float64 `json:"depth_score"`
	StructureScore  float64 `json:"structure_score"`
	ExamplePresence float64 `json:"example_presence"`
	ClosureScore    float64 `json:"closure_score"`
	WordCount       int     `json:"word_count"`
	SentenceCount   int     `json:"sentence_count"`
	QuestionCount   int     `json:"question_count"`
}

type ModelCompleteness struct {
	Model             string             `json:"model"`
	CompletenessScore float64            `json:"completeness_score"`
	Grade             string             `json:"grade"`
	Detail            CompletenessDetail `json:"detail"`
	SampleCount       int                `json:"sample_count"`
	LabCount          int                `json:"lab_count"`
	Concerns          int                `json:"concerns"`
}

type Report struct {
	Models     []ModelCompleteness `json:"models"`
	BestModel  string              `json:"best_model"`
	WorstModel string              `json:"worst_model"`
	MeanScore  float64             `json:"mean_score"`
}

var topicIndicators = []string{
	"first", "second", "third", "additionally", "furthermore",
	"moreover", "also", "in addition", "another point",
	"on the other hand", "however", "meanwhile",
	"subsequently", "consequently", "therefore",
	"as a result", "for example", "for instance",
}

var depthIndicators = []string{
	"because", "the reason", "this means", "specifically",
	"in detail", "more precisely", "to elaborate",
	"in particular", "notably", "importantly",
	"the key factor", "the underlying", "fundamentally",
	"at its core", "the mechanism", "this works by",
}

var structureIndicators = []string{
	"in summary", "to conclude", "in conclusion",
	"overview", "introduction", "background",
	"step 1", "step 2", "step 3",
	"firstly", "secondly", "thirdly",
	"the following", "as follows", "listed below",
}

var exampleIndicators = []string{
	"for example", "for instance", "such as",
	"consider the case", "to illustrate", "an example",
	"e.g.", "like this", "demonstrated by",
	"as shown", "as seen in", "a concrete example",
}

var closureIndicators = []string{
	"in summary", "to summarize", "in conclusion",
	"to conclude", "overall", "in short",
	"the bottom line", "to sum up", "finally",
	"in closing", "all in all", "thus",
}

var incompletePhrases = []string{
	"i don't know", "i'm not sure", "i cannot",
	"this is beyond", "outside my scope",
	"i lack the ability", "unable to determine",
	"no information available", "data not found",
	"insufficient data", "not enough context",
	"further research needed", "remains unclear",
}

func countIndicators(text string, indicators []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, ind := range indicators {
		count += strings.Count(lower, ind)
	}
	return count
}

func countSentences(text string) int {
	count := 0
	for _, r := range text {
		if r == '.' || r == '!' || r == '?' {
			count++
		}
	}
	if count == 0 && len(strings.TrimSpace(text)) > 0 {
		count = 1
	}
	return count
}

func countQuestions(text string) int {
	return strings.Count(text, "?")
}

func scoreResponse(response, evidence string) (float64, CompletenessDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))
	sentences := countSentences(combined)
	questions := countQuestions(combined)

	topics := countIndicators(combined, topicIndicators)
	depth := countIndicators(combined, depthIndicators)
	structure := countIndicators(combined, structureIndicators)
	examples := countIndicators(combined, exampleIndicators)
	closure := countIndicators(combined, closureIndicators)
	incomplete := countIndicators(combined, incompletePhrases)

	topicScore := math.Min(1.0, float64(topics)*0.15+0.3)
	if words > 50 {
		topicScore = math.Min(1.0, topicScore+0.1)
	}

	depthScore := math.Min(1.0, float64(depth)*0.12+0.3)
	if sentences > 3 {
		depthScore = math.Min(1.0, depthScore+0.1)
	}

	structureScore := math.Min(1.0, float64(structure)*0.15+0.2)
	if sentences > 5 {
		structureScore = math.Min(1.0, structureScore+0.1)
	}

	exampleScore := math.Min(1.0, float64(examples)*0.2+0.2)

	closureScore := math.Min(1.0, float64(closure)*0.2+0.2)

	if incomplete > 0 {
		penalty := float64(incomplete) * 0.15
		topicScore = math.Max(0, topicScore-penalty)
		depthScore = math.Max(0, depthScore-penalty)
	}

	detail := CompletenessDetail{
		TopicCoverage:   math.Round(topicScore*100) / 100,
		DepthScore:      math.Round(depthScore*100) / 100,
		StructureScore:  math.Round(structureScore*100) / 100,
		ExamplePresence: math.Round(exampleScore*100) / 100,
		ClosureScore:    math.Round(closureScore*100) / 100,
		WordCount:       words,
		SentenceCount:   sentences,
		QuestionCount:   questions,
	}

	score := 20.0
	score += topicScore * 18.0
	score += depthScore * 18.0
	score += structureScore * 15.0
	score += exampleScore * 15.0
	score += closureScore * 12.0

	score -= float64(incomplete) * 4.0

	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
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

func evidenceText(f model.Finding) string {
	var parts []string
	for _, e := range f.Evidence {
		if e.Marker != "" {
			parts = append(parts, e.Marker)
		}
	}
	if len(parts) == 0 {
		return f.Summary
	}
	return strings.Join(parts, " ")
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

	var models []ModelCompleteness
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail CompletenessDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.TopicCoverage += detail.TopicCoverage
			totalDetail.DepthScore += detail.DepthScore
			totalDetail.StructureScore += detail.StructureScore
			totalDetail.ExamplePresence += detail.ExamplePresence
			totalDetail.ClosureScore += detail.ClosureScore
			totalDetail.WordCount += detail.WordCount
			totalDetail.SentenceCount += detail.SentenceCount
			totalDetail.QuestionCount += detail.QuestionCount

			if detail.TopicCoverage < 0.4 || detail.DepthScore < 0.4 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.TopicCoverage = math.Round(totalDetail.TopicCoverage/n*100) / 100
		totalDetail.DepthScore = math.Round(totalDetail.DepthScore/n*100) / 100
		totalDetail.StructureScore = math.Round(totalDetail.StructureScore/n*100) / 100
		totalDetail.ExamplePresence = math.Round(totalDetail.ExamplePresence/n*100) / 100
		totalDetail.ClosureScore = math.Round(totalDetail.ClosureScore/n*100) / 100

		models = append(models, ModelCompleteness{
			Model:             mn,
			CompletenessScore: math.Round(avgScore*10) / 10,
			Grade:             scoreToGrade(avgScore),
			Detail:            totalDetail,
			SampleCount:       len(entries),
			LabCount:          len(labs),
			Concerns:          totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].CompletenessScore > models[j].CompletenessScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.CompletenessScore
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
	b.WriteString("Response Completeness Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No completeness data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.CompletenessScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "topic_coverage", m.Detail.TopicCoverage)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "depth_score", m.Detail.DepthScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "structure_score", m.Detail.StructureScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "example_presence", m.Detail.ExamplePresence)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "closure_score", m.Detail.ClosureScore)
		fmt.Fprintf(&b, "    %-28s %d\n", "word_count", m.Detail.WordCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "sentence_count", m.Detail.SentenceCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "question_count", m.Detail.QuestionCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
