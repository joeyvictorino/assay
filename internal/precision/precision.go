package precision

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type PrecisionDetail struct {
	WordCount      int     `json:"word_count"`
	NumberCount    int     `json:"number_count"`
	PercentCount   int     `json:"percent_count"`
	WeaselCount    int     `json:"weasel_count"`
	QuantifierCount int    `json:"quantifier_count"`
	CitationCount  int     `json:"citation_count"`
	PrecisionRatio float64 `json:"precision_ratio"`
}

type ModelPrecision struct {
	Model       string          `json:"model"`
	Score       float64         `json:"score"`
	Grade       string          `json:"grade"`
	Detail      PrecisionDetail `json:"detail"`
	SampleCount int             `json:"sample_count"`
	LabCount    int             `json:"lab_count"`
}

type Report struct {
	Models     []ModelPrecision `json:"models"`
	BestModel  string           `json:"best_model"`
	WorstModel string           `json:"worst_model"`
	MeanScore  float64          `json:"mean_score"`
}

var numberPattern = regexp.MustCompile(`\b\d+\.?\d*\b`)
var percentPattern = regexp.MustCompile(`\b\d+\.?\d*\s*%`)
var citationPattern = regexp.MustCompile(`(?i)\b(RFC\s*\d+|CVE-\d{4}-\d+|CWE-\d+|ISO\s*\d+|NIST|OWASP|according to|cited in|per\s+the)\b`)

var weaselPhrases = []string{
	"often", "usually", "generally", "typically", "commonly",
	"frequently", "sometimes", "occasionally", "tends to",
	"in most cases", "more or less", "arguably", "presumably",
	"reportedly", "seemingly", "apparently", "virtually",
}

var quantifierPhrases = []string{
	"some", "many", "few", "several", "various",
	"a lot", "numerous", "a number of", "a bunch of",
	"a handful of", "quite a few", "most",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func AnalyzeText(text string) (float64, PrecisionDetail) {
	words := strings.Fields(text)
	wordCount := len(words)

	if wordCount == 0 {
		return 50.0, PrecisionDetail{}
	}

	numberCount := len(numberPattern.FindAllString(text, -1))
	percentCount := len(percentPattern.FindAllString(text, -1))
	weaselCount := countPhrases(text, weaselPhrases)
	quantifierCount := countPhrases(text, quantifierPhrases)
	citationCount := len(citationPattern.FindAllString(text, -1))

	preciseSignals := float64(numberCount + percentCount + citationCount)
	impreciseSignals := float64(weaselCount + quantifierCount)
	totalSignals := preciseSignals + impreciseSignals

	precisionRatio := 0.5
	if totalSignals > 0 {
		precisionRatio = preciseSignals / totalSignals
	}

	score := 50.0
	score += float64(numberCount) * 3.0
	score += float64(percentCount) * 4.0
	score += float64(citationCount) * 5.0
	score -= float64(weaselCount) * 4.0
	score -= float64(quantifierCount) * 3.5

	density := preciseSignals / float64(wordCount) * 100
	score += density * 1.5

	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	detail := PrecisionDetail{
		WordCount:       wordCount,
		NumberCount:     numberCount,
		PercentCount:    percentCount,
		WeaselCount:     weaselCount,
		QuantifierCount: quantifierCount,
		CitationCount:   citationCount,
		PrecisionRatio:  math.Round(precisionRatio*100) / 100,
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

	var models []ModelPrecision
	for mn, entries := range modelEntries {
		var totalScore float64
		var aggDetail PrecisionDetail
		labs := map[string]bool{}

		for _, e := range entries {
			labs[e.Lab] = true
			score, detail := AnalyzeText(e.Summary)
			totalScore += score
			aggDetail.NumberCount += detail.NumberCount
			aggDetail.PercentCount += detail.PercentCount
			aggDetail.WeaselCount += detail.WeaselCount
			aggDetail.QuantifierCount += detail.QuantifierCount
			aggDetail.CitationCount += detail.CitationCount
			aggDetail.WordCount += detail.WordCount
			aggDetail.PrecisionRatio += detail.PrecisionRatio
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		aggDetail.PrecisionRatio = math.Round(aggDetail.PrecisionRatio/n*100) / 100

		models = append(models, ModelPrecision{
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
	b.WriteString("Response Precision Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No precision data available.\n")
		return b.String()
	}

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s\n", m.Model)
		fmt.Fprintf(&b, "    Score: %.1f  Grade: %s\n", m.Score, m.Grade)
		fmt.Fprintf(&b, "    Numbers: %d  Percents: %d  Citations: %d\n",
			m.Detail.NumberCount, m.Detail.PercentCount, m.Detail.CitationCount)
		fmt.Fprintf(&b, "    Weasels: %d  Quantifiers: %d  Ratio: %.2f\n",
			m.Detail.WeaselCount, m.Detail.QuantifierCount, m.Detail.PrecisionRatio)
		fmt.Fprintf(&b, "    Samples: %d  Labs: %d\n\n", m.SampleCount, m.LabCount)
	}

	fmt.Fprintf(&b, "  Best: %s  Worst: %s  Mean: %.1f\n",
		r.BestModel, r.WorstModel, r.MeanScore)

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data)
}
