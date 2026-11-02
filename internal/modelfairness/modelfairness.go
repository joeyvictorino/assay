package modelfairness

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type ClassDistribution struct {
	Class   string  `json:"class"`
	Count   int     `json:"count"`
	Percent float64 `json:"percent"`
}

type SeverityDistribution struct {
	Severity string  `json:"severity"`
	Count    int     `json:"count"`
	Percent  float64 `json:"percent"`
}

type ModelFairness struct {
	Model               string                 `json:"model"`
	TotalFindings       int                    `json:"total_findings"`
	ClassDistributions  []ClassDistribution    `json:"class_distributions"`
	SeverityDist        []SeverityDistribution `json:"severity_distributions"`
	ClassGini           float64                `json:"class_gini"`
	SeverityGini        float64                `json:"severity_gini"`
	FairnessScore       float64                `json:"fairness_score"`
	Grade               string                 `json:"grade"`
	MostReportedClass   string                 `json:"most_reported_class"`
	LeastReportedClass  string                 `json:"least_reported_class"`
	ClassConcentration  float64                `json:"class_concentration"`
}

type PairwiseComparison struct {
	ModelA         string  `json:"model_a"`
	ModelB         string  `json:"model_b"`
	Divergence     float64 `json:"divergence"`
	AgreementRate  float64 `json:"agreement_rate"`
}

type Report struct {
	Models      []ModelFairness      `json:"models"`
	Pairwise    []PairwiseComparison `json:"pairwise,omitempty"`
	Fairest     string               `json:"fairest"`
	LeastFair   string               `json:"least_fair"`
	MeanScore   float64              `json:"mean_score"`
}

func Analyze(findings []model.Finding) Report {
	byModel := make(map[string][]model.Finding)
	for _, f := range findings {
		m := f.Model.Model
		if m == "" {
			m = "unknown"
		}
		byModel[m] = append(byModel[m], f)
	}

	var models []ModelFairness
	for name, mf := range byModel {
		mfr := scoreModel(name, mf)
		models = append(models, mfr)
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].FairnessScore > models[j].FairnessScore
	})

	var pairwise []PairwiseComparison
	for i := 0; i < len(models); i++ {
		for j := i + 1; j < len(models); j++ {
			pc := comparePair(byModel[models[i].Model], byModel[models[j].Model], models[i].Model, models[j].Model)
			pairwise = append(pairwise, pc)
		}
	}

	var fairest, leastFair string
	var total float64
	if len(models) > 0 {
		fairest = models[0].Model
		leastFair = models[len(models)-1].Model
		for _, m := range models {
			total += m.FairnessScore
		}
	}

	var mean float64
	if len(models) > 0 {
		mean = total / float64(len(models))
	}

	return Report{
		Models:    models,
		Pairwise:  pairwise,
		Fairest:   fairest,
		LeastFair: leastFair,
		MeanScore: math.Round(mean*10) / 10,
	}
}

func scoreModel(name string, findings []model.Finding) ModelFairness {
	classCounts := make(map[string]int)
	sevCounts := make(map[string]int)
	for _, f := range findings {
		classCounts[string(f.Class)]++
		sevCounts[string(f.Severity)]++
	}

	total := len(findings)
	classDist := buildClassDist(classCounts, total)
	sevDist := buildSevDist(sevCounts, total)

	classGini := giniCoefficient(classCounts)
	sevGini := giniCoefficient(sevCounts)

	var mostClass, leastClass string
	maxC, minC := 0, math.MaxInt
	for c, n := range classCounts {
		if n > maxC {
			maxC = n
			mostClass = c
		}
		if n < minC {
			minC = n
			leastClass = c
		}
	}

	concentration := 0.0
	if total > 0 && len(classCounts) > 0 {
		concentration = float64(maxC) / float64(total)
	}

	fairness := 100.0
	fairness -= classGini * 50
	fairness -= sevGini * 30
	fairness -= concentration * 20
	if fairness < 0 {
		fairness = 0
	}
	fairness = math.Round(fairness*10) / 10

	return ModelFairness{
		Model:              name,
		TotalFindings:      total,
		ClassDistributions: classDist,
		SeverityDist:       sevDist,
		ClassGini:          math.Round(classGini*1000) / 1000,
		SeverityGini:       math.Round(sevGini*1000) / 1000,
		FairnessScore:      fairness,
		Grade:              scoreToGrade(fairness),
		MostReportedClass:  mostClass,
		LeastReportedClass: leastClass,
		ClassConcentration: math.Round(concentration*1000) / 1000,
	}
}

