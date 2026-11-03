package counterfactual

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type CounterfactualDetail struct {
	AlternativeGeneration float64 `json:"alternative_generation"`
	ConsequenceTracing    float64 `json:"consequence_tracing"`
	PremiseVariation      float64 `json:"premise_variation"`
	OutcomeComparison     float64 `json:"outcome_comparison"`
	CausalReasoning       float64 `json:"causal_reasoning"`
}

type ModelCounterfactual struct {
	Model    string               `json:"model"`
	Lab      string               `json:"lab"`
	Score    float64              `json:"score"`
	Grade    string               `json:"grade"`
	Count    int                  `json:"count"`
	Detail   CounterfactualDetail `json:"detail"`
	Evidence string               `json:"evidence"`
	Concerns []string             `json:"concerns,omitempty"`
}

type Report struct {
	Title  string                `json:"title"`
	Models []ModelCounterfactual `json:"models"`
}

var alternativePhrases = []string{
	"what if", "alternatively", "suppose instead", "had we chosen",
	"in a different scenario", "if instead", "another possibility",
	"were it the case", "imagining that", "hypothetically if",
}

var consequencePhrases = []string{
	"would result in", "the consequence would be", "leading to",
	"this would cause", "the outcome shifts", "then we would see",
	"as a result", "downstream effect", "ripple effect", "follow-on",
}

var premisePhrases = []string{
	"changing the assumption", "if the premise were", "altering the condition",
	"given different initial", "modifying the starting", "varying the input",
	"under different circumstances", "with a revised basis", "reframing the setup",
	"adjusting the constraint",
}

var outcomePhrases = []string{
	"compared to the original", "differs from the baseline", "in contrast",
	"versus the actual", "the delta between", "relative to what happened",
	"side by side", "juxtaposing outcomes", "diverges from", "unlike the real",
}

var causalPhrases = []string{
	"because of this change", "the causal link", "directly causes",
	"is responsible for", "brings about", "triggers the shift",
	"mechanism behind", "causal chain", "root cause shifts", "causally connected",
}

var speculative = []string{
	"maybe something", "it could be anything", "who knows what",
	"impossible to say", "purely random", "no way to tell",
}

var contradictory = []string{
	"both would and would not", "simultaneously true and false",
	"the same yet different outcome", "contradicts itself",
	"logically impossible result", "paradoxically identical",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	n := 0
	for _, p := range phrases {
		n += strings.Count(lower, p)
	}
	return n
}

func scoreResponse(text string) (float64, CounterfactualDetail) {
	alt := countPhrases(text, alternativePhrases)
	con := countPhrases(text, consequencePhrases)
	pre := countPhrases(text, premisePhrases)
	out := countPhrases(text, outcomePhrases)
	cau := countPhrases(text, causalPhrases)
	spec := countPhrases(text, speculative)
	contra := countPhrases(text, contradictory)

	d := CounterfactualDetail{
		AlternativeGeneration: math.Min(float64(alt)*12, 100),
		ConsequenceTracing:    math.Min(float64(con)*12, 100),
		PremiseVariation:      math.Min(float64(pre)*14, 100),
		OutcomeComparison:     math.Min(float64(out)*14, 100),
		CausalReasoning:       math.Min(float64(cau)*12, 100),
	}

	score := 30.0 +
		d.AlternativeGeneration*0.15 +
		d.ConsequenceTracing*0.15 +
		d.PremiseVariation*0.12 +
		d.OutcomeComparison*0.12 +
		d.CausalReasoning*0.16

	score -= float64(spec) * 3
	score -= float64(contra) * 4

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return score, d
}

func scoreToGrade(s float64) string {
	switch {
	case s >= 90:
		return "A"
	case s >= 80:
		return "B"
	case s >= 70:
		return "C"
	case s >= 60:
		return "D"
	default:
		return "F"
	}
}

func evidenceText(text string) string {
	if len(text) > 120 {
		return text[:120] + "..."
	}
	return text
}

func Analyze(findings []model.Finding) Report {
	type key struct{ m, l string }
	buckets := map[key][]model.Finding{}
	for _, f := range findings {
		k := key{f.Model.Model, f.Lab}
		buckets[k] = append(buckets[k], f)
	}

	var models []ModelCounterfactual
	for k, fs := range buckets {
		var totalScore float64
		var totalDetail CounterfactualDetail
		var sampleEvidence string
		specCount := 0
		contraCount := 0

		for _, f := range fs {
			text := f.Summary
			for _, e := range f.Evidence {
				text += " " + e.Marker
			}
			s, d := scoreResponse(text)
			totalScore += s
			totalDetail.AlternativeGeneration += d.AlternativeGeneration
			totalDetail.ConsequenceTracing += d.ConsequenceTracing
			totalDetail.PremiseVariation += d.PremiseVariation
			totalDetail.OutcomeComparison += d.OutcomeComparison
			totalDetail.CausalReasoning += d.CausalReasoning
			specCount += countPhrases(text, speculative)
			contraCount += countPhrases(text, contradictory)
			if sampleEvidence == "" {
				sampleEvidence = evidenceText(text)
			}
		}

		n := float64(len(fs))
		mean := totalScore / n
		mc := ModelCounterfactual{
			Model: k.m,
			Lab:   k.l,
			Score: math.Round(mean*10) / 10,
			Grade: scoreToGrade(mean),
			Count: len(fs),
			Detail: CounterfactualDetail{
				AlternativeGeneration: math.Round(totalDetail.AlternativeGeneration/n*10) / 10,
				ConsequenceTracing:    math.Round(totalDetail.ConsequenceTracing/n*10) / 10,
				PremiseVariation:      math.Round(totalDetail.PremiseVariation/n*10) / 10,
				OutcomeComparison:     math.Round(totalDetail.OutcomeComparison/n*10) / 10,
				CausalReasoning:       math.Round(totalDetail.CausalReasoning/n*10) / 10,
			},
			Evidence: sampleEvidence,
		}
		if specCount > 3 {
			mc.Concerns = append(mc.Concerns, fmt.Sprintf("high speculative count (%d)", specCount))
		}
		if contraCount > 2 {
			mc.Concerns = append(mc.Concerns, fmt.Sprintf("contradictory reasoning detected (%d)", contraCount))
		}
		models = append(models, mc)
	}

	return Report{Title: "Counterfactual Reasoning Analysis", Models: models}
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString(r.Title + "\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")
	for _, m := range r.Models {
		b.WriteString(fmt.Sprintf("Model: %s  Lab: %s\n", m.Model, m.Lab))
		b.WriteString(fmt.Sprintf("  Score: %.1f  Grade: %s  Findings: %d\n", m.Score, m.Grade, m.Count))
		b.WriteString(fmt.Sprintf("  Alternative: %.1f  Consequence: %.1f  Premise: %.1f\n",
			m.Detail.AlternativeGeneration, m.Detail.ConsequenceTracing, m.Detail.PremiseVariation))
		b.WriteString(fmt.Sprintf("  Outcome: %.1f  Causal: %.1f\n",
			m.Detail.OutcomeComparison, m.Detail.CausalReasoning))
		if len(m.Concerns) > 0 {
			b.WriteString(fmt.Sprintf("  Concerns: %s\n", strings.Join(m.Concerns, "; ")))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
