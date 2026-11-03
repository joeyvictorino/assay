package contradict

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type ContradictDetail struct {
	ContradictionCount int     `json:"contradiction_count"`
	HedgeCount         int     `json:"hedge_count"`
	ConsistencyScore   float64 `json:"consistency_score"`
	ClarityScore       float64 `json:"clarity_score"`
	NegationCount      int     `json:"negation_count"`
	RetractionCount    int     `json:"retraction_count"`
}

type ModelContradict struct {
	Model            string           `json:"model"`
	ContradictScore  float64          `json:"contradict_score"`
	Grade            string           `json:"grade"`
	Detail           ContradictDetail `json:"detail"`
	SampleCount      int              `json:"sample_count"`
	LabCount         int              `json:"lab_count"`
	Concerns         int              `json:"concerns"`
}

type Report struct {
	Models     []ModelContradict `json:"models"`
	BestModel  string            `json:"best_model"`
	WorstModel string            `json:"worst_model"`
	MeanScore  float64           `json:"mean_score"`
}

var contradictPhrases = []string{
	"however, this contradicts", "on the other hand",
	"but actually", "in contrast to what i said",
	"wait, that is wrong", "actually, the opposite",
	"i take that back", "correction:",
	"contrary to my earlier", "that said, the reverse",
	"this conflicts with", "inconsistent with",
	"this disagrees with", "paradoxically",
}

var hedgePhrases = []string{
	"it depends", "it is hard to say",
	"there is no definitive answer", "it could go either way",
	"some might argue", "opinions vary",
	"this is debatable", "it is uncertain",
	"possibly", "perhaps", "maybe",
	"it is not clear", "arguably",
}

var negationPhrases = []string{
	"this is not true", "that is incorrect",
	"this is wrong", "i was mistaken",
	"that is not the case", "this is false",
	"disregard my earlier", "ignore the above",
	"my previous statement was wrong",
	"let me correct myself",
}

var retractionPhrases = []string{
	"i apologize for the confusion",
	"sorry, that was incorrect",
	"i need to correct",
	"upon reflection",
	"after reconsideration",
	"i should clarify that",
	"to be more accurate",
	"a better answer would be",
	"let me rephrase",
	"what i meant was",
}

var consistencyPhrases = []string{
	"as i mentioned", "consistent with",
	"building on this point", "to reinforce",
	"as noted earlier", "this supports",
	"in line with", "corroborating",
	"this confirms", "as established",
	"following from", "this aligns with",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, ContradictDetail) {
	combined := response + " " + evidence

	contradictions := countPhrases(combined, contradictPhrases)
	hedges := countPhrases(combined, hedgePhrases)
	negations := countPhrases(combined, negationPhrases)
	retractions := countPhrases(combined, retractionPhrases)
	consistency := countPhrases(combined, consistencyPhrases)

	consistencyScore := 0.5
	if consistency > 0 {
		consistencyScore = math.Min(1.0, float64(consistency)*0.2+0.5)
	}
	if contradictions > 0 {
		consistencyScore = math.Max(0.0, consistencyScore-float64(contradictions)*0.2)
	}

	clarityScore := 0.7
	if hedges > 2 {
		clarityScore = math.Max(0.0, clarityScore-float64(hedges-2)*0.1)
	}

	detail := ContradictDetail{
		ContradictionCount: contradictions,
		HedgeCount:         hedges,
		ConsistencyScore:   math.Round(consistencyScore*100) / 100,
		ClarityScore:       math.Round(clarityScore*100) / 100,
		NegationCount:      negations,
		RetractionCount:    retractions,
	}

	score := 50.0
	score += consistencyScore * 20.0
	score += clarityScore * 15.0

	score -= float64(contradictions) * 8.0
	score -= float64(negations) * 5.0
	score -= float64(retractions) * 3.0
	score -= float64(hedges) * 1.5

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

	var models []ModelContradict
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail ContradictDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.ContradictionCount += detail.ContradictionCount
			totalDetail.HedgeCount += detail.HedgeCount
			totalDetail.ConsistencyScore += detail.ConsistencyScore
			totalDetail.ClarityScore += detail.ClarityScore
			totalDetail.NegationCount += detail.NegationCount
			totalDetail.RetractionCount += detail.RetractionCount

			if detail.ContradictionCount > 1 || detail.NegationCount > 2 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.ConsistencyScore = math.Round(totalDetail.ConsistencyScore/n*100) / 100
		totalDetail.ClarityScore = math.Round(totalDetail.ClarityScore/n*100) / 100

		models = append(models, ModelContradict{
			Model:           mn,
			ContradictScore: math.Round(avgScore*10) / 10,
			Grade:           scoreToGrade(avgScore),
			Detail:          totalDetail,
			SampleCount:     len(entries),
			LabCount:        len(labs),
			Concerns:        totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].ContradictScore > models[j].ContradictScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.ContradictScore
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
	b.WriteString("Self-Contradiction Detection Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No contradiction data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.ContradictScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "contradiction_count", m.Detail.ContradictionCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "hedge_count", m.Detail.HedgeCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "consistency_score", m.Detail.ConsistencyScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "clarity_score", m.Detail.ClarityScore)
		fmt.Fprintf(&b, "    %-28s %d\n", "negation_count", m.Detail.NegationCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "retraction_count", m.Detail.RetractionCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
