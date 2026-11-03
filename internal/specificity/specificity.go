package specificity

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/joeyvictorino/assay/internal/model"
)

type SpecificityDetail struct {
	NumberCount    int     `json:"number_count"`
	ProperNounEst  int     `json:"proper_noun_est"`
	HedgeCount     int     `json:"hedge_count"`
	ConcreteCount  int     `json:"concrete_count"`
	VagueCount     int     `json:"vague_count"`
	WordCount      int     `json:"word_count"`
	SpecificRatio  float64 `json:"specific_ratio"`
}

type ModelSpecificity struct {
	Model       string            `json:"model"`
	Score       float64           `json:"score"`
	Grade       string            `json:"grade"`
	Detail      SpecificityDetail `json:"detail"`
	SampleCount int               `json:"sample_count"`
	LabCount    int               `json:"lab_count"`
}

type Report struct {
	Models     []ModelSpecificity `json:"models"`
	BestModel  string             `json:"best_model"`
	WorstModel string             `json:"worst_model"`
	MeanScore  float64            `json:"mean_score"`
}

var hedgePhrases = []string{
	"maybe", "perhaps", "possibly", "probably", "might",
	"could be", "it seems", "i think", "sort of", "kind of",
	"more or less", "in general", "tends to", "arguably",
	"somewhat", "approximately", "roughly", "likely",
}

var vaguePhrases = []string{
	"things", "stuff", "something", "somehow", "somewhere",
	"various", "several", "many", "some", "certain",
	"aspects", "elements", "factors", "issues", "a lot",
	"basically", "essentially", "generally",
}

var concretePhrases = []string{
	"specifically", "for example", "for instance", "such as",
	"in particular", "namely", "including", "e.g.",
	"according to", "based on", "as shown in",
	"the following", "step 1", "step 2", "step 3",
}

var numberPattern = regexp.MustCompile(`\b\d+\.?\d*\b`)

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func estimateProperNouns(text string) int {
	words := strings.Fields(text)
	count := 0
	for i, w := range words {
		if i == 0 {
			continue
		}
		cleaned := strings.TrimFunc(w, func(r rune) bool {
			return !unicode.IsLetter(r)
		})
		if len(cleaned) > 1 && unicode.IsUpper(rune(cleaned[0])) {
			count++
		}
	}
	return count
}

func AnalyzeText(text string) (float64, SpecificityDetail) {
	words := strings.Fields(text)
	wordCount := len(words)

	if wordCount == 0 {
		return 50.0, SpecificityDetail{}
	}

	numberCount := len(numberPattern.FindAllString(text, -1))
	properNounEst := estimateProperNouns(text)
	hedgeCount := countPhrases(text, hedgePhrases)
	concreteCount := countPhrases(text, concretePhrases)
	vagueCount := countPhrases(text, vaguePhrases)

	specificSignals := float64(numberCount + properNounEst + concreteCount)
	vagueSignals := float64(hedgeCount + vagueCount)
	totalSignals := specificSignals + vagueSignals

	specificRatio := 0.5
	if totalSignals > 0 {
		specificRatio = specificSignals / totalSignals
	}

	score := 50.0
	score += float64(numberCount) * 3.0
	score += float64(properNounEst) * 2.5
	score += float64(concreteCount) * 4.0
	score -= float64(hedgeCount) * 4.0
	score -= float64(vagueCount) * 3.0

	density := specificSignals / float64(wordCount) * 100
	score += density * 1.5

	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	detail := SpecificityDetail{
		NumberCount:   numberCount,
		ProperNounEst: properNounEst,
		HedgeCount:    hedgeCount,
		ConcreteCount: concreteCount,
		VagueCount:    vagueCount,
		WordCount:     wordCount,
		SpecificRatio: math.Round(specificRatio*100) / 100,
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

	var models []ModelSpecificity
	for mn, entries := range modelEntries {
		var totalScore float64
		var aggDetail SpecificityDetail
		labs := map[string]bool{}

		for _, e := range entries {
			labs[e.Lab] = true
			score, detail := AnalyzeText(e.Summary)
			totalScore += score
			aggDetail.NumberCount += detail.NumberCount
			aggDetail.ProperNounEst += detail.ProperNounEst
			aggDetail.HedgeCount += detail.HedgeCount
			aggDetail.ConcreteCount += detail.ConcreteCount
			aggDetail.VagueCount += detail.VagueCount
			aggDetail.WordCount += detail.WordCount
			aggDetail.SpecificRatio += detail.SpecificRatio
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		aggDetail.SpecificRatio = math.Round(aggDetail.SpecificRatio/n*100) / 100

		models = append(models, ModelSpecificity{
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
	b.WriteString("Response Specificity Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No specificity data available.\n")
		return b.String()
	}

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s\n", m.Model)
		fmt.Fprintf(&b, "    Score: %.1f  Grade: %s\n", m.Score, m.Grade)
		fmt.Fprintf(&b, "    Numbers: %d  Proper Nouns: %d  Concrete: %d\n",
			m.Detail.NumberCount, m.Detail.ProperNounEst, m.Detail.ConcreteCount)
		fmt.Fprintf(&b, "    Hedges: %d  Vague: %d  Ratio: %.2f\n",
			m.Detail.HedgeCount, m.Detail.VagueCount, m.Detail.SpecificRatio)
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
