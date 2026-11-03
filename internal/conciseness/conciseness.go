package conciseness

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

var fillerPhrases = []string{
	"it is important to note that",
	"it should be noted that",
	"it is worth mentioning that",
	"as a matter of fact",
	"in order to",
	"for the purpose of",
	"due to the fact that",
	"in spite of the fact that",
	"at this point in time",
	"at the present time",
	"in the event that",
	"on the other hand",
	"in terms of",
	"with regard to",
	"with respect to",
	"in light of the fact that",
	"it goes without saying",
	"needless to say",
	"as previously mentioned",
	"as stated above",
}

var redundantQualifiers = []string{
	"very unique",
	"completely finished",
	"absolutely essential",
	"totally complete",
	"very important",
	"extremely crucial",
	"highly significant",
	"really important",
	"quite significant",
	"very necessary",
	"basically",
	"essentially",
	"fundamentally",
	"actually",
	"literally",
	"honestly",
	"personally i think",
	"in my opinion i believe",
	"i would say that",
	"i think that perhaps",
}

var hedgingWords = []string{
	"maybe",
	"perhaps",
	"possibly",
	"might",
	"could potentially",
	"it seems like",
	"it appears that",
	"somewhat",
	"sort of",
	"kind of",
	"more or less",
	"to some extent",
	"in a way",
	"arguably",
	"presumably",
	"supposedly",
	"conceivably",
	"ostensibly",
	"theoretically speaking",
	"hypothetically",
}

var repetitionMarkers = []string{
	"as i said",
	"as mentioned earlier",
	"as noted above",
	"to reiterate",
	"again,",
	"once again",
	"to repeat",
	"as i already stated",
	"i already mentioned",
	"let me say again",
	"in other words",
	"to put it another way",
	"that is to say",
	"what i mean is",
	"to clarify what i said",
	"to be more specific about what i said",
	"going back to what i said",
	"returning to my earlier point",
	"as i was saying",
	"to summarize what i already said",
}

type Detail struct {
	FillerCount     int     `json:"filler_count"`
	RedundantCount  int     `json:"redundant_count"`
	HedgingCount    int     `json:"hedging_count"`
	RepetitionCount int     `json:"repetition_count"`
	TotalWords      int     `json:"total_words"`
	PaddingRatio    float64 `json:"padding_ratio"`
}

type ModelConciseness struct {
	Model       string  `json:"model"`
	Score       float64 `json:"score"`
	Grade       string  `json:"grade"`
	Detail      Detail  `json:"detail"`
	SampleCount int     `json:"sample_count"`
}

type Report struct {
	Models     []ModelConciseness `json:"models"`
	BestModel  string             `json:"best_model"`
	WorstModel string             `json:"worst_model"`
	MeanScore  float64            `json:"mean_score"`
}

func countWords(text string) int {
	return len(strings.Fields(text))
}

func countPhrases(text string, phrases []string) int {
	n := 0
	for _, p := range phrases {
		idx := 0
		for {
			i := strings.Index(text[idx:], p)
			if i < 0 {
				break
			}
			n++
			idx += i + len(p)
		}
	}
	return n
}

func analyzeSample(summary string) Detail {
	lower := strings.ToLower(summary)
	words := countWords(summary)
	filler := countPhrases(lower, fillerPhrases)
	redundant := countPhrases(lower, redundantQualifiers)
	hedging := countPhrases(lower, hedgingWords)
	repetition := countPhrases(lower, repetitionMarkers)
	total := filler + redundant + hedging + repetition
	ratio := 0.0
	if words > 0 {
		ratio = float64(total) / float64(words)
	}
	return Detail{
		FillerCount:     filler,
		RedundantCount:  redundant,
		HedgingCount:    hedging,
		RepetitionCount: repetition,
		TotalWords:      words,
		PaddingRatio:    ratio,
	}
}

func grade(score float64) string {
	switch {
	case score >= 0.95:
		return "A+"
	case score >= 0.90:
		return "A"
	case score >= 0.85:
		return "A-"
	case score >= 0.80:
		return "B+"
	case score >= 0.75:
		return "B"
	case score >= 0.70:
		return "B-"
	case score >= 0.65:
		return "C+"
	case score >= 0.60:
		return "C"
	case score >= 0.55:
		return "C-"
	case score >= 0.50:
		return "D"
	default:
		return "F"
	}
}

func Analyze(findings []model.Finding) Report {
	byModel := map[string][]model.Finding{}
	for _, f := range findings {
		byModel[f.Model.Model] = append(byModel[f.Model.Model], f)
	}

	var models []ModelConciseness
	for name, ff := range byModel {
		var totalFiller, totalRedundant, totalHedging, totalRepetition, totalWords int
		for _, f := range ff {
			d := analyzeSample(f.Summary)
			totalFiller += d.FillerCount
			totalRedundant += d.RedundantCount
			totalHedging += d.HedgingCount
			totalRepetition += d.RepetitionCount
			totalWords += d.TotalWords
		}
		padding := totalFiller + totalRedundant + totalHedging + totalRepetition
		ratio := 0.0
		if totalWords > 0 {
			ratio = float64(padding) / float64(totalWords)
		}
		score := math.Min(1.0, math.Max(0.0, 1.0-ratio*5))
		models = append(models, ModelConciseness{
			Model:       name,
			Score:       score,
			Grade:       grade(score),
			SampleCount: len(ff),
			Detail: Detail{
				FillerCount:     totalFiller,
				RedundantCount:  totalRedundant,
				HedgingCount:    totalHedging,
				RepetitionCount: totalRepetition,
				TotalWords:      totalWords,
				PaddingRatio:    ratio,
			},
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].Score > models[j].Score
	})

	r := Report{Models: models}
	if len(models) > 0 {
		r.BestModel = models[0].Model
		r.WorstModel = models[len(models)-1].Model
		sum := 0.0
		for _, m := range models {
			sum += m.Score
		}
		r.MeanScore = sum / float64(len(models))
	}
	return r
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Conciseness Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No models analyzed.\n")
		return b.String()
	}

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s\n", m.Model)
		fmt.Fprintf(&b, "    Score: %.2f  Grade: %s  Samples: %d\n", m.Score, m.Grade, m.SampleCount)
		fmt.Fprintf(&b, "    Filler: %d  Redundant: %d  Hedging: %d  Repetition: %d\n",
			m.Detail.FillerCount, m.Detail.RedundantCount, m.Detail.HedgingCount, m.Detail.RepetitionCount)
		fmt.Fprintf(&b, "    Words: %d  Padding ratio: %.4f\n\n", m.Detail.TotalWords, m.Detail.PaddingRatio)
	}

	fmt.Fprintf(&b, "  Best: %s  Worst: %s  Mean: %.2f\n", r.BestModel, r.WorstModel, r.MeanScore)
	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data)
}
