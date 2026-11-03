package numeracy

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type NumeracyDetail struct {
	QuantitativeUse    float64 `json:"quantitative_use"`
	PrecisionScore     float64 `json:"precision_score"`
	UnitConsistency    float64 `json:"unit_consistency"`
	ComparisonQuality  float64 `json:"comparison_quality"`
	UncertaintyHandling float64 `json:"uncertainty_handling"`
	NumericCount       int     `json:"numeric_count"`
	VagueCount         int     `json:"vague_count"`
	InconsistentCount  int     `json:"inconsistent_count"`
}

type ModelNumeracy struct {
	Model         string         `json:"model"`
	NumeracyScore float64        `json:"numeracy_score"`
	Grade         string         `json:"grade"`
	Detail        NumeracyDetail `json:"detail"`
	SampleCount   int            `json:"sample_count"`
	LabCount      int            `json:"lab_count"`
	Concerns      int            `json:"concerns"`
}

type Report struct {
	Models     []ModelNumeracy `json:"models"`
	BestModel  string          `json:"best_model"`
	WorstModel string          `json:"worst_model"`
	MeanScore  float64         `json:"mean_score"`
}

var quantitativePhrases = []string{
	"percent", "percentage", "ratio",
	"rate of", "frequency of", "count of",
	"measured at", "scored", "averaged",
	"total of", "sum of", "median",
	"standard deviation", "variance",
	"confidence interval",
}

var precisionPhrases = []string{
	"exactly", "precisely", "approximately",
	"within the range", "between", "from x to y",
	"plus or minus", "margin of error",
	"to the nearest", "rounded to",
	"significant figures", "decimal places",
	"order of magnitude", "baseline of",
}

var unitPhrases = []string{
	"milliseconds", "seconds", "minutes", "hours",
	"bytes", "kilobytes", "megabytes", "gigabytes",
	"requests per second", "per minute",
	"per hour", "per day", "per month",
	"percentage points", "basis points",
}

var comparisonPhrases = []string{
	"compared to", "relative to", "versus",
	"higher than", "lower than", "greater than",
	"less than", "increased by", "decreased by",
	"times more", "times less", "fold increase",
	"order of magnitude higher", "significantly more",
	"double", "triple", "half",
}

var uncertaintyPhrases = []string{
	"confidence level", "p-value", "statistically significant",
	"within error bounds", "margin of error",
	"sample size", "not statistically significant",
	"the range suggests", "with high confidence",
	"within tolerance", "error rate of",
	"false positive rate", "true positive rate",
}

var vaguePhrases = []string{
	"a lot", "many", "few", "some",
	"a number of", "several", "various",
	"quite a bit", "substantial amount",
	"roughly", "around", "about",
	"in the ballpark", "give or take",
}

var inconsistentPhrases = []string{
	"but earlier i said", "actually the number is",
	"i need to correct", "my calculation was wrong",
	"let me recalculate", "i made an error",
	"the previous figure", "contradicts the",
	"does not add up", "mathematically impossible",
	"off by a factor", "miscounted",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, NumeracyDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	quantitative := countPhrases(combined, quantitativePhrases)
	precision := countPhrases(combined, precisionPhrases)
	units := countPhrases(combined, unitPhrases)
	comparison := countPhrases(combined, comparisonPhrases)
	uncertainty := countPhrases(combined, uncertaintyPhrases)
	vague := countPhrases(combined, vaguePhrases)
	inconsistent := countPhrases(combined, inconsistentPhrases)

	numericTotal := quantitative + precision + units + comparison + uncertainty

	quantScore := 0.2
	if words > 0 {
		quantScore = math.Min(1.0, float64(quantitative)*2.0/float64(words)*10.0+0.2)
	}

	precisionScore := 0.2
	if words > 0 {
		precisionScore = math.Min(1.0, float64(precision)*2.0/float64(words)*10.0+0.2)
	}

	unitScore := 0.3
	if units > 0 || comparison > 0 {
		unitScore = math.Min(1.0, float64(units+comparison)*0.15+0.3)
	}

	comparisonScore := 0.2
	if words > 0 {
		comparisonScore = math.Min(1.0, float64(comparison)*2.0/float64(words)*10.0+0.2)
	}

	uncertaintyScore := 0.2
	if vague == 0 && inconsistent == 0 {
		uncertaintyScore = 0.8
	} else if vague+inconsistent < 3 {
		uncertaintyScore = 0.5
	}

	detail := NumeracyDetail{
		QuantitativeUse:     math.Round(quantScore*100) / 100,
		PrecisionScore:      math.Round(precisionScore*100) / 100,
		UnitConsistency:     math.Round(unitScore*100) / 100,
		ComparisonQuality:   math.Round(comparisonScore*100) / 100,
		UncertaintyHandling: math.Round(uncertaintyScore*100) / 100,
		NumericCount:        numericTotal,
		VagueCount:          vague,
		InconsistentCount:   inconsistent,
	}

	score := 30.0
	score += quantScore * 15.0
	score += precisionScore * 15.0
	score += unitScore * 10.0
	score += comparisonScore * 10.0
	score += uncertaintyScore * 10.0

	score -= float64(vague) * 4.0
	score -= float64(inconsistent) * 5.0

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

	var models []ModelNumeracy
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail NumeracyDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.QuantitativeUse += detail.QuantitativeUse
			totalDetail.PrecisionScore += detail.PrecisionScore
			totalDetail.UnitConsistency += detail.UnitConsistency
			totalDetail.ComparisonQuality += detail.ComparisonQuality
			totalDetail.UncertaintyHandling += detail.UncertaintyHandling
			totalDetail.NumericCount += detail.NumericCount
			totalDetail.VagueCount += detail.VagueCount
			totalDetail.InconsistentCount += detail.InconsistentCount

			if detail.VagueCount > 3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.QuantitativeUse = math.Round(totalDetail.QuantitativeUse/n*100) / 100
		totalDetail.PrecisionScore = math.Round(totalDetail.PrecisionScore/n*100) / 100
		totalDetail.UnitConsistency = math.Round(totalDetail.UnitConsistency/n*100) / 100
		totalDetail.ComparisonQuality = math.Round(totalDetail.ComparisonQuality/n*100) / 100
		totalDetail.UncertaintyHandling = math.Round(totalDetail.UncertaintyHandling/n*100) / 100

		models = append(models, ModelNumeracy{
			Model:         mn,
			NumeracyScore: math.Round(avgScore*10) / 10,
			Grade:         scoreToGrade(avgScore),
			Detail:        totalDetail,
			SampleCount:   len(entries),
			LabCount:      len(labs),
			Concerns:      totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].NumeracyScore > models[j].NumeracyScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.NumeracyScore
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
	b.WriteString("Numerical Accuracy Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No numeracy data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.NumeracyScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "quantitative_use", m.Detail.QuantitativeUse)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "precision_score", m.Detail.PrecisionScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "unit_consistency", m.Detail.UnitConsistency)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "comparison_quality", m.Detail.ComparisonQuality)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "uncertainty_handling", m.Detail.UncertaintyHandling)
		fmt.Fprintf(&b, "    %-28s %d\n", "numeric_count", m.Detail.NumericCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "vague_count", m.Detail.VagueCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "inconsistent_count", m.Detail.InconsistentCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