func buildClassDist(counts map[string]int, total int) []ClassDistribution {
	var dist []ClassDistribution
	for class, count := range counts {
		pct := 0.0
		if total > 0 {
			pct = math.Round(float64(count)/float64(total)*1000) / 10
		}
		dist = append(dist, ClassDistribution{Class: class, Count: count, Percent: pct})
	}
	sort.Slice(dist, func(i, j int) bool { return dist[i].Count > dist[j].Count })
	return dist
}

func buildSevDist(counts map[string]int, total int) []SeverityDistribution {
	var dist []SeverityDistribution
	for sev, count := range counts {
		pct := 0.0
		if total > 0 {
			pct = math.Round(float64(count)/float64(total)*1000) / 10
		}
		dist = append(dist, SeverityDistribution{Severity: sev, Count: count, Percent: pct})
	}
	sort.Slice(dist, func(i, j int) bool { return dist[i].Count > dist[j].Count })
	return dist
}

func giniCoefficient(counts map[string]int) float64 {
	if len(counts) <= 1 {
		return 0
	}
	vals := make([]float64, 0, len(counts))
	for _, c := range counts {
		vals = append(vals, float64(c))
	}
	sort.Float64s(vals)

	n := float64(len(vals))
	var sumNumerator float64
	for i, v := range vals {
		sumNumerator += (2*float64(i+1) - n - 1) * v
	}
	var sumAll float64
	for _, v := range vals {
		sumAll += v
	}
	if sumAll == 0 {
		return 0
	}
	return sumNumerator / (n * sumAll)
}

func comparePair(aFindings, bFindings []model.Finding, nameA, nameB string) PairwiseComparison {
	aClasses := make(map[string]int)
	bClasses := make(map[string]int)
	for _, f := range aFindings {
		aClasses[string(f.Class)]++
	}
	for _, f := range bFindings {
		bClasses[string(f.Class)]++
	}

	allClasses := make(map[string]bool)
	for c := range aClasses {
		allClasses[c] = true
	}
	for c := range bClasses {
		allClasses[c] = true
	}

	totalA := float64(len(aFindings))
	totalB := float64(len(bFindings))

	var divergence float64
	var agree int
	for c := range allClasses {
		pA, pB := 0.0, 0.0
		if totalA > 0 {
			pA = float64(aClasses[c]) / totalA
		}
		if totalB > 0 {
			pB = float64(bClasses[c]) / totalB
		}
		d := pA - pB
		divergence += d * d
		if aClasses[c] > 0 && bClasses[c] > 0 {
			agree++
		}
	}

	agreementRate := 0.0
	if len(allClasses) > 0 {
		agreementRate = float64(agree) / float64(len(allClasses))
	}

	return PairwiseComparison{
		ModelA:        nameA,
		ModelB:        nameB,
		Divergence:    math.Round(math.Sqrt(divergence)*1000) / 1000,
		AgreementRate: math.Round(agreementRate*1000) / 1000,
	}
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

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Model Fairness Evaluation\n")
	b.WriteString(strings.Repeat("=", 55) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No fairness data to analyze.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated: %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:       %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Fairest:          %s\n", r.Fairest)
	fmt.Fprintf(&b, "  Least fair:       %s\n\n", r.LeastFair)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s [%s] (%.1f/100)\n", m.Model, m.Grade, m.FairnessScore)
		fmt.Fprintf(&b, "    Total findings:      %d\n", m.TotalFindings)
		fmt.Fprintf(&b, "    Class Gini:          %.3f\n", m.ClassGini)
		fmt.Fprintf(&b, "    Severity Gini:       %.3f\n", m.SeverityGini)
		fmt.Fprintf(&b, "    Class concentration: %.3f\n", m.ClassConcentration)
		fmt.Fprintf(&b, "    Most reported:       %s\n", m.MostReportedClass)
		fmt.Fprintf(&b, "    Least reported:      %s\n", m.LeastReportedClass)
		b.WriteString("    Class distribution:\n")
		for _, cd := range m.ClassDistributions {
			fmt.Fprintf(&b, "      %s: %d (%.1f%%)\n", cd.Class, cd.Count, cd.Percent)
		}
		b.WriteString("    Severity distribution:\n")
		for _, sd := range m.SeverityDist {
			fmt.Fprintf(&b, "      %s: %d (%.1f%%)\n", sd.Severity, sd.Count, sd.Percent)
		}
		b.WriteString("\n")
	}

	if len(r.Pairwise) > 0 {
		b.WriteString("  Pairwise Comparisons\n")
		b.WriteString("  " + strings.Repeat("-", 40) + "\n")
		for _, p := range r.Pairwise {
			fmt.Fprintf(&b, "    %s vs %s: divergence=%.3f agreement=%.3f\n",
				p.ModelA, p.ModelB, p.Divergence, p.AgreementRate)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
