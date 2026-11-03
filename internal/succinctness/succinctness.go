package succinctness

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

var stopWords = map[string]bool{
	"a": true, "an": true, "the": true, "is": true, "are": true,
	"was": true, "were": true, "be": true, "been": true, "being": true,
	"have": true, "has": true, "had": true, "do": true, "does": true,
	"did": true, "will": true, "would": true, "could": true, "should": true,
	"may": true, "might": true, "shall": true, "can": true, "need": true,
	"to": true, "of": true, "in": true, "for": true, "on": true,
	"with": true, "at": true, "by": true, "from": true, "as": true,
	"into": true, "through": true, "during": true, "before": true,
	"after": true, "above": true, "below": true, "between": true,
	"and": true, "but": true, "or": true, "nor": true, "not": true,
	"so": true, "yet": true, "both": true, "either": true, "neither": true,
	"this": true, "that": true, "these": true, "those": true,
	"it": true, "its": true, "i": true, "we": true, "you": true,
	"he": true, "she": true, "they": true, "me": true, "him": true,
	"her": true, "us": true, "them": true, "my": true, "your": true,
	"his": true, "our": true, "their": true,
}

type SuccinctnessDetail struct {
	BrevityScore     float64 `json:"brevity_score"`
	RedundancyPen    float64 `json:"redundancy_penalty"`
	FillerPen        float64 `json:"filler_penalty"`
	InformationDensity float64 `json:"information_density"`
}

type ModelSuccinctness struct {
	Model             string              `json:"model"`
	SuccinctnessScore float64             `json:"succinctness_score"`
	Grade             string              `json:"grade"`
	Detail            SuccinctnessDetail  `json:"detail"`
	SampleCount       int                 `json:"sample_count"`
	VerboseCount      int                 `json:"verbose_count"`
	RedundantCount    int                 `json:"redundant_count"`
}

type Report struct {
	Models         []ModelSuccinctness `json:"models"`
	MostSuccinct   string              `json:"most_succinct"`
	LeastSuccinct  string              `json:"least_succinct"`
	MeanScore      float64             `json:"mean_score"`
}

func extractKeywords(text string) map[string]int {
	words := strings.Fields(strings.ToLower(text))
	kw := map[string]int{}
	for _, w := range words {
		clean := strings.Trim(w, ".,;:!?\"'()[]{}")
		if len(clean) < 2 || stopWords[clean] {
			continue
		}
		kw[clean]++
	}
	return kw
}

func brevityScore(lab, summary string) float64 {
	labWords := strings.Fields(lab)
	sumWords := strings.Fields(summary)
	if len(labWords) == 0 {
		return 50
	}
	ratio := float64(len(sumWords)) / float64(len(labWords))
	if ratio <= 2.0 {
		return 100
	}
	if ratio <= 4.0 {
		return 100 - (ratio-2.0)*20
	}
	if ratio <= 8.0 {
		return 60 - (ratio-4.0)*10
	}
	return math.Max(0, 20-(ratio-8.0)*5)
}

func redundancyPenalty(summary string) float64 {
	kw := extractKeywords(summary)
	if len(kw) == 0 {
		return 100
	}

	totalRepeats := 0
	for _, count := range kw {
		if count > 2 {
			totalRepeats += count - 2
		}
	}

	words := strings.Fields(summary)
	if len(words) == 0 {
		return 100
	}

	ratio := float64(totalRepeats) / float64(len(words))
	score := 100 - math.Min(100, ratio*500)
	if score < 0 {
		score = 0
	}
	return math.Round(score*10) / 10
}

func fillerPenalty(summary string) float64 {
	lower := strings.ToLower(summary)
	fillerPhrases := []string{
		"it is worth noting", "it should be noted",
		"it is important to mention", "as mentioned earlier",
		"needless to say", "in other words",
		"as a matter of fact", "at the end of the day",
		"all things considered", "by and large",
		"for all intents and purposes", "in conclusion",
		"to summarize", "in summary", "basically",
		"essentially", "fundamentally", "overall",
	}
	fillerCount := 0
	for _, f := range fillerPhrases {
		if strings.Contains(lower, f) {
			fillerCount++
		}
	}

	score := 100 - math.Min(100, float64(fillerCount)*20)
	if score < 0 {
		score = 0
	}
	return math.Round(score*10) / 10
}

