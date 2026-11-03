package negation

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type NegationDetail struct {
	NegationAccuracy   float64 `json:"negation_accuracy"`
	DoubleNegScore     float64 `json:"double_neg_score"`
	ScopeHandling      float64 `json:"scope_handling"`
	ContradictionScore float64 `json:"contradiction_score"`
	NegationCount      int     `json:"negation_count"`
	DoubleNegCount     int     `json:"double_neg_count"`
	ContradictionCount int     `json:"contradiction_count"`
}

type ModelNegation struct {
	Model         string         `json:"model"`
	NegationScore float64        `json:"negation_score"`
	Grade         string         `json:"grade"`
	Detail        NegationDetail `json:"detail"`
	SampleCount   int            `json:"sample_count"`
	Concerns      int            `json:"concerns"`
}

type Report struct {
	Models     []ModelNegation `json:"models"`
	BestModel  string          `json:"best_model"`
	WorstModel string          `json:"worst_model"`
	MeanScore  float64         `json:"mean_score"`
}

var negationPhrases = []string{
	"not", "no", "never", "neither", "nor",
	"cannot", "can't", "won't", "shouldn't", "wouldn't",
	"doesn't", "don't", "isn't", "aren't", "wasn't",
	"weren't", "hasn't", "haven't", "hadn't",
	"without", "unless", "except", "exclude",
	"impossible", "unable", "incapable", "forbidden",
}

var doubleNegPhrases = []string{
	"not un", "not in", "not im", "not dis",
	"can't not", "don't not", "won't not",
	"never not", "no un", "isn't un",
	"not without", "not none", "not neither",
	"hardly un", "barely un", "scarcely un",
}

var scopePhrases = []string{
	"only applies to", "does not apply", "limited to",
	"except when", "unless otherwise", "in all cases except",
	"negation scope", "scope of negation", "applies only",
	"specifically excludes", "with the exception", "barring",
	"save for", "other than", "apart from",
}

var contradictionPhrases = []string{
	"contradict", "inconsistent", "conflict",
	"oppose", "contrary", "paradox",
	"at odds", "incompatible", "irreconcilable",
	"mutually exclusive", "logically impossible", "self-defeating",
	"oxymoron", "antithetical", "diametrically opposed",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, NegationDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))
	negation := countPhrases(combined, negationPhrases)
	doubleNeg := countPhrases(combined, doubleNegPhrases)
	scope := countPhrases(combined, scopePhrases)
	contradiction := countPhrases(combined, contradictionPhrases)

	negAccuracy := 0.4
	if words > 0 && negation > 0 {
		negAccuracy = math.Min(1.0, float64(negation)*1.5/float64(words)*10.0+0.4)
	}
	doubleNegScore := 0.5
	if doubleNeg > 0 {
		doubleNegScore = math.Max(0.1, doubleNegScore-float64(doubleNeg)*0.1)
	}
	scopeScore := 0.3
	if scope > 0 {
		scopeScore = math.Min(1.0, float64(scope)*0.15+0.3)
	}
	contradictionScore := 0.5
	if contradiction > 0 {
		contradictionScore = math.Max(0.1, contradictionScore-float64(contradiction)*0.08)
	}

	detail := NegationDetail{
		NegationAccuracy:   math.Round(negAccuracy*100) / 100,
		DoubleNegScore:     math.Round(doubleNegScore*100) / 100,
		ScopeHandling:      math.Round(scopeScore*100) / 100,
		ContradictionScore: math.Round(contradictionScore*100) / 100,
		NegationCount:      negation,
		DoubleNegCount:     doubleNeg,
		ContradictionCount: contradiction,
	}

	score := 25.0
	score += negAccuracy * 20.0
	score += doubleNegScore * 20.0
	score += scopeScore * 15.0
	score += contradictionScore * 15.0
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
	var models []ModelNegation
	for mn, ff := range modelEntries {
		var total float64
		var concerns int
		var detail NegationDetail
		for _, f := range ff {
			s, d := scoreResponse(f.Summary, evidenceText(f))
			total += s
			detail.NegationCount += d.NegationCount
			detail.DoubleNegCount += d.DoubleNegCount
			detail.ContradictionCount += d.ContradictionCount
			detail.NegationAccuracy += d.NegationAccuracy
			detail.DoubleNegScore += d.DoubleNegScore
			detail.ScopeHandling += d.ScopeHandling
			detail.ContradictionScore += d.ContradictionScore
			if s < 50 {
				concerns++
			}
		}
		n := float64(len(ff))
		avg := math.Round(total/n*10) / 10
		detail.NegationAccuracy = math.Round(detail.NegationAccuracy/n*100) / 100
		detail.DoubleNegScore = math.Round(detail.DoubleNegScore/n*100) / 100
		detail.ScopeHandling = math.Round(detail.ScopeHandling/n*100) / 100
		detail.ContradictionScore = math.Round(detail.ContradictionScore/n*100) / 100
		models = append(models, ModelNegation{
			Model: mn, NegationScore: avg, Grade: scoreToGrade(avg),
			Detail: detail, SampleCount: len(ff), Concerns: concerns,
		})
	}
	sort.Slice(models, func(i, j int) bool { return models[i].NegationScore > models[j].NegationScore })
	var sum float64
	for _, m := range models {
		sum += m.NegationScore
	}
	r := Report{Models: models, MeanScore: math.Round(sum/float64(len(models))*10) / 10}
	if len(models) > 0 {
		r.BestModel = models[0].Model
		r.WorstModel = models[len(models)-1].Model
	}
	return r
}

func FormatText(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Negation Understanding Correctness (%d models, mean %.1f)\n\n", len(r.Models), r.MeanScore)
	for _, m := range r.Models {
		fmt.Fprintf(&b, "  %-30s  score: %5.1f  grade: %s  samples: %d  concerns: %d\n",
			m.Model, m.NegationScore, m.Grade, m.SampleCount, m.Concerns)
		fmt.Fprintf(&b, "    negation=%.2f  double_neg=%.2f  scope=%.2f  contradiction=%.2f\n",
			m.Detail.NegationAccuracy, m.Detail.DoubleNegScore, m.Detail.ScopeHandling,
			m.Detail.ContradictionScore)
	}
	if r.BestModel != "" {
		fmt.Fprintf(&b, "\n  Best:  %s\n  Worst: %s\n", r.BestModel, r.WorstModel)
	}
	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
