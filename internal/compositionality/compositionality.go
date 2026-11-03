package compositionality

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type ComposDetail struct {
	ConceptIntegration float64 `json:"concept_integration"`
	LayeredReasoning   float64 `json:"layered_reasoning"`
	CrossDomainLink    float64 `json:"cross_domain_link"`
	StructuralCoherence float64 `json:"structural_coherence"`
	SynthesisDepth     float64 `json:"synthesis_depth"`
	CompositeCount     int     `json:"composite_count"`
	FragmentedCount    int     `json:"fragmented_count"`
	ShallowCount       int     `json:"shallow_count"`
}

type ModelCompos struct {
	Model       string       `json:"model"`
	ComposScore float64      `json:"compositionality_score"`
	Grade       string       `json:"grade"`
	Detail      ComposDetail `json:"detail"`
	SampleCount int          `json:"sample_count"`
	LabCount    int          `json:"lab_count"`
	Concerns    int          `json:"concerns"`
}

type Report struct {
	Models     []ModelCompos `json:"models"`
	BestModel  string        `json:"best_model"`
	WorstModel string        `json:"worst_model"`
	MeanScore  float64       `json:"mean_score"`
}

var integrationPhrases = []string{
	"combining", "integrating", "merging",
	"bringing together", "synthesizing", "unifying",
	"in conjunction with", "alongside", "together with",
	"builds upon", "extends to", "connects with",
	"complementary to", "in combination", "holistically",
}

var layeredPhrases = []string{
	"at a deeper level", "more fundamentally",
	"building on this", "taking this further",
	"at the surface level", "underneath",
	"the first layer", "the second aspect",
	"from a higher perspective", "zooming in",
	"the broader picture", "drilling down",
	"at the core", "peeling back", "nested within",
}

var crossDomainPhrases = []string{
	"from a different field", "borrowing from",
	"analogous to", "cross-disciplinary",
	"in other domains", "transferring the concept",
	"interdisciplinary", "multi-faceted",
	"drawing from", "applies equally to",
	"transcends the boundary", "bridges the gap",
	"across fields", "domain-agnostic", "universally",
}

var coherencePhrases = []string{
	"therefore", "consequently", "as a result",
	"this leads to", "it follows that",
	"given the above", "taken together",
	"the overall structure", "in summary",
	"the unifying theme", "tying it together",
	"the common thread", "overarching",
	"the key takeaway", "in conclusion",
}

var fragmentedPhrases = []string{
	"on the other hand", "separately",
	"in isolation", "unrelated to",
	"independently", "without connection",
	"moving on to a different", "switching topics",
	"in a separate vein", "tangentially",
	"aside from", "disconnected",
	"shifting gears", "pivoting to",
}

var shallowPhrases = []string{
	"simply put", "basically",
	"in simple terms", "just",
	"nothing more than", "merely",
	"at face value", "superficially",
	"without going deeper", "on the surface",
	"a quick overview", "briefly",
	"in a nutshell",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, ComposDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	integration := countPhrases(combined, integrationPhrases)
	layered := countPhrases(combined, layeredPhrases)
	crossDomain := countPhrases(combined, crossDomainPhrases)
	coherence := countPhrases(combined, coherencePhrases)
	fragmented := countPhrases(combined, fragmentedPhrases)
	shallow := countPhrases(combined, shallowPhrases)

	composite := integration + layered + crossDomain + coherence

	integrationScore := 0.2
	if words > 0 {
		integrationScore = math.Min(1.0, float64(integration)*2.0/float64(words)*10.0+0.2)
	}

	layeredScore := 0.2
	if words > 0 {
		layeredScore = math.Min(1.0, float64(layered)*2.0/float64(words)*10.0+0.2)
	}

	crossDomainScore := 0.2
	if crossDomain > 0 || layered > 0 {
		crossDomainScore = math.Min(1.0, float64(crossDomain+layered)*0.12+0.2)
	}

	coherenceScore := 0.3
	if words > 0 {
		coherenceScore = math.Min(1.0, float64(coherence)*2.0/float64(words)*10.0+0.3)
	}

	synthesisScore := 0.2
	if fragmented == 0 && shallow == 0 {
		synthesisScore = 0.8
	} else if fragmented+shallow < 3 {
		synthesisScore = 0.5
	}

	detail := ComposDetail{
		ConceptIntegration:  math.Round(integrationScore*100) / 100,
		LayeredReasoning:    math.Round(layeredScore*100) / 100,
		CrossDomainLink:     math.Round(crossDomainScore*100) / 100,
		StructuralCoherence: math.Round(coherenceScore*100) / 100,
		SynthesisDepth:      math.Round(synthesisScore*100) / 100,
		CompositeCount:      composite,
		FragmentedCount:     fragmented,
		ShallowCount:        shallow,
	}

	score := 30.0
	score += integrationScore * 15.0
	score += layeredScore * 15.0
	score += crossDomainScore * 10.0
	score += coherenceScore * 10.0
	score += synthesisScore * 10.0

	score -= float64(fragmented) * 3.0
	score -= float64(shallow) * 4.0

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

	var models []ModelCompos
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail ComposDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.ConceptIntegration += detail.ConceptIntegration
			totalDetail.LayeredReasoning += detail.LayeredReasoning
			totalDetail.CrossDomainLink += detail.CrossDomainLink
			totalDetail.StructuralCoherence += detail.StructuralCoherence
			totalDetail.SynthesisDepth += detail.SynthesisDepth
			totalDetail.CompositeCount += detail.CompositeCount
			totalDetail.FragmentedCount += detail.FragmentedCount
			totalDetail.ShallowCount += detail.ShallowCount

			if detail.FragmentedCount > 3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.ConceptIntegration = math.Round(totalDetail.ConceptIntegration/n*100) / 100
		totalDetail.LayeredReasoning = math.Round(totalDetail.LayeredReasoning/n*100) / 100
		totalDetail.CrossDomainLink = math.Round(totalDetail.CrossDomainLink/n*100) / 100
		totalDetail.StructuralCoherence = math.Round(totalDetail.StructuralCoherence/n*100) / 100
		totalDetail.SynthesisDepth = math.Round(totalDetail.SynthesisDepth/n*100) / 100

		models = append(models, ModelCompos{
			Model:       mn,
			ComposScore: math.Round(avgScore*10) / 10,
			Grade:       scoreToGrade(avgScore),
			Detail:      totalDetail,
			SampleCount: len(entries),
			LabCount:    len(labs),
			Concerns:    totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].ComposScore > models[j].ComposScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.ComposScore
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
	b.WriteString("Compositional Reasoning Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No compositionality data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.ComposScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "concept_integration", m.Detail.ConceptIntegration)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "layered_reasoning", m.Detail.LayeredReasoning)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "cross_domain_link", m.Detail.CrossDomainLink)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "structural_coherence", m.Detail.StructuralCoherence)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "synthesis_depth", m.Detail.SynthesisDepth)
		fmt.Fprintf(&b, "    %-28s %d\n", "composite_count", m.Detail.CompositeCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "fragmented_count", m.Detail.FragmentedCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "shallow_count", m.Detail.ShallowCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