func informationDensity(lab, summary string) float64 {
	labKW := extractKeywords(lab)
	sumKW := extractKeywords(summary)

	words := strings.Fields(summary)
	if len(words) == 0 {
		return 0
	}

	relevantKW := 0
	for w := range sumKW {
		if labKW[w] > 0 {
			relevantKW++
		}
		for lw := range labKW {
			if w != lw && (strings.Contains(w, lw) || strings.Contains(lw, w)) {
				relevantKW++
				break
			}
		}
	}

	ratio := float64(relevantKW) / float64(len(words))
	score := math.Min(100, ratio*400)
	return math.Round(score*10) / 10
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

	type pair struct {
		lab     string
		summary string
	}
	modelPairs := map[string][]pair{}
	for _, f := range findings {
		mn := f.Model.Model
		if mn == "" {
			mn = "unknown"
		}
		modelPairs[mn] = append(modelPairs[mn], pair{lab: f.Lab, summary: f.Summary})
	}

	var models []ModelSuccinctness
	for mn, pairs := range modelPairs {
		var sumBrevity, sumRedundancy, sumFiller, sumDensity float64
		verboseCount := 0
		redundantCount := 0

		for _, p := range pairs {
			brev := brevityScore(p.lab, p.summary)
			redun := redundancyPenalty(p.summary)
			filler := fillerPenalty(p.summary)
			density := informationDensity(p.lab, p.summary)

			sumBrevity += brev
			sumRedundancy += redun
			sumFiller += filler
			sumDensity += density

			if brev < 40 {
				verboseCount++
			}
			if redun < 50 {
				redundantCount++
			}
		}

		n := float64(len(pairs))
		avgBrevity := sumBrevity / n
		avgRedundancy := sumRedundancy / n
		avgFiller := sumFiller / n
		avgDensity := sumDensity / n

		overall := (avgBrevity + avgRedundancy + avgFiller + avgDensity) / 4

		models = append(models, ModelSuccinctness{
			Model:             mn,
			SuccinctnessScore: math.Round(overall*10) / 10,
			Grade:             scoreToGrade(overall),
			Detail: SuccinctnessDetail{
				BrevityScore:      math.Round(avgBrevity*10) / 10,
				RedundancyPen:     math.Round(avgRedundancy*10) / 10,
				FillerPen:         math.Round(avgFiller*10) / 10,
				InformationDensity: math.Round(avgDensity*10) / 10,
			},
			SampleCount:    len(pairs),
			VerboseCount:   verboseCount,
			RedundantCount: redundantCount,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].SuccinctnessScore > models[j].SuccinctnessScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.SuccinctnessScore
	}

	r := Report{Models: models}
	if len(models) > 0 {
		r.MostSuccinct = models[0].Model
		r.LeastSuccinct = models[len(models)-1].Model
		r.MeanScore = math.Round(sumScore/float64(len(models))*10) / 10
	}
	return r
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Response Succinctness Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No succinctness data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:    %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:          %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Most succinct:       %s\n", r.MostSuccinct)
	fmt.Fprintf(&b, "  Least succinct:      %s\n\n", r.LeastSuccinct)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples)\n",
			m.Model, m.Grade, m.SuccinctnessScore, m.SampleCount)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "brevity_score", m.Detail.BrevityScore)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "redundancy_penalty", m.Detail.RedundancyPen)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "filler_penalty", m.Detail.FillerPen)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "information_density", m.Detail.InformationDensity)
		fmt.Fprintf(&b, "    Verbose warnings: %d  Redundant: %d\n",
			m.VerboseCount, m.RedundantCount)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
