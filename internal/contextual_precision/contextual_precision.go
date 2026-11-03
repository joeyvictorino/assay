package contextual_precision

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type PrecisionDetail struct {
	RelevanceScore  float64 `json:"relevance_score"`
	SpecificityScore float64 `json:"specificity_score"`
	AccuracyScore   float64 `json:"accuracy_score"`
	FocusScore      float64 `json:"focus_score"`
	ConcisenessScore float64 `json:"conciseness_score"`
	RelevanceCount  int     `json:"relevance_count"`
	VaguenessCount  int     `json:"vagueness_count"`
	DigressionCount int     `json:"digression_count"`
}

type ModelContextualPrecision struct {
	Model                   string          `json:"model"`
	ContextualPrecisionScore float64        `json:"contextual_precision_score"`
	Grade                   string          `json:"grade"`
	Detail                  PrecisionDetail `json:"detail"`
	SampleCount             int             `json:"sample_count"`
	LabCount                int             `json:"lab_count"`
	Concerns                int             `json:"concerns"`
}

type Report struct {
	Models     []ModelContextualPrecision `json:"models"`
	BestModel  string                     `json:"best_model"`
	WorstModel string                     `json:"worst_model"`
	MeanScore  float64                    `json:"mean_score"`
}

var relevancePhrases = []string{
	"specifically", "in this case", "given that",
	"as you mentioned", "regarding your", "for your situation",
	"in your context", "based on what you", "addressing your",
	"to answer your question", "relevant to", "pertaining to",
	"with respect to your", "concerning your", "as it relates to",
}

var specificityPhrases = []string{
	"for example", "such as", "in particular",
	"precisely", "exactly", "concretely",
	"namely", "to illustrate", "the specific",
	"in detail", "step by step", "the exact",
}

var accuracyPhrases = []string{
	"according to", "based on", "as documented",
	"verified", "confirmed", "established",
	"demonstrated", "proven", "evidence shows",
	"research indicates", "data suggests", "studies show",
}

var focusPhrases = []string{
	"the key point", "most importantly", "the core issue",
	"fundamentally", "essentially", "the main",
	"primarily", "the central", "at its heart",
	"the critical", "the essential", "the crux",
}

var vaguenessPhrases = []string{
	"it depends", "in general", "usually",
	"sometimes", "it varies", "more or less",
	"kind of", "sort of", "roughly",
	"approximately", "arguably", "potentially",
}

var digressionPhrases = []string{
	"by the way", "on a side note", "incidentally",
	"speaking of which", "tangentially", "as an aside",
	"while we're at it", "on another note", "additionally",
	"furthermore", "moreover", "also worth noting",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, PrecisionDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	relevance := countPhrases(combined, relevancePhrases)
	specificity := countPhrases(combined, specificityPhrases)
	accuracy := countPhrases(combined, accuracyPhrases)
	focus := countPhrases(combined, focusPhrases)
	vagueness := countPhrases(combined, vaguenessPhrases)
	digression := countPhrases(combined, digressionPhrases)

	relScore := 0.2
	if words > 0 {
		relScore = math.Min(1.0, float64(relevance)*2.0/float64(words)*10.0+0.2)
	}

	specScore := 0.2
	if words > 0 {
		specScore = math.Min(1.0, float64(specificity)*2.0/float64(words)*10.0+0.2)
	}

	accScore := 0.2
	if words > 0 {
		accScore = math.Min(1.0, float64(accuracy)*2.0/float64(words)*10.0+0.2)
	}

	focScore := 0.3
	if focus > 0 {
		focScore = math.Min(1.0, float64(focus)*0.2+0.3)
	}

	concScore := 0.3
	if words > 0 && words < 200 {
		concScore = math.Min(1.0, 0.3+float64(200-words)*0.003)
	}

	detail := PrecisionDetail{
		RelevanceScore:  math.Round(relScore*100) / 100,
		SpecificityScore: math.Round(specScore*100) / 100,
		AccuracyScore:   math.Round(accScore*100) / 100,
		FocusScore:      math.Round(focScore*100) / 100,
		ConcisenessScore: math.Round(concScore*100) / 100,
		RelevanceCount:  relevance,
		VaguenessCount:  vagueness,
		DigressionCount: digression,
	}

	score := 30.0
	score += relScore * 15.0
	score += specScore * 15.0
	score += accScore * 15.0
	score += focScore * 10.0
	score += concScore * 10.0

	score -= float64(vagueness) * 3.0
	score -= float64(digression) * 3.0

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

	var models []ModelContextualPrecision
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail PrecisionDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.RelevanceScore += detail.RelevanceScore
			totalDetail.SpecificityScore += detail.SpecificityScore
			totalDetail.AccuracyScore += detail.AccuracyScore
			totalDetail.FocusScore += detail.FocusScore
			totalDetail.ConcisenessScore += detail.ConcisenessScore
			totalDetail.RelevanceCount += detail.RelevanceCount
			totalDetail.VaguenessCount += detail.VaguenessCount
			totalDetail.DigressionCount += detail.DigressionCount

			if detail.VaguenessCount > 2 || detail.DigressionCount > 2 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.RelevanceScore = math.Round(totalDetail.RelevanceScore/n*100) / 100
		totalDetail.SpecificityScore = math.Round(totalDetail.SpecificityScore/n*100) / 100
		totalDetail.AccuracyScore = math.Round(totalDetail.AccuracyScore/n*100) / 100
		totalDetail.FocusScore = math.Round(totalDetail.FocusScore/n*100) / 100
		totalDetail.ConcisenessScore = math.Round(totalDetail.ConcisenessScore/n*100) / 100

		models = append(models, ModelContextualPrecision{
			Model:                   mn,
			ContextualPrecisionScore: math.Round(avgScore*10) / 10,
			Grade:                   scoreToGrade(avgScore),
			Detail:                  totalDetail,
			SampleCount:             len(entries),
			LabCount:                len(labs),
			Concerns:                totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].ContextualPrecisionScore > models[j].ContextualPrecisionScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.ContextualPrecisionScore
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
	b.WriteString("Contextual Precision Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No contextual precision data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.ContextualPrecisionScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "relevance_score", m.Detail.RelevanceScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "specificity_score", m.Detail.SpecificityScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "accuracy_score", m.Detail.AccuracyScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "focus_score", m.Detail.FocusScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "conciseness_score", m.Detail.ConcisenessScore)
		fmt.Fprintf(&b, "    %-28s %d\n", "relevance_count", m.Detail.RelevanceCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "vagueness_count", m.Detail.VaguenessCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "digression_count", m.Detail.DigressionCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
