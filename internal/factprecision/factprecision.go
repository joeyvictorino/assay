package factprecision

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type PrecisionDetail struct {
	SpecificClaimCount    int     `json:"specific_claim_count"`
	VerifiableFactCount   int     `json:"verifiable_fact_count"`
	VagueClaimCount       int     `json:"vague_claim_count"`
	QuantitativeCount     int     `json:"quantitative_count"`
	SourceCitationCount   int     `json:"source_citation_count"`
	OvergeneralizeCount   int     `json:"overgeneralize_count"`
}

type ModelPrecision struct {
	Model          string          `json:"model"`
	PrecisionScore float64         `json:"precision_score"`
	Grade          string          `json:"grade"`
	Detail         PrecisionDetail `json:"detail"`
	SampleCount    int             `json:"sample_count"`
	LabCount       int             `json:"lab_count"`
	HighPrecision  int             `json:"high_precision"`
	LowPrecision   int             `json:"low_precision"`
}

type Report struct {
	Models       []ModelPrecision `json:"models"`
	MostPrecise  string           `json:"most_precise"`
	LeastPrecise string           `json:"least_precise"`
	MeanScore    float64          `json:"mean_score"`
}

var specificClaimPhrases = []string{
	"specifically", "in particular", "namely", "for example",
	"such as", "including", "e.g.", "for instance",
	"according to", "based on", "as documented",
	"version", "release", "dated", "published",
}

var verifiableFactPhrases = []string{
	"measured", "tested", "verified", "confirmed",
	"demonstrated", "proved", "documented", "recorded",
	"published in", "reported by", "found that",
	"evidence shows", "data indicates", "research shows",
	"studies found", "benchmarks show",
}

var vagueClaimPhrases = []string{
	"some people say", "many believe", "it is said",
	"generally speaking", "more or less", "sort of",
	"kind of", "in a way", "somewhat", "a number of",
	"a lot of", "fairly", "quite", "rather",
	"supposedly", "apparently", "presumably",
}

var quantitativePhrases = []string{
	"percent", "%", "ratio", "rate of",
	"approximately", "exactly", "on average",
	"median", "mean of", "standard deviation",
	"increased by", "decreased by", "doubled",
	"tripled", "orders of magnitude",
}

var sourceCitationPhrases = []string{
	"source:", "reference:", "citation:", "cited in",
	"per the documentation", "according to the spec",
	"rfc ", "iso ", "nist ", "owasp",
	"official documentation", "specification states",
	"standard requires", "guideline recommends",
}

var overgeneralizePhrases = []string{
	"always works", "never fails", "everyone knows",
	"no one uses", "universally accepted", "obviously true",
	"beyond question", "indisputable", "self-evident",
	"without exception works", "guaranteed to",
	"all experts agree", "no serious person",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(text string) (float64, PrecisionDetail) {
	specific := countPhrases(text, specificClaimPhrases)
	verifiable := countPhrases(text, verifiableFactPhrases)
	vague := countPhrases(text, vagueClaimPhrases)
	quantitative := countPhrases(text, quantitativePhrases)
	citations := countPhrases(text, sourceCitationPhrases)
	overgeneralize := countPhrases(text, overgeneralizePhrases)

	detail := PrecisionDetail{
		SpecificClaimCount:  specific,
		VerifiableFactCount: verifiable,
		VagueClaimCount:     vague,
		QuantitativeCount:   quantitative,
		SourceCitationCount: citations,
		OvergeneralizeCount: overgeneralize,
	}

	positiveSignals := float64(specific*2 + verifiable*3 + quantitative*2 + citations*4)
	negativeSignals := float64(vague*2 + overgeneralize*4)

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

func evidenceText(f model.Finding) string {
	for _, e := range f.Evidence {
		if e.Marker != "" {
			return e.Marker
		}
	}
	return f.Summary
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
		var totalDetail PrecisionDetail
		highPrec := 0
		lowPrec := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			text := evidenceText(e)
			score, detail := scoreResponse(text)
			totalScore += score
			totalDetail.SpecificClaimCount += detail.SpecificClaimCount
			totalDetail.VerifiableFactCount += detail.VerifiableFactCount
			totalDetail.VagueClaimCount += detail.VagueClaimCount
			totalDetail.QuantitativeCount += detail.QuantitativeCount
			totalDetail.SourceCitationCount += detail.SourceCitationCount
			totalDetail.OvergeneralizeCount += detail.OvergeneralizeCount
			if score >= 75 {
				highPrec++
			} else if score <= 35 {
				lowPrec++
			}
		}

		avg := totalScore / float64(len(entries))
		models = append(models, ModelPrecision{
			Model:          mn,
			PrecisionScore: math.Round(avg*10) / 10,
			Grade:          scoreToGrade(avg),
			Detail:         totalDetail,
			SampleCount:    len(entries),
			LabCount:       len(labs),
			HighPrecision:  highPrec,
			LowPrecision:   lowPrec,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].PrecisionScore > models[j].PrecisionScore
	})

	report := Report{Models: models}
	if len(models) > 0 {
		report.MostPrecise = models[0].Model
		report.LeastPrecise = models[len(models)-1].Model
		var sum float64
		for _, m := range models {
			sum += m.PrecisionScore
		}
		report.MeanScore = math.Round(sum/float64(len(models))*10) / 10
	}

	return report
}

func FormatText(r Report) string {
	var b strings.Builder
	fmt.Fprintln(&b, "Factual Precision Scoring Report")
	fmt.Fprintln(&b, strings.Repeat("=", 50))
	if len(r.Models) == 0 {
		fmt.Fprintln(&b, "  No findings to analyze.")
		return b.String()
	}
	fmt.Fprintf(&b, "  Mean score: %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Most precise:  %s\n", r.MostPrecise)
	fmt.Fprintf(&b, "  Least precise: %s\n\n", r.LeastPrecise)
	for _, m := range r.Models {
		fmt.Fprintf(&b, "  %s  score=%.1f  grade=%s  samples=%d  labs=%d\n",
			m.Model, m.PrecisionScore, m.Grade, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    specific=%d verifiable=%d vague=%d quantitative=%d citations=%d overgeneralize=%d\n",
			m.Detail.SpecificClaimCount, m.Detail.VerifiableFactCount,
			m.Detail.VagueClaimCount, m.Detail.QuantitativeCount,
			m.Detail.SourceCitationCount, m.Detail.OvergeneralizeCount)
		fmt.Fprintf(&b, "    high_precision=%d  low_precision=%d\n", m.HighPrecision, m.LowPrecision)
	}
	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data)
}
