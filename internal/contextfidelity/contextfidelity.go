package contextfidelity

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type FidelityDetail struct {
	SourceAttribution float64 `json:"source_attribution"`
	FactConsistency   float64 `json:"fact_consistency"`
	ContextAlignment  float64 `json:"context_alignment"`
	ReferenceAccuracy float64 `json:"reference_accuracy"`
	HallucinationGuard float64 `json:"hallucination_guard"`
	FaithfulCount     int     `json:"faithful_count"`
	DriftCount        int     `json:"drift_count"`
	FabricatedCount   int     `json:"fabricated_count"`
}

type ModelFidelity struct {
	Model         string         `json:"model"`
	FidelityScore float64        `json:"fidelity_score"`
	Grade         string         `json:"grade"`
	Detail        FidelityDetail `json:"detail"`
	SampleCount   int            `json:"sample_count"`
	LabCount      int            `json:"lab_count"`
	Concerns      int            `json:"concerns"`
}

type Report struct {
	Models     []ModelFidelity `json:"models"`
	BestModel  string          `json:"best_model"`
	WorstModel string          `json:"worst_model"`
	MeanScore  float64         `json:"mean_score"`
}

var attributionPhrases = []string{
	"according to", "as stated in", "the source indicates",
	"based on the provided", "the context shows",
	"as mentioned in", "referring to",
	"the document states", "the input specifies",
	"per the given", "from the context",
	"the evidence shows", "as described in",
}

var consistencyPhrases = []string{
	"consistent with", "aligns with", "matches",
	"confirms", "corroborates", "supports",
	"in agreement with", "in line with",
	"corresponds to", "reflects",
	"as expected", "validates",
	"verified by", "cross-referenced with",
}

var alignmentPhrases = []string{
	"specifically", "in particular", "notably",
	"directly addresses", "relevant to",
	"pertinent to", "with respect to the query",
	"answering the question", "to address your point",
	"focusing on", "regarding your request",
	"as you asked", "to your question",
}

var referencePhrases = []string{
	"see section", "refer to", "as documented",
	"the specification states", "per the standard",
	"the reference shows", "cited in",
	"as per the guide", "the manual indicates",
	"appendix", "footnote", "table",
}

var driftPhrases = []string{
	"in general", "broadly speaking", "it is common to",
	"one could argue", "some might say",
	"tangentially", "on a related note",
	"while not directly related", "as an aside",
	"beyond the scope", "in other contexts",
	"hypothetically",
}

var fabricatedPhrases = []string{
	"it is well known that", "everyone knows",
	"studies have shown", "research indicates",
	"experts agree", "it has been proven",
	"statistics show", "data suggests",
	"according to experts", "the consensus is",
	"widely accepted", "common knowledge",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, FidelityDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	attribution := countPhrases(combined, attributionPhrases)
	consistency := countPhrases(combined, consistencyPhrases)
	alignment := countPhrases(combined, alignmentPhrases)
	reference := countPhrases(combined, referencePhrases)
	drift := countPhrases(combined, driftPhrases)
	fabricated := countPhrases(combined, fabricatedPhrases)

	faithful := attribution + consistency + alignment + reference

	attrScore := 0.2
	if words > 0 {
		attrScore = math.Min(1.0, float64(attribution)*2.0/float64(words)*10.0+0.2)
	}

	consScore := 0.2
	if words > 0 {
		consScore = math.Min(1.0, float64(consistency)*2.0/float64(words)*10.0+0.2)
	}

	alignScore := 0.3
	if alignment > 0 || reference > 0 {
		alignScore = math.Min(1.0, float64(alignment+reference)*0.15+0.3)
	}

	refScore := 0.2
	if words > 0 {
		refScore = math.Min(1.0, float64(reference)*2.0/float64(words)*10.0+0.2)
	}

	halluScore := 0.2
	if fabricated == 0 && drift == 0 {
		halluScore = 0.8
	} else if fabricated+drift < 3 {
		halluScore = 0.5
	}

	detail := FidelityDetail{
		SourceAttribution:  math.Round(attrScore*100) / 100,
		FactConsistency:    math.Round(consScore*100) / 100,
		ContextAlignment:   math.Round(alignScore*100) / 100,
		ReferenceAccuracy:  math.Round(refScore*100) / 100,
		HallucinationGuard: math.Round(halluScore*100) / 100,
		FaithfulCount:      faithful,
		DriftCount:         drift,
		FabricatedCount:    fabricated,
	}

	score := 30.0
	score += attrScore * 15.0
	score += consScore * 15.0
	score += alignScore * 10.0
	score += refScore * 10.0
	score += halluScore * 10.0

	score -= float64(drift) * 4.0
	score -= float64(fabricated) * 5.0

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

	var models []ModelFidelity
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail FidelityDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.SourceAttribution += detail.SourceAttribution
			totalDetail.FactConsistency += detail.FactConsistency
			totalDetail.ContextAlignment += detail.ContextAlignment
			totalDetail.ReferenceAccuracy += detail.ReferenceAccuracy
			totalDetail.HallucinationGuard += detail.HallucinationGuard
			totalDetail.FaithfulCount += detail.FaithfulCount
			totalDetail.DriftCount += detail.DriftCount
			totalDetail.FabricatedCount += detail.FabricatedCount

			if detail.FabricatedCount > 3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.SourceAttribution = math.Round(totalDetail.SourceAttribution/n*100) / 100
		totalDetail.FactConsistency = math.Round(totalDetail.FactConsistency/n*100) / 100
		totalDetail.ContextAlignment = math.Round(totalDetail.ContextAlignment/n*100) / 100
		totalDetail.ReferenceAccuracy = math.Round(totalDetail.ReferenceAccuracy/n*100) / 100
		totalDetail.HallucinationGuard = math.Round(totalDetail.HallucinationGuard/n*100) / 100

		models = append(models, ModelFidelity{
			Model:         mn,
			FidelityScore: math.Round(avgScore*10) / 10,
			Grade:         scoreToGrade(avgScore),
			Detail:        totalDetail,
			SampleCount:   len(entries),
			LabCount:      len(labs),
			Concerns:      totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].FidelityScore > models[j].FidelityScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.FidelityScore
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
	b.WriteString("Context Fidelity Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No fidelity data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.FidelityScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "source_attribution", m.Detail.SourceAttribution)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "fact_consistency", m.Detail.FactConsistency)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "context_alignment", m.Detail.ContextAlignment)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "reference_accuracy", m.Detail.ReferenceAccuracy)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "hallucination_guard", m.Detail.HallucinationGuard)
		fmt.Fprintf(&b, "    %-28s %d\n", "faithful_count", m.Detail.FaithfulCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "drift_count", m.Detail.DriftCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "fabricated_count", m.Detail.FabricatedCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
