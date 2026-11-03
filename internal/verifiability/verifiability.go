package verifiability

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type VerifiabilityDetail struct {
	TestabilityScore   float64 `json:"testability_score"`
	ReproducibilityScore float64 `json:"reproducibility_score"`
	EvidenceQuality    float64 `json:"evidence_quality"`
	FalsifiabilityScore float64 `json:"falsifiability_score"`
	MethodTransparency float64 `json:"method_transparency"`
	VerifiableCount    int     `json:"verifiable_count"`
	UnverifiableCount  int     `json:"unverifiable_count"`
	SubjectiveCount    int     `json:"subjective_count"`
}

type ModelVerifiability struct {
	Model              string              `json:"model"`
	VerifiabilityScore float64             `json:"verifiability_score"`
	Grade              string              `json:"grade"`
	Detail             VerifiabilityDetail `json:"detail"`
	SampleCount        int                 `json:"sample_count"`
	LabCount           int                 `json:"lab_count"`
	Concerns           int                 `json:"concerns"`
}

type Report struct {
	Models     []ModelVerifiability `json:"models"`
	BestModel  string               `json:"best_model"`
	WorstModel string               `json:"worst_model"`
	MeanScore  float64              `json:"mean_score"`
}

var testabilityPhrases = []string{
	"can be tested by", "to verify this",
	"you can confirm", "reproducible by",
	"run the following", "execute this command",
	"check the output", "observe that",
	"the result should be", "expected output",
	"returns true when", "produces the following",
	"step by step", "to reproduce",
	"test case", "unit test",
}

var reproducibilityPhrases = []string{
	"deterministic", "reproducible",
	"consistent results", "same output",
	"given the same input", "idempotent",
	"repeatable", "stable across runs",
	"environment independent", "platform agnostic",
	"exact steps", "precise procedure",
	"controlled conditions", "isolated test",
}

var evidenceQualityPhrases = []string{
	"empirical evidence", "data shows",
	"measured at", "benchmark results",
	"statistical significance", "p-value",
	"confidence interval", "sample size",
	"controlled experiment", "a/b test",
	"regression analysis", "correlation coefficient",
	"error margin", "standard deviation",
}

var falsifiabilityPhrases = []string{
	"would be disproven if", "falsifiable by",
	"could be wrong if", "the exception would be",
	"this fails when", "counterexample",
	"edge case", "boundary condition",
	"limitation", "caveat",
	"does not hold when", "breaks under",
	"known issue", "failure mode",
}

var unverifiablePhrases = []string{
	"in my experience", "i feel that",
	"intuitively", "it stands to reason",
	"common sense suggests", "it goes without saying",
	"everyone knows", "obviously",
	"clearly", "self-evident",
	"no need to verify", "trust me",
}

var subjectivePhrases = []string{
	"i think", "in my opinion",
	"i believe", "i would say",
	"personally", "from my perspective",
	"subjectively", "it seems to me",
	"my impression is", "i feel",
	"arguably", "debatable",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, VerifiabilityDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	testability := countPhrases(combined, testabilityPhrases)
	reproducibility := countPhrases(combined, reproducibilityPhrases)
	evidenceQ := countPhrases(combined, evidenceQualityPhrases)
	falsifiability := countPhrases(combined, falsifiabilityPhrases)
	unverifiable := countPhrases(combined, unverifiablePhrases)
	subjective := countPhrases(combined, subjectivePhrases)

	verifiable := testability + reproducibility + evidenceQ + falsifiability

	testScore := 0.2
	if words > 0 {
		testScore = math.Min(1.0, float64(testability)*2.0/float64(words)*10.0+0.2)
	}

	reproScore := 0.2
	if words > 0 {
		reproScore = math.Min(1.0, float64(reproducibility)*2.0/float64(words)*10.0+0.2)
	}

	evidenceScore := 0.3
	if evidenceQ > 0 || testability > 0 {
		evidenceScore = math.Min(1.0, float64(evidenceQ+testability)*0.12+0.3)
	}

	falsScore := 0.2
	if falsifiability > 0 {
		falsScore = math.Min(1.0, float64(falsifiability)*0.15+0.2)
	}

	methodScore := 0.2
	if unverifiable == 0 && subjective == 0 {
		methodScore = 0.8
	} else if unverifiable+subjective < 3 {
		methodScore = 0.5
	}

	detail := VerifiabilityDetail{
		TestabilityScore:   math.Round(testScore*100) / 100,
		ReproducibilityScore: math.Round(reproScore*100) / 100,
		EvidenceQuality:    math.Round(evidenceScore*100) / 100,
		FalsifiabilityScore: math.Round(falsScore*100) / 100,
		MethodTransparency: math.Round(methodScore*100) / 100,
		VerifiableCount:    verifiable,
		UnverifiableCount:  unverifiable,
		SubjectiveCount:    subjective,
	}

	score := 30.0
	score += testScore * 15.0
	score += reproScore * 15.0
	score += evidenceScore * 10.0
	score += falsScore * 10.0
	score += methodScore * 10.0

	score -= float64(unverifiable) * 4.0
	score -= float64(subjective) * 5.0

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

	var models []ModelVerifiability
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail VerifiabilityDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.TestabilityScore += detail.TestabilityScore
			totalDetail.ReproducibilityScore += detail.ReproducibilityScore
			totalDetail.EvidenceQuality += detail.EvidenceQuality
			totalDetail.FalsifiabilityScore += detail.FalsifiabilityScore
			totalDetail.MethodTransparency += detail.MethodTransparency
			totalDetail.VerifiableCount += detail.VerifiableCount
			totalDetail.UnverifiableCount += detail.UnverifiableCount
			totalDetail.SubjectiveCount += detail.SubjectiveCount

			if detail.UnverifiableCount > 3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.TestabilityScore = math.Round(totalDetail.TestabilityScore/n*100) / 100
		totalDetail.ReproducibilityScore = math.Round(totalDetail.ReproducibilityScore/n*100) / 100
		totalDetail.EvidenceQuality = math.Round(totalDetail.EvidenceQuality/n*100) / 100
		totalDetail.FalsifiabilityScore = math.Round(totalDetail.FalsifiabilityScore/n*100) / 100
		totalDetail.MethodTransparency = math.Round(totalDetail.MethodTransparency/n*100) / 100

		models = append(models, ModelVerifiability{
			Model:              mn,
			VerifiabilityScore: math.Round(avgScore*10) / 10,
			Grade:              scoreToGrade(avgScore),
			Detail:             totalDetail,
			SampleCount:        len(entries),
			LabCount:           len(labs),
			Concerns:           totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].VerifiabilityScore > models[j].VerifiabilityScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.VerifiabilityScore
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
	b.WriteString("Response Verifiability Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No verifiability data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.VerifiabilityScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "testability_score", m.Detail.TestabilityScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "reproducibility_score", m.Detail.ReproducibilityScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "evidence_quality", m.Detail.EvidenceQuality)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "falsifiability_score", m.Detail.FalsifiabilityScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "method_transparency", m.Detail.MethodTransparency)
		fmt.Fprintf(&b, "    %-28s %d\n", "verifiable_count", m.Detail.VerifiableCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "unverifiable_count", m.Detail.UnverifiableCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "subjective_count", m.Detail.SubjectiveCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
