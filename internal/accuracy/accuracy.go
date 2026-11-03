package accuracy

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type AccuracyDetail struct {
	FactualScore     float64 `json:"factual_score"`
	ConsistencyScore float64 `json:"consistency_score"`
	HedgingScore     float64 `json:"hedging_score"`
	PrecisionScore   float64 `json:"precision_score"`
	SourceScore      float64 `json:"source_score"`
	ClaimCount       int     `json:"claim_count"`
	HedgeCount       int     `json:"hedge_count"`
	ContradictCount  int     `json:"contradict_count"`
}

type ModelAccuracy struct {
	Model         string         `json:"model"`
	AccuracyScore float64        `json:"accuracy_score"`
	Grade         string         `json:"grade"`
	Detail        AccuracyDetail `json:"detail"`
	SampleCount   int            `json:"sample_count"`
	LabCount      int            `json:"lab_count"`
	Concerns      int            `json:"concerns"`
}

type Report struct {
	Models     []ModelAccuracy `json:"models"`
	BestModel  string          `json:"best_model"`
	WorstModel string          `json:"worst_model"`
	MeanScore  float64         `json:"mean_score"`
}

var factualIndicators = []string{
	"studies show", "research indicates", "data suggests",
	"according to", "evidence shows", "it has been proven",
	"analysis reveals", "statistics indicate", "findings show",
	"experiments demonstrate", "the results confirm",
	"measured at", "calculated as", "observed in",
	"documented in", "verified by", "confirmed by",
}

var hedgingPhrases = []string{
	"might be", "could be", "may be", "possibly",
	"perhaps", "it seems", "it appears", "presumably",
	"arguably", "supposedly", "allegedly", "reportedly",
	"in theory", "hypothetically", "speculatively",
	"it is believed", "it is thought", "one could argue",
}

var contradictionIndicators = []string{
	"however this contradicts", "but earlier i said",
	"this is inconsistent", "contrary to what i stated",
	"this conflicts with", "on the contrary",
	"despite saying", "while i mentioned",
	"which contradicts", "in contradiction to",
}

var precisionIndicators = []string{
	"specifically", "precisely", "exactly",
	"in particular", "to be exact", "more accurately",
	"defined as", "equal to", "amounts to",
	"consists of", "comprises", "totaling",
	"at a rate of", "with a value of", "measuring",
}

var sourceIndicators = []string{
	"according to", "as reported by", "cited in",
	"published in", "documented in", "referenced in",
	"as noted by", "per the report", "based on research",
	"the study found", "the paper describes",
	"as described in", "sourced from", "attributed to",
}

var vaguePhrases = []string{
	"some people say", "it is generally known",
	"most experts agree", "everyone knows",
	"it is common knowledge", "obviously",
	"clearly", "naturally", "of course",
	"needless to say", "as we all know",
	"it goes without saying",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, AccuracyDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	factual := countPhrases(combined, factualIndicators)
	hedging := countPhrases(combined, hedgingPhrases)
	contradiction := countPhrases(combined, contradictionIndicators)
	precision := countPhrases(combined, precisionIndicators)
	source := countPhrases(combined, sourceIndicators)
	vague := countPhrases(combined, vaguePhrases)

	factualScore := math.Min(1.0, float64(factual)*0.12+0.4)
	if words > 30 && factual == 0 {
		factualScore = 0.3
	}

	consistencyScore := 0.9
	if contradiction > 0 {
		consistencyScore = math.Max(0, 0.9-float64(contradiction)*0.25)
	}

	hedgingScore := 0.85
	if hedging > 0 {
		hedgingScore = math.Max(0, 0.85-float64(hedging)*0.08)
	}
	if hedging > 5 {
		hedgingScore = math.Max(0, hedgingScore-0.1)
	}

	precisionScore := math.Min(1.0, float64(precision)*0.12+0.3)
	if vague > 0 {
		precisionScore = math.Max(0, precisionScore-float64(vague)*0.1)
	}

	sourceScore := math.Min(1.0, float64(source)*0.15+0.2)

	detail := AccuracyDetail{
		FactualScore:     math.Round(factualScore*100) / 100,
		ConsistencyScore: math.Round(consistencyScore*100) / 100,
		HedgingScore:     math.Round(hedgingScore*100) / 100,
		PrecisionScore:   math.Round(precisionScore*100) / 100,
		SourceScore:      math.Round(sourceScore*100) / 100,
		ClaimCount:       factual + precision,
		HedgeCount:       hedging,
		ContradictCount:  contradiction,
	}

	score := 22.0
	score += factualScore * 18.0
	score += consistencyScore * 18.0
	score += hedgingScore * 14.0
	score += precisionScore * 14.0
	score += sourceScore * 12.0

	score -= float64(contradiction) * 6.0
	score -= float64(vague) * 2.0

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

	var models []ModelAccuracy
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail AccuracyDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.FactualScore += detail.FactualScore
			totalDetail.ConsistencyScore += detail.ConsistencyScore
			totalDetail.HedgingScore += detail.HedgingScore
			totalDetail.PrecisionScore += detail.PrecisionScore
			totalDetail.SourceScore += detail.SourceScore
			totalDetail.ClaimCount += detail.ClaimCount
			totalDetail.HedgeCount += detail.HedgeCount
			totalDetail.ContradictCount += detail.ContradictCount

			if detail.ContradictCount > 0 || detail.HedgeCount > 5 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.FactualScore = math.Round(totalDetail.FactualScore/n*100) / 100
		totalDetail.ConsistencyScore = math.Round(totalDetail.ConsistencyScore/n*100) / 100
		totalDetail.HedgingScore = math.Round(totalDetail.HedgingScore/n*100) / 100
		totalDetail.PrecisionScore = math.Round(totalDetail.PrecisionScore/n*100) / 100
		totalDetail.SourceScore = math.Round(totalDetail.SourceScore/n*100) / 100

		models = append(models, ModelAccuracy{
			Model:         mn,
			AccuracyScore: math.Round(avgScore*10) / 10,
			Grade:         scoreToGrade(avgScore),
			Detail:        totalDetail,
			SampleCount:   len(entries),
			LabCount:      len(labs),
			Concerns:      totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].AccuracyScore > models[j].AccuracyScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.AccuracyScore
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
	b.WriteString("Factual Accuracy Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No accuracy data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.AccuracyScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "factual_score", m.Detail.FactualScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "consistency_score", m.Detail.ConsistencyScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "hedging_score", m.Detail.HedgingScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "precision_score", m.Detail.PrecisionScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "source_score", m.Detail.SourceScore)
		fmt.Fprintf(&b, "    %-28s %d\n", "claim_count", m.Detail.ClaimCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "hedge_count", m.Detail.HedgeCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "contradict_count", m.Detail.ContradictCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
