package dialectic

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type DialecticDetail struct {
	ThesisClarity    float64 `json:"thesis_clarity"`
	AntithesisDepth  float64 `json:"antithesis_depth"`
	SynthesisQuality float64 `json:"synthesis_quality"`
	NuanceAwareness  float64 `json:"nuance_awareness"`
	LogicalRigor     float64 `json:"logical_rigor"`
	ThesisCount      int     `json:"thesis_count"`
	AntithesisCount  int     `json:"antithesis_count"`
	SynthesisCount   int     `json:"synthesis_count"`
}

type ModelDialectic struct {
	Model           string          `json:"model"`
	DialecticScore  float64         `json:"dialectic_score"`
	Grade           string          `json:"grade"`
	Detail          DialecticDetail `json:"detail"`
	SampleCount     int             `json:"sample_count"`
	LabCount        int             `json:"lab_count"`
	Concerns        int             `json:"concerns"`
}

type Report struct {
	Models     []ModelDialectic `json:"models"`
	BestModel  string           `json:"best_model"`
	WorstModel string           `json:"worst_model"`
	MeanScore  float64          `json:"mean_score"`
}

var thesisPhrases = []string{
	"the argument is", "the position is", "the claim is",
	"the thesis", "it is the case that", "the main point",
	"the central argument", "the primary claim", "the core idea",
	"fundamentally", "at its core", "the proposition",
	"the assertion", "the premise", "the hypothesis",
}

var antithesisPhrases = []string{
	"on the other hand", "however", "conversely",
	"critics argue", "opponents contend", "an opposing view",
	"counterargument", "counter-argument", "the objection is",
	"nevertheless", "in contrast", "alternatively",
	"a dissenting view", "some disagree", "the rebuttal",
}

var synthesisPhrases = []string{
	"synthesizing these views", "taking both into account",
	"a balanced view", "integrating these perspectives",
	"reconciling these positions", "the middle ground",
	"a more nuanced view", "combining these insights",
	"drawing from both sides", "a comprehensive view",
	"the resolution", "bridging the gap", "in balance",
}

var nuancePhrases = []string{
	"it depends on", "context matters", "there are trade-offs",
	"the nuance here", "this is complex", "multiple factors",
	"it's not black and white", "shades of gray", "the subtlety",
	"the complexity lies in", "a qualified answer", "with caveats",
	"conditionally", "in certain contexts", "depending on",
}

var rigorPhrases = []string{
	"logically follows", "therefore", "it follows that",
	"given the premises", "by this reasoning", "consequently",
	"the evidence suggests", "deductively", "inductively",
	"by analogy", "from first principles", "the inference is",
	"this entails", "this implies", "the conclusion is",
}

var weakPhrases = []string{
	"everyone knows", "obviously", "it's clear that",
	"without a doubt", "there is no question", "undeniably",
	"anyone can see", "it goes without saying",
	"no reasonable person", "common sense dictates",
	"only a fool would", "clearly the only answer",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, DialecticDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	thesis := countPhrases(combined, thesisPhrases)
	antithesis := countPhrases(combined, antithesisPhrases)
	synthesis := countPhrases(combined, synthesisPhrases)
	nuance := countPhrases(combined, nuancePhrases)
	rigor := countPhrases(combined, rigorPhrases)
	weak := countPhrases(combined, weakPhrases)

	thesisScore := 0.2
	if words > 0 {
		thesisScore = math.Min(1.0, float64(thesis)*2.0/float64(words)*10.0+0.2)
	}

	antithesisScore := 0.2
	if words > 0 {
		antithesisScore = math.Min(1.0, float64(antithesis)*2.0/float64(words)*10.0+0.2)
	}

	synthesisScore := 0.2
	if synthesis > 0 {
		synthesisScore = math.Min(1.0, float64(synthesis)*0.2+0.2)
	}

	nuanceScore := 0.2
	if words > 0 {
		nuanceScore = math.Min(1.0, float64(nuance)*2.0/float64(words)*10.0+0.2)
	}

	rigorScore := 0.2
	if rigor > 0 || thesis > 0 {
		rigorScore = math.Min(1.0, float64(rigor+thesis)*0.1+0.2)
	}

	detail := DialecticDetail{
		ThesisClarity:    math.Round(thesisScore*100) / 100,
		AntithesisDepth:  math.Round(antithesisScore*100) / 100,
		SynthesisQuality: math.Round(synthesisScore*100) / 100,
		NuanceAwareness:  math.Round(nuanceScore*100) / 100,
		LogicalRigor:     math.Round(rigorScore*100) / 100,
		ThesisCount:      thesis,
		AntithesisCount:  antithesis,
		SynthesisCount:   synthesis,
	}

	score := 25.0
	score += thesisScore * 15.0
	score += antithesisScore * 20.0
	score += synthesisScore * 15.0
	score += nuanceScore * 10.0
	score += rigorScore * 10.0

	score -= float64(weak) * 5.0

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

	var models []ModelDialectic
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail DialecticDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.ThesisClarity += detail.ThesisClarity
			totalDetail.AntithesisDepth += detail.AntithesisDepth
			totalDetail.SynthesisQuality += detail.SynthesisQuality
			totalDetail.NuanceAwareness += detail.NuanceAwareness
			totalDetail.LogicalRigor += detail.LogicalRigor
			totalDetail.ThesisCount += detail.ThesisCount
			totalDetail.AntithesisCount += detail.AntithesisCount
			totalDetail.SynthesisCount += detail.SynthesisCount

			if detail.AntithesisCount == 0 && detail.SynthesisCount == 0 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.ThesisClarity = math.Round(totalDetail.ThesisClarity/n*100) / 100
		totalDetail.AntithesisDepth = math.Round(totalDetail.AntithesisDepth/n*100) / 100
		totalDetail.SynthesisQuality = math.Round(totalDetail.SynthesisQuality/n*100) / 100
		totalDetail.NuanceAwareness = math.Round(totalDetail.NuanceAwareness/n*100) / 100
		totalDetail.LogicalRigor = math.Round(totalDetail.LogicalRigor/n*100) / 100

		models = append(models, ModelDialectic{
			Model:          mn,
			DialecticScore: math.Round(avgScore*10) / 10,
			Grade:          scoreToGrade(avgScore),
			Detail:         totalDetail,
			SampleCount:    len(entries),
			LabCount:       len(labs),
			Concerns:       totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].DialecticScore > models[j].DialecticScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.DialecticScore
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
	b.WriteString("Dialectic Reasoning Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No dialectic data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.DialecticScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "thesis_clarity", m.Detail.ThesisClarity)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "antithesis_depth", m.Detail.AntithesisDepth)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "synthesis_quality", m.Detail.SynthesisQuality)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "nuance_awareness", m.Detail.NuanceAwareness)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "logical_rigor", m.Detail.LogicalRigor)
		fmt.Fprintf(&b, "    %-28s %d\n", "thesis_count", m.Detail.ThesisCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "antithesis_count", m.Detail.AntithesisCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "synthesis_count", m.Detail.SynthesisCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
