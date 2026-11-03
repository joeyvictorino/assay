package concreteness

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type ConcreteDetail struct {
	SpecificityScore  float64 `json:"specificity_score"`
	ExampleScore      float64 `json:"example_score"`
	QuantitativeScore float64 `json:"quantitative_score"`
	ActionableScore   float64 `json:"actionable_score"`
	EvidenceScore     float64 `json:"evidence_score"`
	SpecificCount     int     `json:"specific_count"`
	VagueCount        int     `json:"vague_count"`
	ExampleCount      int     `json:"example_count"`
}

type ModelConcrete struct {
	Model          string         `json:"model"`
	ConcreteScore  float64        `json:"concrete_score"`
	Grade          string         `json:"grade"`
	Detail         ConcreteDetail `json:"detail"`
	SampleCount    int            `json:"sample_count"`
	LabCount       int            `json:"lab_count"`
	Concerns       int            `json:"concerns"`
}

type Report struct {
	Models     []ModelConcrete `json:"models"`
	BestModel  string          `json:"best_model"`
	WorstModel string          `json:"worst_model"`
	MeanScore  float64         `json:"mean_score"`
}

var specificityPhrases = []string{
	"specifically", "in particular", "for example", "such as", "namely",
	"including", "e.g.", "i.e.", "to illustrate", "concretely",
	"the exact", "precisely", "the specific", "in this case", "as shown",
}

var vaguePhrases = []string{
	"generally", "usually", "often", "sometimes", "typically",
	"in general", "broadly", "roughly", "approximately", "more or less",
	"some kind of", "a type of", "various", "several things", "stuff",
}

var examplePhrases = []string{
	"for instance", "for example", "such as", "e.g.", "consider",
	"to illustrate", "as an example", "like", "imagine", "suppose",
	"take the case", "picture this", "here is an example", "a concrete case", "demonstration",
}

var quantitativePhrases = []string{
	"percent", "%", "ratio", "rate of", "frequency",
	"count of", "number of", "measured", "quantified", "calculated",
	"on average", "median", "approximately", "total of", "sum of",
}

var actionablePhrases = []string{
	"step 1", "step 2", "first,", "then,", "next,",
	"run the command", "execute", "apply", "implement", "configure",
	"set up", "install", "create a", "add the", "modify the",
}

var evidencePhrases = []string{
	"according to", "as stated in", "the documentation shows", "the log indicates", "the output shows",
	"the error message", "the stack trace", "the metric", "the benchmark", "the test result",
	"as reported", "the data shows", "the analysis reveals", "observation", "measurement",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, ConcreteDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	specific := countPhrases(combined, specificityPhrases)
	vague := countPhrases(combined, vaguePhrases)
	examples := countPhrases(combined, examplePhrases)
	quantitative := countPhrases(combined, quantitativePhrases)
	actionable := countPhrases(combined, actionablePhrases)
	evidenceCount := countPhrases(combined, evidencePhrases)

	specificityScore := 0.3
	if specific > 0 {
		specificityScore = math.Min(1.0, float64(specific)*0.12+0.3)
	}
	if vague > 0 {
		specificityScore = math.Max(0, specificityScore-float64(vague)*0.08)
	}

	exampleScore := 0.2
	if examples > 0 {
		exampleScore = math.Min(1.0, float64(examples)*0.15+0.2)
	}

	quantitativeScore := 0.2
	if quantitative > 0 {
		quantitativeScore = math.Min(1.0, float64(quantitative)*0.12+0.2)
	}

	actionableScore := 0.2
	if actionable > 0 {
		actionableScore = math.Min(1.0, float64(actionable)*0.1+0.2)
	}

	evidenceScore := 0.2
	if evidenceCount > 0 {
		evidenceScore = math.Min(1.0, float64(evidenceCount)*0.12+0.2)
	}

	_ = words

	detail := ConcreteDetail{
		SpecificityScore:  math.Round(specificityScore*100) / 100,
		ExampleScore:      math.Round(exampleScore*100) / 100,
		QuantitativeScore: math.Round(quantitativeScore*100) / 100,
		ActionableScore:   math.Round(actionableScore*100) / 100,
		EvidenceScore:     math.Round(evidenceScore*100) / 100,
		SpecificCount:     specific,
		VagueCount:        vague,
		ExampleCount:      examples,
	}

	score := 20.0
	score += specificityScore * 20.0
	score += exampleScore * 15.0
	score += quantitativeScore * 15.0
	score += actionableScore * 15.0
	score += evidenceScore * 10.0
	score -= float64(vague) * 2.5

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

	var models []ModelConcrete
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail ConcreteDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.SpecificityScore += detail.SpecificityScore
			totalDetail.ExampleScore += detail.ExampleScore
			totalDetail.QuantitativeScore += detail.QuantitativeScore
			totalDetail.ActionableScore += detail.ActionableScore
			totalDetail.EvidenceScore += detail.EvidenceScore
			totalDetail.SpecificCount += detail.SpecificCount
			totalDetail.VagueCount += detail.VagueCount
			totalDetail.ExampleCount += detail.ExampleCount

			if detail.VagueCount > 3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.SpecificityScore = math.Round(totalDetail.SpecificityScore/n*100) / 100
		totalDetail.ExampleScore = math.Round(totalDetail.ExampleScore/n*100) / 100
		totalDetail.QuantitativeScore = math.Round(totalDetail.QuantitativeScore/n*100) / 100
		totalDetail.ActionableScore = math.Round(totalDetail.ActionableScore/n*100) / 100
		totalDetail.EvidenceScore = math.Round(totalDetail.EvidenceScore/n*100) / 100

		models = append(models, ModelConcrete{
			Model:         mn,
			ConcreteScore: math.Round(avgScore*10) / 10,
			Grade:         scoreToGrade(avgScore),
			Detail:        totalDetail,
			SampleCount:   len(entries),
			LabCount:      len(labs),
			Concerns:      totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].ConcreteScore > models[j].ConcreteScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.ConcreteScore
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
	b.WriteString("Concreteness Quality Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No concreteness data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.ConcreteScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "specificity_score", m.Detail.SpecificityScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "example_score", m.Detail.ExampleScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "quantitative_score", m.Detail.QuantitativeScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "actionable_score", m.Detail.ActionableScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "evidence_score", m.Detail.EvidenceScore)
		fmt.Fprintf(&b, "    %-28s %d\n", "specific_count", m.Detail.SpecificCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "vague_count", m.Detail.VagueCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "example_count", m.Detail.ExampleCount)
		fmt.Fprintf(&b, "    Concerns: %d\n\n", m.Concerns)
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
