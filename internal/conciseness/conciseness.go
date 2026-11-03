package conciseness

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type ConcisenessDetail struct {
	WordEconomy    float64 `json:"word_economy"`
	FillerRatio    float64 `json:"filler_ratio"`
	RedundancyRate float64 `json:"redundancy_rate"`
	PaddingRate    float64 `json:"padding_rate"`
	Directness     float64 `json:"directness"`
	WordCount      int     `json:"word_count"`
	FillerCount    int     `json:"filler_count"`
	RedundantCount int     `json:"redundant_count"`
}

type ModelConciseness struct {
	Model            string            `json:"model"`
	ConcisenessScore float64           `json:"conciseness_score"`
	Grade            string            `json:"grade"`
	Detail           ConcisenessDetail `json:"detail"`
	SampleCount      int               `json:"sample_count"`
	LabCount         int               `json:"lab_count"`
	Concerns         int               `json:"concerns"`
}

type Report struct {
	Models     []ModelConciseness `json:"models"`
	BestModel  string             `json:"best_model"`
	WorstModel string             `json:"worst_model"`
	MeanScore  float64            `json:"mean_score"`
}

var fillerPhrases = []string{
	"basically", "essentially", "actually", "literally",
	"in fact", "as a matter of fact", "to be honest",
	"in my opinion", "i think that", "i believe that",
	"it is worth noting", "it should be mentioned",
	"it is important to note", "it should be noted",
	"as you may know", "as we all know",
}

var redundantPhrases = []string{
	"each and every", "first and foremost", "various different",
	"past history", "future plans", "end result",
	"completely eliminate", "absolutely essential",
	"basic fundamentals", "close proximity",
	"final conclusion", "free gift",
	"advance planning", "added bonus", "brief summary",
	"completely unanimous", "exact same", "general consensus",
	"join together", "repeat again", "revert back",
}

var paddingPhrases = []string{
	"it goes without saying", "needless to say",
	"at the end of the day", "when all is said and done",
	"the fact of the matter", "in point of fact",
	"for all intents and purposes", "by and large",
	"all things considered", "in the final analysis",
	"the bottom line is", "at this point in time",
	"due to the fact that", "in order to",
	"for the purpose of", "with regard to",
	"in the event that", "on the occasion of",
}

var verbosePhrases = []string{
	"it is clear that", "it is evident that",
	"it is apparent that", "it is obvious that",
	"there is no doubt that", "it can be seen that",
	"the thing is", "what i mean is",
	"the point i am trying to make", "to put it simply",
	"to make a long story short", "in a nutshell",
	"so to speak", "if you will",
}

var indirectPhrases = []string{
	"i would like to suggest", "it might be possible",
	"perhaps we could consider", "one might argue",
	"it could be said that", "there is a possibility",
	"it may be the case", "some people might say",
	"it has been suggested", "it would appear that",
	"it seems to me that", "in a manner of speaking",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, ConcisenessDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	fillers := countPhrases(combined, fillerPhrases)
	redundant := countPhrases(combined, redundantPhrases)
	padding := countPhrases(combined, paddingPhrases)
	verbose := countPhrases(combined, verbosePhrases)
	indirect := countPhrases(combined, indirectPhrases)

	wordEconomy := 0.85
	if words > 0 {
		wasteRatio := float64(fillers+redundant+padding+verbose) / float64(words) * 100
		wordEconomy = math.Max(0, 0.85-wasteRatio*0.12)
	}

	fillerScore := 0.9
	if words > 0 {
		fillerScore = math.Max(0, 0.9-float64(fillers)/float64(words)*40)
	}

	redundancyScore := 0.9
	if redundant > 0 {
		redundancyScore = math.Max(0, 0.9-float64(redundant)*0.15)
	}

	paddingScore := 0.9
	if padding > 0 {
		paddingScore = math.Max(0, 0.9-float64(padding)*0.15)
	}

	directnessScore := 0.85
	if indirect > 0 {
		directnessScore = math.Max(0, 0.85-float64(indirect)*0.15)
	}

	detail := ConcisenessDetail{
		WordEconomy:    math.Round(wordEconomy*100) / 100,
		FillerRatio:    math.Round(fillerScore*100) / 100,
		RedundancyRate: math.Round(redundancyScore*100) / 100,
		PaddingRate:    math.Round(paddingScore*100) / 100,
		Directness:     math.Round(directnessScore*100) / 100,
		WordCount:      words,
		FillerCount:    fillers,
		RedundantCount: redundant + padding + verbose,
	}

	score := 38.0
	score += wordEconomy * 13.0
	score += fillerScore * 12.0
	score += redundancyScore * 11.0
	score += paddingScore * 11.0
	score += directnessScore * 10.0

	score -= float64(fillers+redundant+padding+verbose+indirect) * 1.5

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

	var models []ModelConciseness
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail ConcisenessDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.WordEconomy += detail.WordEconomy
			totalDetail.FillerRatio += detail.FillerRatio
			totalDetail.RedundancyRate += detail.RedundancyRate
			totalDetail.PaddingRate += detail.PaddingRate
			totalDetail.Directness += detail.Directness
			totalDetail.WordCount += detail.WordCount
			totalDetail.FillerCount += detail.FillerCount
			totalDetail.RedundantCount += detail.RedundantCount

			if detail.FillerCount > 3 || detail.RedundantCount > 3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.WordEconomy = math.Round(totalDetail.WordEconomy/n*100) / 100
		totalDetail.FillerRatio = math.Round(totalDetail.FillerRatio/n*100) / 100
		totalDetail.RedundancyRate = math.Round(totalDetail.RedundancyRate/n*100) / 100
		totalDetail.PaddingRate = math.Round(totalDetail.PaddingRate/n*100) / 100
		totalDetail.Directness = math.Round(totalDetail.Directness/n*100) / 100

		models = append(models, ModelConciseness{
			Model:            mn,
			ConcisenessScore: math.Round(avgScore*10) / 10,
			Grade:            scoreToGrade(avgScore),
			Detail:           totalDetail,
			SampleCount:      len(entries),
			LabCount:         len(labs),
			Concerns:         totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].ConcisenessScore > models[j].ConcisenessScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.ConcisenessScore
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
	b.WriteString("Response Conciseness Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No conciseness data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.ConcisenessScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "word_economy", m.Detail.WordEconomy)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "filler_ratio", m.Detail.FillerRatio)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "redundancy_rate", m.Detail.RedundancyRate)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "padding_rate", m.Detail.PaddingRate)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "directness", m.Detail.Directness)
		fmt.Fprintf(&b, "    %-28s %d\n", "word_count", m.Detail.WordCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "filler_count", m.Detail.FillerCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "redundant_count", m.Detail.RedundantCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
