package factcheck

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type FactDetail struct {
	ClaimCount       int `json:"claim_count"`
	HedgedClaims     int `json:"hedged_claims"`
	AssertedClaims   int `json:"asserted_claims"`
	UnverifiableCues int `json:"unverifiable_cues"`
	CitationCues     int `json:"citation_cues"`
	QuantityClaims   int `json:"quantity_claims"`
}

type ModelFact struct {
	Model           string     `json:"model"`
	FactScore       float64    `json:"fact_score"`
	Grade           string     `json:"grade"`
	Detail          FactDetail `json:"detail"`
	SampleCount     int        `json:"sample_count"`
	LabCount        int        `json:"lab_count"`
	HighReliability int        `json:"high_reliability"`
	LowReliability  int        `json:"low_reliability"`
}

type Report struct {
	Models          []ModelFact `json:"models"`
	MostReliable    string      `json:"most_reliable"`
	LeastReliable   string      `json:"least_reliable"`
	MeanScore       float64     `json:"mean_score"`
}

var claimIndicators = []string{
	"is", "are", "was", "were", "has been", "have been",
	"will be", "can be", "shows that", "proves that",
	"demonstrates", "confirms", "establishes",
}

var hedgingIndicators = []string{
	"might", "may", "could", "possibly", "perhaps",
	"likely", "probably", "arguably", "it seems",
	"it appears", "tends to", "generally", "some suggest",
	"it is believed", "reportedly", "allegedly",
}

var assertionIndicators = []string{
	"certainly", "definitely", "absolutely", "undoubtedly",
	"clearly", "obviously", "without question", "proven",
	"established fact", "indisputable", "unquestionable",
	"always", "never", "100%", "guaranteed",
}

var unverifiableCues = []string{
	"everyone knows", "it is well known", "common knowledge",
	"many people say", "experts agree", "some say",
	"sources say", "it has been said", "word on the street",
	"people believe", "widely believed", "generally accepted",
}

var citationCues = []string{
	"according to", "study shows", "research indicates",
	"data suggests", "published in", "reported by",
	"survey found", "analysis reveals", "meta-analysis",
	"peer-reviewed", "journal", "institution",
	"university", "et al", "findings show",
}

var quantityCues = []string{
	"percent", "%", "million", "billion", "thousand",
	"approximately", "roughly", "about", "estimated",
	"measured at", "recorded at", "calculated",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(text string) (float64, FactDetail) {
	claims := countPhrases(text, claimIndicators)
	hedged := countPhrases(text, hedgingIndicators)
	asserted := countPhrases(text, assertionIndicators)
	unverifiable := countPhrases(text, unverifiableCues)
	citations := countPhrases(text, citationCues)
	quantities := countPhrases(text, quantityCues)

	detail := FactDetail{
		ClaimCount:       claims,
		HedgedClaims:     hedged,
		AssertedClaims:   asserted,
		UnverifiableCues: unverifiable,
		CitationCues:     citations,
		QuantityClaims:   quantities,
	}

	words := len(strings.Fields(text))
	if words == 0 {
		return 50, detail
	}

	positiveSignals := float64(citations*4 + quantities*2 + hedged*1)
	negativeSignals := float64(unverifiable*4 + asserted*2)

	normalizer := math.Log2(float64(words) + 1)
	if normalizer < 1 {
		normalizer = 1
	}

	rawScore := (positiveSignals - negativeSignals) / normalizer * 10
	score := 60 + rawScore
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

	var models []ModelFact
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail FactDetail
		highReliability := 0
		lowReliability := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			score, detail := scoreResponse(e.Summary)
			totalScore += score
			totalDetail.ClaimCount += detail.ClaimCount
			totalDetail.HedgedClaims += detail.HedgedClaims
			totalDetail.AssertedClaims += detail.AssertedClaims
			totalDetail.UnverifiableCues += detail.UnverifiableCues
			totalDetail.CitationCues += detail.CitationCues
			totalDetail.QuantityClaims += detail.QuantityClaims

			if score >= 80 {
				highReliability++
			} else if score < 40 {
				lowReliability++
			}
		}

		avgScore := totalScore / float64(len(entries))
		models = append(models, ModelFact{
			Model:           mn,
			FactScore:       math.Round(avgScore*10) / 10,
			Grade:           scoreToGrade(avgScore),
			Detail:          totalDetail,
			SampleCount:     len(entries),
			LabCount:        len(labs),
			HighReliability: highReliability,
			LowReliability:  lowReliability,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].FactScore > models[j].FactScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.FactScore
	}

	r := Report{Models: models}
	if len(models) > 0 {
		r.MostReliable = models[0].Model
		r.LeastReliable = models[len(models)-1].Model
		r.MeanScore = math.Round(sumScore/float64(len(models))*10) / 10
	}
	return r
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Factual Claim Verification\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No factual data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Most reliable:        %s\n", r.MostReliable)
	fmt.Fprintf(&b, "  Least reliable:       %s\n\n", r.LeastReliable)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.FactScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "claim_count", m.Detail.ClaimCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "hedged_claims", m.Detail.HedgedClaims)
		fmt.Fprintf(&b, "    %-28s %d\n", "asserted_claims", m.Detail.AssertedClaims)
		fmt.Fprintf(&b, "    %-28s %d\n", "unverifiable_cues", m.Detail.UnverifiableCues)
		fmt.Fprintf(&b, "    %-28s %d\n", "citation_cues", m.Detail.CitationCues)
		fmt.Fprintf(&b, "    %-28s %d\n", "quantity_claims", m.Detail.QuantityClaims)
		fmt.Fprintf(&b, "    High reliability: %d  Low reliability: %d\n", m.HighReliability, m.LowReliability)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
