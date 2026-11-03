package objectivity

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type ObjectivityDetail struct {
	NeutralCount     int     `json:"neutral_count"`
	BiasCount        int     `json:"bias_count"`
	OpinionCount     int     `json:"opinion_count"`
	QualifiedCount   int     `json:"qualified_count"`
	EmotionalCount   int     `json:"emotional_count"`
	FactualCount     int     `json:"factual_count"`
	WordCount        int     `json:"word_count"`
	ObjectivityRatio float64 `json:"objectivity_ratio"`
}

type ModelObjectivity struct {
	Model       string            `json:"model"`
	Score       float64           `json:"score"`
	Grade       string            `json:"grade"`
	Detail      ObjectivityDetail `json:"detail"`
	SampleCount int               `json:"sample_count"`
	LabCount    int               `json:"lab_count"`
}

type Report struct {
	Models     []ModelObjectivity `json:"models"`
	BestModel  string             `json:"best_model"`
	WorstModel string             `json:"worst_model"`
	MeanScore  float64            `json:"mean_score"`
}

var biasPhrases = []string{
	"obviously", "clearly everyone", "no one would", "everyone knows",
	"it's obvious that", "without question", "undeniably",
	"any reasonable person", "only a fool", "the best approach is always",
	"never use", "always prefer", "you should always", "you must never",
}

var opinionPhrases = []string{
	"i personally", "in my opinion", "i feel that", "i love",
	"i hate", "i prefer", "my favorite", "i strongly believe",
	"honestly i think", "to be honest", "frankly",
}

var emotionalPhrases = []string{
	"amazing", "terrible", "awful", "fantastic", "horrible",
	"incredible", "disgusting", "absolutely perfect", "totally broken",
	"worst ever", "best ever", "love it", "hate it",
	"ridiculously", "insanely", "outrageously",
}

var neutralPhrases = []string{
	"according to", "based on", "the documentation states",
	"the specification defines", "as measured by", "data shows",
	"benchmarks indicate", "the standard recommends",
	"one approach is", "an alternative is", "there are tradeoffs",
	"it depends on", "the tradeoff is",
}

var qualifiedPhrases = []string{
	"in this context", "under these conditions", "depending on",
	"for this use case", "when the requirement is", "given that",
	"assuming", "provided that", "in scenarios where",
	"if the priority is", "when performance matters",
}

var factualPhrases = []string{
	"returns", "produces", "outputs", "computes",
	"the function", "the method", "the api", "the endpoint",
	"the result is", "the value is", "the type is",
	"takes", "accepts", "requires",
}

var numberPattern = regexp.MustCompile(`\b\d+(\.\d+)?\b`)

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func AnalyzeText(text string) (float64, ObjectivityDetail) {
	words := strings.Fields(text)
	wordCount := len(words)

	if wordCount == 0 {
		return 50.0, ObjectivityDetail{}
	}

	neutral := countPhrases(text, neutralPhrases)
	bias := countPhrases(text, biasPhrases)
	opinion := countPhrases(text, opinionPhrases)
	qualified := countPhrases(text, qualifiedPhrases)
	emotional := countPhrases(text, emotionalPhrases)
	factual := countPhrases(text, factualPhrases)
	numbers := len(numberPattern.FindAllString(text, -1))

	positiveSignals := float64(neutral + qualified + factual + numbers)
	negativeSignals := float64(bias + opinion + emotional)
	totalSignals := positiveSignals + negativeSignals

	ratio := 0.5
	if totalSignals > 0 {
		ratio = positiveSignals / totalSignals
	}

	score := 50.0
	score += float64(neutral) * 3.0
	score += float64(qualified) * 2.5
	score += float64(factual) * 2.0
	score += float64(numbers) * 1.0
	score -= float64(bias) * 5.0
	score -= float64(opinion) * 4.0
	score -= float64(emotional) * 3.5

	density := positiveSignals / float64(wordCount) * 100
	score += density * 1.5

	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	detail := ObjectivityDetail{
		NeutralCount:     neutral,
		BiasCount:        bias,
		OpinionCount:     opinion,
		QualifiedCount:   qualified,
		EmotionalCount:   emotional,
		FactualCount:     factual,
		WordCount:        wordCount,
		ObjectivityRatio: math.Round(ratio*100) / 100,
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

	var models []ModelObjectivity
	for mn, entries := range modelEntries {
		var totalScore float64
		var aggDetail ObjectivityDetail
		labs := map[string]bool{}

		for _, e := range entries {
			labs[e.Lab] = true
			score, detail := AnalyzeText(e.Summary)
			totalScore += score
			aggDetail.NeutralCount += detail.NeutralCount
			aggDetail.BiasCount += detail.BiasCount
			aggDetail.OpinionCount += detail.OpinionCount
			aggDetail.QualifiedCount += detail.QualifiedCount
			aggDetail.EmotionalCount += detail.EmotionalCount
			aggDetail.FactualCount += detail.FactualCount
			aggDetail.WordCount += detail.WordCount
			aggDetail.ObjectivityRatio += detail.ObjectivityRatio
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		aggDetail.ObjectivityRatio = math.Round(aggDetail.ObjectivityRatio/n*100) / 100

		models = append(models, ModelObjectivity{
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
	b.WriteString("Response Objectivity Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No objectivity data available.\n")
		return b.String()
	}

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s\n", m.Model)
		fmt.Fprintf(&b, "    Score: %.1f  Grade: %s\n", m.Score, m.Grade)
		fmt.Fprintf(&b, "    Neutral: %d  Qualified: %d  Factual: %d\n",
			m.Detail.NeutralCount, m.Detail.QualifiedCount, m.Detail.FactualCount)
		fmt.Fprintf(&b, "    Bias: %d  Opinion: %d  Emotional: %d  Ratio: %.2f\n",
			m.Detail.BiasCount, m.Detail.OpinionCount, m.Detail.EmotionalCount, m.Detail.ObjectivityRatio)
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
