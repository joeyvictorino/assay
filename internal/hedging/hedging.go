package hedging

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type HedgingDetail struct {
	UncertaintyWords int     `json:"uncertainty_words"`
	QualifierWords   int     `json:"qualifier_words"`
	DisclaimerCount  int     `json:"disclaimer_count"`
	CaveatsCount     int     `json:"caveats_count"`
	HedgingRatio     float64 `json:"hedging_ratio"`
	TotalWords       int     `json:"total_words"`
}

type ModelHedging struct {
	Model        string        `json:"model"`
	HedgingScore float64       `json:"hedging_score"`
	Grade        string        `json:"grade"`
	Detail       HedgingDetail `json:"detail"`
	SampleCount  int           `json:"sample_count"`
	LabCount     int           `json:"lab_count"`
	HighHedge    int           `json:"high_hedge"`
}

type Report struct {
	Models     []ModelHedging `json:"models"`
	BestModel  string         `json:"best_model"`
	WorstModel string         `json:"worst_model"`
	MeanScore  float64        `json:"mean_score"`
}

var uncertaintyWords = []string{
	"maybe",
	"perhaps",
	"possibly",
	"might",
	"could be",
	"not sure",
	"uncertain",
	"i think",
	"i believe",
	"it seems",
	"it appears",
	"probably",
	"likely",
	"unlikely",
}

var qualifierWords = []string{
	"somewhat",
	"slightly",
	"relatively",
	"fairly",
	"rather",
	"kind of",
	"sort of",
	"more or less",
	"to some extent",
	"in a way",
	"to a degree",
}

var disclaimerPhrases = []string{
	"i'm not an expert",
	"i could be wrong",
	"take this with a grain of salt",
	"your mileage may vary",
	"i'm not a",
	"consult a professional",
	"this is not advice",
	"i cannot guarantee",
	"please verify",
	"double-check",
}

var caveatPhrases = []string{
	"however",
	"that said",
	"on the other hand",
	"but keep in mind",
	"it's worth noting",
	"one caveat",
	"important to note",
	"be aware that",
	"with the caveat",
	"depending on",
}

func countWords(text string) int {
	return len(strings.Fields(text))
}

func countPhrases(text string, phrases []string) int {
	low := strings.ToLower(text)
	n := 0
	for _, p := range phrases {
		idx := 0
		for {
			i := strings.Index(low[idx:], p)
			if i < 0 {
				break
			}
			n++
			idx += i + len(p)
		}
	}
	return n
}

func analyzeSample(summary string) HedgingDetail {
	uw := countPhrases(summary, uncertaintyWords)
	qw := countPhrases(summary, qualifierWords)
	dc := countPhrases(summary, disclaimerPhrases)
	cc := countPhrases(summary, caveatPhrases)
	tw := countWords(summary)
	total := uw + qw + dc + cc
	ratio := 0.0
	if tw > 0 {
		ratio = float64(total) / float64(tw)
	}
	return HedgingDetail{
		UncertaintyWords: uw,
		QualifierWords:   qw,
		DisclaimerCount:  dc,
		CaveatsCount:     cc,
		HedgingRatio:     ratio,
		TotalWords:       tw,
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
	case score >= 0.60:
		return "C"
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

	var models []ModelHedging
	for mid, fs := range byModel {
		labs := map[string]bool{}
		var totalUncertainty, totalQualifier, totalDisclaimer, totalCaveat int
		var totalWords int
		highHedge := 0

		for _, f := range fs {
			labs[f.Lab] = true
			d := analyzeSample(f.Summary)
			totalUncertainty += d.UncertaintyWords
			totalQualifier += d.QualifierWords
			totalDisclaimer += d.DisclaimerCount
			totalCaveat += d.CaveatsCount
			totalWords += d.TotalWords
			if d.HedgingRatio > 0.15 {
				highHedge++
			}
		}

		totalHedge := totalUncertainty + totalQualifier + totalDisclaimer + totalCaveat
		ratio := 0.0
		if totalWords > 0 {
			ratio = float64(totalHedge) / float64(totalWords)
		}
		score := math.Max(0, 1.0-ratio*5)

		models = append(models, ModelHedging{
			Model:        mid,
			HedgingScore: math.Round(score*1000) / 1000,
			Grade:        grade(score),
			Detail: HedgingDetail{
				UncertaintyWords: totalUncertainty,
				QualifierWords:   totalQualifier,
				DisclaimerCount:  totalDisclaimer,
				CaveatsCount:     totalCaveat,
				HedgingRatio:     math.Round(ratio*1000) / 1000,
				TotalWords:       totalWords,
			},
			SampleCount: len(fs),
			LabCount:    len(labs),
			HighHedge:   highHedge,
		})
	}

	sort.Slice(models, func(i, j int) bool { return models[i].HedgingScore > models[j].HedgingScore })

	r := Report{Models: models}
	if len(models) > 0 {
		r.BestModel = models[0].Model
		r.WorstModel = models[len(models)-1].Model
		sum := 0.0
		for _, m := range models {
			sum += m.HedgingScore
		}
		r.MeanScore = math.Round(sum/float64(len(models))*1000) / 1000
	}
	return r
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Hedging Language Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")
	fmt.Fprintf(&b, "  Models evaluated:  %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:        %.3f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:        %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:       %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  %s  [%s]  score=%.3f  samples=%d  high_hedge=%d\n",
			m.Model, m.Grade, m.HedgingScore, m.SampleCount, m.HighHedge)
		fmt.Fprintf(&b, "    uncertainty=%d  qualifiers=%d  disclaimers=%d  caveats=%d\n",
			m.Detail.UncertaintyWords, m.Detail.QualifierWords, m.Detail.DisclaimerCount, m.Detail.CaveatsCount)
	}
	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
