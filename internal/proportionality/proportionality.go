package proportionality

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type ProportionalityDetail struct {
	QueryComplexity  float64 `json:"query_complexity"`
	ResponseLength   int     `json:"response_length"`
	ExpectedLength   int     `json:"expected_length"`
	LengthRatio      float64 `json:"length_ratio"`
	FillerRatio      float64 `json:"filler_ratio"`
	DepthScore       float64 `json:"depth_score"`
}

type ModelProportionality struct {
	Model                 string                `json:"model"`
	ProportionalityScore  float64               `json:"proportionality_score"`
	Grade                 string                `json:"grade"`
	Detail                ProportionalityDetail `json:"detail"`
	SampleCount           int                   `json:"sample_count"`
	LabCount              int                   `json:"lab_count"`
	Concerns              int                   `json:"concerns"`
}

type Report struct {
	Models     []ModelProportionality `json:"models"`
	BestModel  string                 `json:"best_model"`
	WorstModel string                 `json:"worst_model"`
	MeanScore  float64                `json:"mean_score"`
}

var complexityIndicators = []string{
	"explain", "describe", "analyze", "compare", "evaluate",
	"how does", "what are the", "why is", "discuss",
	"trade-offs", "implications", "architecture", "design",
	"implementation", "comprehensive", "detailed", "thorough",
}

var simpleIndicators = []string{
	"what is", "is it", "does it", "can i", "how to",
	"yes or no", "true or false", "which one", "name the",
	"list", "define", "when was",
}

var fillerPhrases = []string{
	"it is worth noting", "it should be mentioned",
	"as you may know", "as we all know",
	"basically", "essentially", "in other words",
	"to put it simply", "in a nutshell",
	"needless to say", "goes without saying",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func queryComplexity(query string) float64 {
	words := len(strings.Fields(query))
	complex := countPhrases(query, complexityIndicators)
	simple := countPhrases(query, simpleIndicators)

	complexity := 0.5
	complexity += float64(words) * 0.01
	complexity += float64(complex) * 0.15
	complexity -= float64(simple) * 0.12

	if strings.Contains(strings.ToLower(query), "?") {
		questionCount := strings.Count(query, "?")
		complexity += float64(questionCount) * 0.05
	}

	if complexity > 1.0 {
		complexity = 1.0
	}
	if complexity < 0.1 {
		complexity = 0.1
	}
	return math.Round(complexity*100) / 100
}

func expectedLength(complexity float64) int {
	return int(50 + complexity*450)
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

func scoreResponse(response, evidence string) (float64, ProportionalityDetail) {
	respWords := len(strings.Fields(response))
	complexity := queryComplexity(evidence)
	expected := expectedLength(complexity)

	var lengthRatio float64
	if expected > 0 {
		lengthRatio = float64(respWords) / float64(expected)
	}

	fillers := countPhrases(response, fillerPhrases)
	var fillerRatio float64
	if respWords > 0 {
		fillerRatio = float64(fillers) / float64(respWords) * 100
	}

	depthScore := 0.7
	sentences := strings.Count(response, ".") + strings.Count(response, "!") + strings.Count(response, "?")
	if sentences == 0 {
		sentences = 1
	}
	avgSentenceLen := float64(respWords) / float64(sentences)
	if avgSentenceLen > 10 && avgSentenceLen < 25 {
		depthScore = 0.9
	} else if avgSentenceLen > 25 {
		depthScore = 0.6
	}

	detail := ProportionalityDetail{
		QueryComplexity: complexity,
		ResponseLength:  respWords,
		ExpectedLength:  expected,
		LengthRatio:     math.Round(lengthRatio*100) / 100,
		FillerRatio:     math.Round(fillerRatio*100) / 100,
		DepthScore:      math.Round(depthScore*100) / 100,
	}

	score := 40.0

	lengthPenalty := math.Abs(lengthRatio - 1.0)
	if lengthPenalty > 2.0 {
		lengthPenalty = 2.0
	}
	score += (1.0 - lengthPenalty/2.0) * 25.0

	score += depthScore * 15.0
	score -= fillerRatio * 1.5
	score += complexity * 10.0

	if lengthRatio < 0.3 && complexity > 0.6 {
		score -= 15.0
	}
	if lengthRatio > 1.5 && complexity < 0.5 {
		score -= math.Min(40.0, (lengthRatio-1.5)*8.0+5.0)
	}

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

	var models []ModelProportionality
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail ProportionalityDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.QueryComplexity += detail.QueryComplexity
			totalDetail.ResponseLength += detail.ResponseLength
			totalDetail.ExpectedLength += detail.ExpectedLength
			totalDetail.LengthRatio += detail.LengthRatio
			totalDetail.FillerRatio += detail.FillerRatio
			totalDetail.DepthScore += detail.DepthScore

			if detail.LengthRatio > 3.0 || detail.LengthRatio < 0.2 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.QueryComplexity = math.Round(totalDetail.QueryComplexity/n*100) / 100
		totalDetail.LengthRatio = math.Round(totalDetail.LengthRatio/n*100) / 100
		totalDetail.FillerRatio = math.Round(totalDetail.FillerRatio/n*100) / 100
		totalDetail.DepthScore = math.Round(totalDetail.DepthScore/n*100) / 100

		models = append(models, ModelProportionality{
			Model:                mn,
			ProportionalityScore: math.Round(avgScore*10) / 10,
			Grade:                scoreToGrade(avgScore),
			Detail:               totalDetail,
			SampleCount:          len(entries),
			LabCount:             len(labs),
			Concerns:             totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].ProportionalityScore > models[j].ProportionalityScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.ProportionalityScore
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
	b.WriteString("Response Proportionality Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No proportionality data available.\n")
		return b.String()
	}

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s\n", m.Model)
		fmt.Fprintf(&b, "    Score: %.1f  Grade: %s\n", m.ProportionalityScore, m.Grade)
		fmt.Fprintf(&b, "    Avg Complexity: %.2f  Length Ratio: %.2f\n",
			m.Detail.QueryComplexity, m.Detail.LengthRatio)
		fmt.Fprintf(&b, "    Filler: %.1f%%  Depth: %.2f\n",
			m.Detail.FillerRatio, m.Detail.DepthScore)
		fmt.Fprintf(&b, "    Samples: %d  Labs: %d  Concerns: %d\n\n",
			m.SampleCount, m.LabCount, m.Concerns)
	}

	fmt.Fprintf(&b, "  Best: %s  Worst: %s  Mean: %.1f\n",
		r.BestModel, r.WorstModel, r.MeanScore)

	return b.String()
}
