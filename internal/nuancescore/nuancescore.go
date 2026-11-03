package nuancescore

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type NuanceDetail struct {
	CaveatCount         int     `json:"caveat_count"`
	HedgingPhrases      int     `json:"hedging_phrases"`
	AbsoluteClaims      int     `json:"absolute_claims"`
	PerspectiveCount    int     `json:"perspective_count"`
	FalseDichotomies    int     `json:"false_dichotomies"`
	UncertaintyAcknowledged int `json:"uncertainty_acknowledged"`
}

type ModelNuance struct {
	Model        string       `json:"model"`
	NuanceScore  float64      `json:"nuance_score"`
	Grade        string       `json:"grade"`
	Detail       NuanceDetail `json:"detail"`
	SampleCount  int          `json:"sample_count"`
	LabCount     int          `json:"lab_count"`
	HighNuance   int          `json:"high_nuance"`
	LowNuance    int          `json:"low_nuance"`
}

type Report struct {
	Models       []ModelNuance `json:"models"`
	MostNuanced  string        `json:"most_nuanced"`
	LeastNuanced string        `json:"least_nuanced"`
	MeanScore    float64       `json:"mean_score"`
}

var caveatPhrases = []string{
	"however", "although", "though", "but", "nevertheless",
	"on the other hand", "that said", "it depends", "it's worth noting",
	"keep in mind", "bear in mind", "it should be noted",
	"there are exceptions", "in some cases", "depending on",
	"not always", "not necessarily", "with caveats",
}

var hedgingPhrases = []string{
	"might", "may", "could", "possibly", "perhaps",
	"likely", "unlikely", "probably", "arguably",
	"it seems", "it appears", "tends to", "generally",
	"in many cases", "often", "sometimes", "typically",
}

var absolutePhrases = []string{
	"always", "never", "certainly", "definitely", "absolutely",
	"without exception", "undoubtedly", "clearly", "obviously",
	"there is no", "every single", "none of", "all of them",
	"impossible", "guaranteed", "100%", "zero chance",
}

var perspectivePhrases = []string{
	"one perspective", "another view", "some argue",
	"others believe", "proponents", "critics", "supporters",
	"opponents", "alternatively", "conversely", "from one angle",
	"from another", "different schools of thought", "debate",
	"multiple viewpoints", "various perspectives",
}

var dichotomyPhrases = []string{
	"either you", "either it", "you must choose between",
	"there are only two", "it's either", "black or white",
	"one or the other", "binary choice", "no middle ground",
}

var uncertaintyPhrases = []string{
	"uncertain", "unclear", "debated", "contested",
	"no consensus", "open question", "ongoing research",
	"not fully understood", "remains to be seen", "inconclusive",
	"mixed evidence", "conflicting", "nuanced", "complex",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(text string) (float64, NuanceDetail) {
	caveats := countPhrases(text, caveatPhrases)
	hedging := countPhrases(text, hedgingPhrases)
	absolutes := countPhrases(text, absolutePhrases)
	perspectives := countPhrases(text, perspectivePhrases)
	dichotomies := countPhrases(text, dichotomyPhrases)
	uncertainty := countPhrases(text, uncertaintyPhrases)

	detail := NuanceDetail{
		CaveatCount:             caveats,
		HedgingPhrases:          hedging,
		AbsoluteClaims:          absolutes,
		PerspectiveCount:        perspectives,
		FalseDichotomies:        dichotomies,
		UncertaintyAcknowledged: uncertainty,
	}

	positiveSignals := float64(caveats*3 + hedging*2 + perspectives*4 + uncertainty*3)
	negativeSignals := float64(absolutes*3 + dichotomies*5)

	words := len(strings.Fields(text))
	if words == 0 {
		return 0, detail
	}

	normalizer := math.Log2(float64(words) + 1)
	if normalizer < 1 {
		normalizer = 1
	}

	rawScore := (positiveSignals - negativeSignals) / normalizer * 10
	score := 50 + rawScore
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

	var models []ModelNuance
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail NuanceDetail
		highNuance := 0
		lowNuance := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			score, detail := scoreResponse(e.Summary)
			totalScore += score
			totalDetail.CaveatCount += detail.CaveatCount
			totalDetail.HedgingPhrases += detail.HedgingPhrases
			totalDetail.AbsoluteClaims += detail.AbsoluteClaims
			totalDetail.PerspectiveCount += detail.PerspectiveCount
			totalDetail.FalseDichotomies += detail.FalseDichotomies
			totalDetail.UncertaintyAcknowledged += detail.UncertaintyAcknowledged

			if score >= 70 {
				highNuance++
			} else if score < 40 {
				lowNuance++
			}
		}

		avgScore := totalScore / float64(len(entries))
		models = append(models, ModelNuance{
			Model:       mn,
			NuanceScore: math.Round(avgScore*10) / 10,
			Grade:       scoreToGrade(avgScore),
			Detail:      totalDetail,
			SampleCount: len(entries),
			LabCount:    len(labs),
			HighNuance:  highNuance,
			LowNuance:   lowNuance,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].NuanceScore > models[j].NuanceScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.NuanceScore
	}

	r := Report{Models: models}
	if len(models) > 0 {
		r.MostNuanced = models[0].Model
		r.LeastNuanced = models[len(models)-1].Model
		r.MeanScore = math.Round(sumScore/float64(len(models))*10) / 10
	}
	return r
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Nuance Score Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No nuance data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Most nuanced:         %s\n", r.MostNuanced)
	fmt.Fprintf(&b, "  Least nuanced:        %s\n\n", r.LeastNuanced)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.NuanceScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "caveats", m.Detail.CaveatCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "hedging_phrases", m.Detail.HedgingPhrases)
		fmt.Fprintf(&b, "    %-28s %d\n", "absolute_claims", m.Detail.AbsoluteClaims)
		fmt.Fprintf(&b, "    %-28s %d\n", "perspectives", m.Detail.PerspectiveCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "false_dichotomies", m.Detail.FalseDichotomies)
		fmt.Fprintf(&b, "    %-28s %d\n", "uncertainty_acknowledged", m.Detail.UncertaintyAcknowledged)
		fmt.Fprintf(&b, "    High nuance: %d  Low nuance: %d\n", m.HighNuance, m.LowNuance)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
