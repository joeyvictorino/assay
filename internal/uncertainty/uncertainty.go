package uncertainty

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type UncertaintyDetail struct {
	HedgingLevel     float64 `json:"hedging_level"`
	ConfidenceClarity float64 `json:"confidence_clarity"`
	CaveatBalance    float64 `json:"caveat_balance"`
	CommitmentLevel  float64 `json:"commitment_level"`
	PrecisionLevel   float64 `json:"precision_level"`
	HedgeCount       int     `json:"hedge_count"`
	ConfidentCount   int     `json:"confident_count"`
	CaveatCount      int     `json:"caveat_count"`
}

type ModelUncertainty struct {
	Model            string            `json:"model"`
	UncertaintyScore float64           `json:"uncertainty_score"`
	Grade            string            `json:"grade"`
	Detail           UncertaintyDetail `json:"detail"`
	SampleCount      int               `json:"sample_count"`
	LabCount         int               `json:"lab_count"`
	Concerns         int               `json:"concerns"`
}

type Report struct {
	Models     []ModelUncertainty `json:"models"`
	BestModel  string             `json:"best_model"`
	WorstModel string             `json:"worst_model"`
	MeanScore  float64            `json:"mean_score"`
}

var hedgingPhrases = []string{
	"might be", "could be", "possibly", "perhaps",
	"it seems", "it appears", "arguably", "presumably",
	"i think", "i believe", "in my opinion",
	"it's possible", "there's a chance",
	"not entirely sure", "hard to say",
}

var confidentPhrases = []string{
	"definitely", "certainly", "absolutely",
	"without a doubt", "clearly", "obviously",
	"undeniably", "unquestionably", "precisely",
	"specifically", "exactly", "the answer is",
	"this is correct", "the fact is",
}

var caveatPhrases = []string{
	"however", "on the other hand", "that said",
	"it depends", "there are exceptions",
	"in some cases", "under certain conditions",
	"with some caveats", "generally speaking",
	"in most cases", "typically",
	"but keep in mind", "worth noting",
}

var vaguePhrases = []string{
	"sort of", "kind of", "more or less",
	"to some extent", "in a way", "somewhat",
	"a bit", "a little", "fairly",
	"rather", "quite", "pretty much",
}

var precisionPhrases = []string{
	"according to", "based on", "research shows",
	"studies indicate", "data suggests", "evidence points to",
	"measured at", "calculated as", "documented in",
	"as of", "between", "approximately",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, UncertaintyDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	hedges := countPhrases(combined, hedgingPhrases)
	confident := countPhrases(combined, confidentPhrases)
	caveats := countPhrases(combined, caveatPhrases)
	vague := countPhrases(combined, vaguePhrases)
	precise := countPhrases(combined, precisionPhrases)

	hedgingScore := 0.7
	if words > 0 {
		density := float64(hedges+vague) / float64(words) * 100
		hedgingScore = math.Max(0, math.Min(1.0, 0.7-density*0.1+0.1))
	}

	confidenceScore := 0.5
	if words > 0 {
		confidenceScore = math.Min(1.0, float64(confident)*2.0/float64(words)*10.0+0.5)
	}

	caveatScore := 0.6
	if caveats > 0 && caveats <= 3 {
		caveatScore = math.Min(1.0, 0.6+float64(caveats)*0.12)
	} else if caveats > 3 {
		caveatScore = math.Max(0.3, 0.9-float64(caveats-3)*0.1)
	}

	commitScore := 0.6
	if hedges+vague > 0 && confident > 0 {
		ratio := float64(confident) / float64(hedges+vague+confident)
		commitScore = math.Min(1.0, ratio+0.3)
	} else if confident > 0 {
		commitScore = 0.9
	}

	precisionScore := 0.5
	if precise > 0 {
		precisionScore = math.Min(1.0, float64(precise)*0.2+0.5)
	}

	detail := UncertaintyDetail{
		HedgingLevel:      math.Round(hedgingScore*100) / 100,
		ConfidenceClarity: math.Round(confidenceScore*100) / 100,
		CaveatBalance:     math.Round(caveatScore*100) / 100,
		CommitmentLevel:   math.Round(commitScore*100) / 100,
		PrecisionLevel:    math.Round(precisionScore*100) / 100,
		HedgeCount:        hedges + vague,
		ConfidentCount:    confident,
		CaveatCount:       caveats,
	}

	score := 40.0
	score += hedgingScore * 12.0
	score += confidenceScore * 12.0
	score += caveatScore * 10.0
	score += commitScore * 10.0
	score += precisionScore * 10.0

	score -= float64(vague) * 3.0

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

	var models []ModelUncertainty
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail UncertaintyDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.HedgingLevel += detail.HedgingLevel
			totalDetail.ConfidenceClarity += detail.ConfidenceClarity
			totalDetail.CaveatBalance += detail.CaveatBalance
			totalDetail.CommitmentLevel += detail.CommitmentLevel
			totalDetail.PrecisionLevel += detail.PrecisionLevel
			totalDetail.HedgeCount += detail.HedgeCount
			totalDetail.ConfidentCount += detail.ConfidentCount
			totalDetail.CaveatCount += detail.CaveatCount

			if detail.HedgeCount > 3 && detail.ConfidentCount == 0 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.HedgingLevel = math.Round(totalDetail.HedgingLevel/n*100) / 100
		totalDetail.ConfidenceClarity = math.Round(totalDetail.ConfidenceClarity/n*100) / 100
		totalDetail.CaveatBalance = math.Round(totalDetail.CaveatBalance/n*100) / 100
		totalDetail.CommitmentLevel = math.Round(totalDetail.CommitmentLevel/n*100) / 100
		totalDetail.PrecisionLevel = math.Round(totalDetail.PrecisionLevel/n*100) / 100

		models = append(models, ModelUncertainty{
			Model:            mn,
			UncertaintyScore: math.Round(avgScore*10) / 10,
			Grade:            scoreToGrade(avgScore),
			Detail:           totalDetail,
			SampleCount:      len(entries),
			LabCount:         len(labs),
			Concerns:         totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].UncertaintyScore > models[j].UncertaintyScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.UncertaintyScore
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
	b.WriteString("Uncertainty Calibration Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No uncertainty data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.UncertaintyScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "hedging_level", m.Detail.HedgingLevel)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "confidence_clarity", m.Detail.ConfidenceClarity)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "caveat_balance", m.Detail.CaveatBalance)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "commitment_level", m.Detail.CommitmentLevel)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "precision_level", m.Detail.PrecisionLevel)
		fmt.Fprintf(&b, "    %-28s %d\n", "hedge_count", m.Detail.HedgeCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "confident_count", m.Detail.ConfidentCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "caveat_count", m.Detail.CaveatCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
