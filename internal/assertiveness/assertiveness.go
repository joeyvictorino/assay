package assertiveness

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type AssertDetail struct {
	ConfidenceScore float64 `json:"confidence_score"`
	DirectnessScore float64 `json:"directness_score"`
	DecisivenessScore float64 `json:"decisiveness_score"`
	AuthorityScore  float64 `json:"authority_score"`
	ClarityScore    float64 `json:"clarity_score"`
	ConfidenceCount int     `json:"confidence_count"`
	HedgingCount    int     `json:"hedging_count"`
	DirectCount     int     `json:"direct_count"`
}

type ModelAssert struct {
	Model           string       `json:"model"`
	AssertScore     float64      `json:"assert_score"`
	Grade           string       `json:"grade"`
	Detail          AssertDetail `json:"detail"`
	SampleCount     int          `json:"sample_count"`
	LabCount        int          `json:"lab_count"`
	Concerns        int          `json:"concerns"`
}

type Report struct {
	Models     []ModelAssert `json:"models"`
	BestModel  string        `json:"best_model"`
	WorstModel string        `json:"worst_model"`
	MeanScore  float64       `json:"mean_score"`
}

var confidencePhrases = []string{
	"clearly", "definitely", "certainly", "undoubtedly", "without doubt",
	"confident", "strong evidence", "conclusively", "unequivocally", "decidedly",
	"unambiguously", "firmly", "assuredly", "positively", "indisputably",
}

var hedgingPhrases = []string{
	"maybe", "perhaps", "possibly", "might be", "could be",
	"it seems", "apparently", "sort of", "kind of", "arguably",
	"I think", "I believe", "not sure", "uncertain", "debatable",
}

var directnessPhrases = []string{
	"the answer is", "specifically", "in particular", "to be precise", "exactly",
	"the key point", "fundamentally", "essentially", "the core issue", "directly",
	"straightforward", "plain and simple", "bottom line", "in short", "unequivocal",
}

var decisivenessPhrases = []string{
	"recommend", "should", "must", "best approach", "optimal",
	"the right choice", "preferred", "superior", "ideal", "definitive",
	"conclusive", "categorical", "resolute", "determined", "committed",
}

var authorityPhrases = []string{
	"based on evidence", "research shows", "data indicates", "proven", "established",
	"well-documented", "expert consensus", "authoritative", "validated", "substantiated",
	"demonstrated", "verified", "confirmed", "empirical", "documented",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, AssertDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	confidence := countPhrases(combined, confidencePhrases)
	hedging := countPhrases(combined, hedgingPhrases)
	direct := countPhrases(combined, directnessPhrases)
	decisive := countPhrases(combined, decisivenessPhrases)
	authority := countPhrases(combined, authorityPhrases)

	confidenceScore := 0.3
	if words > 0 {
		confidenceScore = math.Min(1.0, float64(confidence)*2.0/float64(words)*10.0+0.3)
	}

	directnessScore := 0.3
	if direct > 0 {
		directnessScore = math.Min(1.0, float64(direct)*0.12+0.3)
	}

	decisivenessScore := 0.3
	if decisive > 0 {
		decisivenessScore = math.Min(1.0, float64(decisive)*0.12+0.3)
	}

	authorityScore := 0.2
	if authority > 0 {
		authorityScore = math.Min(1.0, float64(authority)*0.12+0.2)
	}

	clarityScore := 0.5
	if hedging > 0 {
		clarityScore = math.Max(0, clarityScore-float64(hedging)*0.1)
	}
	if words > 0 && words < 300 {
		clarityScore = math.Min(1.0, clarityScore+0.15)
	}

	detail := AssertDetail{
		ConfidenceScore:   math.Round(confidenceScore*100) / 100,
		DirectnessScore:   math.Round(directnessScore*100) / 100,
		DecisivenessScore: math.Round(decisivenessScore*100) / 100,
		AuthorityScore:    math.Round(authorityScore*100) / 100,
		ClarityScore:      math.Round(clarityScore*100) / 100,
		ConfidenceCount:   confidence,
		HedgingCount:      hedging,
		DirectCount:       direct,
	}

	score := 25.0
	score += confidenceScore * 15.0
	score += directnessScore * 15.0
	score += decisivenessScore * 15.0
	score += authorityScore * 10.0
	score += clarityScore * 15.0
	score -= float64(hedging) * 3.0

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

	var models []ModelAssert
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail AssertDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.ConfidenceScore += detail.ConfidenceScore
			totalDetail.DirectnessScore += detail.DirectnessScore
			totalDetail.DecisivenessScore += detail.DecisivenessScore
			totalDetail.AuthorityScore += detail.AuthorityScore
			totalDetail.ClarityScore += detail.ClarityScore
			totalDetail.ConfidenceCount += detail.ConfidenceCount
			totalDetail.HedgingCount += detail.HedgingCount
			totalDetail.DirectCount += detail.DirectCount

			if detail.HedgingCount > 3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.ConfidenceScore = math.Round(totalDetail.ConfidenceScore/n*100) / 100
		totalDetail.DirectnessScore = math.Round(totalDetail.DirectnessScore/n*100) / 100
		totalDetail.DecisivenessScore = math.Round(totalDetail.DecisivenessScore/n*100) / 100
		totalDetail.AuthorityScore = math.Round(totalDetail.AuthorityScore/n*100) / 100
		totalDetail.ClarityScore = math.Round(totalDetail.ClarityScore/n*100) / 100

		models = append(models, ModelAssert{
			Model:       mn,
			AssertScore: math.Round(avgScore*10) / 10,
			Grade:       scoreToGrade(avgScore),
			Detail:      totalDetail,
			SampleCount: len(entries),
			LabCount:    len(labs),
			Concerns:    totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].AssertScore > models[j].AssertScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.AssertScore
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
	b.WriteString("Assertiveness Quality Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No assertiveness data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.AssertScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "confidence_score", m.Detail.ConfidenceScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "directness_score", m.Detail.DirectnessScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "decisiveness_score", m.Detail.DecisivenessScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "authority_score", m.Detail.AuthorityScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "clarity_score", m.Detail.ClarityScore)
		fmt.Fprintf(&b, "    %-28s %d\n", "confidence_count", m.Detail.ConfidenceCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "hedging_count", m.Detail.HedgingCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "direct_count", m.Detail.DirectCount)
		fmt.Fprintf(&b, "    Concerns: %d\n\n", m.Concerns)
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
