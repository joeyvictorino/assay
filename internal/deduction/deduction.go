package deduction

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type DeductionDetail struct {
	PremiseIdentification float64 `json:"premise_identification"`
	LogicalChaining       float64 `json:"logical_chaining"`
	ContrapositivUse      float64 `json:"contrapositive_use"`
	SyllogisticReasoning  float64 `json:"syllogistic_reasoning"`
	ConclusionValidity    float64 `json:"conclusion_validity"`
}

type ModelDeduction struct {
	Model    string          `json:"model"`
	Lab      string          `json:"lab"`
	Score    float64         `json:"score"`
	Grade    string          `json:"grade"`
	Count    int             `json:"count"`
	Detail   DeductionDetail `json:"detail"`
	Evidence string          `json:"evidence"`
	Concerns []string        `json:"concerns,omitempty"`
}

type Report struct {
	Title  string           `json:"title"`
	Models []ModelDeduction `json:"models"`
}

var premisePhrases = []string{
	"given that", "assuming that", "if we accept", "the premise states",
	"it is established that", "from the given", "taking as fact",
	"based on the axiom", "starting from", "the foundation is",
}

var chainingPhrases = []string{
	"therefore", "it follows that", "consequently", "which implies",
	"from this we derive", "leading to the conclusion", "hence",
	"by extension", "this entails", "we can deduce",
}

var contrapositivePhrases = []string{
	"contrapositive", "if not", "the negation implies", "conversely if",
	"by denying", "the inverse tells us", "modus tollens",
	"if the conclusion fails", "negating the consequent", "by contradiction",
}

var syllogismPhrases = []string{
	"all A are B", "if all", "since every", "no X is Y",
	"some are", "the major premise", "minor premise",
	"middle term", "universal affirmative", "particular negative",
}

var validityPhrases = []string{
	"valid conclusion", "logically follows", "sound argument",
	"the deduction holds", "formally valid", "truth-preserving",
	"necessarily true", "logically necessary", "demonstrably correct",
	"proven by deduction",
}

var fallacyPhrases = []string{
	"affirming the consequent", "denying the antecedent",
	"non sequitur", "begging the question", "circular reasoning",
	"hasty generalization",
}

var vaguePhrases = []string{
	"probably maybe", "it might or might not", "hard to say",
	"could go either way", "unclear logic", "reasoning is fuzzy",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	n := 0
	for _, p := range phrases {
		n += strings.Count(lower, p)
	}
	return n
}

func scoreResponse(text string) (float64, DeductionDetail) {
	pre := countPhrases(text, premisePhrases)
	chain := countPhrases(text, chainingPhrases)
	contra := countPhrases(text, contrapositivePhrases)
	syl := countPhrases(text, syllogismPhrases)
	val := countPhrases(text, validityPhrases)
	fallacy := countPhrases(text, fallacyPhrases)
	vague := countPhrases(text, vaguePhrases)

	d := DeductionDetail{
		PremiseIdentification: math.Min(float64(pre)*12, 100),
		LogicalChaining:       math.Min(float64(chain)*10, 100),
		ContrapositivUse:      math.Min(float64(contra)*16, 100),
		SyllogisticReasoning:  math.Min(float64(syl)*14, 100),
		ConclusionValidity:    math.Min(float64(val)*12, 100),
	}

	score := 30.0 +
		d.PremiseIdentification*0.14 +
		d.LogicalChaining*0.16 +
		d.ContrapositivUse*0.12 +
		d.SyllogisticReasoning*0.12 +
		d.ConclusionValidity*0.16

	score -= float64(fallacy) * 3
	score -= float64(vague) * 4

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

	var models []ModelDeduction
	for k, fs := range buckets {
		var totalScore float64
		var totalDetail DeductionDetail
		var sampleEvidence string
		fallacyCount := 0
		vagueCount := 0

		for _, f := range fs {
			text := f.Summary
			for _, e := range f.Evidence {
				text += " " + e.Marker
			}
			s, d := scoreResponse(text)
			totalScore += s
			totalDetail.PremiseIdentification += d.PremiseIdentification
			totalDetail.LogicalChaining += d.LogicalChaining
			totalDetail.ContrapositivUse += d.ContrapositivUse
			totalDetail.SyllogisticReasoning += d.SyllogisticReasoning
			totalDetail.ConclusionValidity += d.ConclusionValidity
			fallacyCount += countPhrases(text, fallacyPhrases)
			vagueCount += countPhrases(text, vaguePhrases)
			if sampleEvidence == "" {
				sampleEvidence = evidenceText(text)
			}
		}

		n := float64(len(fs))
		mean := totalScore / n
		md := ModelDeduction{
			Model: k.m,
			Lab:   k.l,
			Score: math.Round(mean*10) / 10,
			Grade: scoreToGrade(mean),
			Count: len(fs),
			Detail: DeductionDetail{
				PremiseIdentification: math.Round(totalDetail.PremiseIdentification/n*10) / 10,
				LogicalChaining:       math.Round(totalDetail.LogicalChaining/n*10) / 10,
				ContrapositivUse:      math.Round(totalDetail.ContrapositivUse/n*10) / 10,
				SyllogisticReasoning:  math.Round(totalDetail.SyllogisticReasoning/n*10) / 10,
				ConclusionValidity:    math.Round(totalDetail.ConclusionValidity/n*10) / 10,
			},
			Evidence: sampleEvidence,
		}
		if fallacyCount > 3 {
			md.Concerns = append(md.Concerns, fmt.Sprintf("high fallacy count (%d)", fallacyCount))
		}
		if vagueCount > 2 {
			md.Concerns = append(md.Concerns, fmt.Sprintf("vague reasoning detected (%d)", vagueCount))
		}
		models = append(models, md)
	}

	return Report{Title: "Deductive Reasoning Analysis", Models: models}
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString(r.Title + "\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")
	for _, m := range r.Models {
		b.WriteString(fmt.Sprintf("Model: %s  Lab: %s\n", m.Model, m.Lab))
		b.WriteString(fmt.Sprintf("  Score: %.1f  Grade: %s  Findings: %d\n", m.Score, m.Grade, m.Count))
		b.WriteString(fmt.Sprintf("  Premise: %.1f  Chaining: %.1f  Contrapositive: %.1f\n",
			m.Detail.PremiseIdentification, m.Detail.LogicalChaining, m.Detail.ContrapositivUse))
		b.WriteString(fmt.Sprintf("  Syllogistic: %.1f  Validity: %.1f\n",
			m.Detail.SyllogisticReasoning, m.Detail.ConclusionValidity))
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
