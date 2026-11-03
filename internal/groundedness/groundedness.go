package groundedness

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type GroundednessDetail struct {
	ClaimCount     int     `json:"claim_count"`
	GroundedCount  int     `json:"grounded_count"`
	HallucinatedCount int  `json:"hallucinated_count"`
	OverlapRatio   float64 `json:"overlap_ratio"`
	EvidenceCoverage float64 `json:"evidence_coverage"`
}

type ModelGroundedness struct {
	Model            string             `json:"model"`
	GroundednessScore float64           `json:"groundedness_score"`
	Grade            string             `json:"grade"`
	Detail           GroundednessDetail `json:"detail"`
	SampleCount      int                `json:"sample_count"`
	LabCount         int                `json:"lab_count"`
	Concerns         int                `json:"concerns"`
}

type Report struct {
	Models     []ModelGroundedness `json:"models"`
	BestModel  string              `json:"best_model"`
	WorstModel string              `json:"worst_model"`
	MeanScore  float64             `json:"mean_score"`
}

var hedgePhrases = []string{
	"i think", "i believe", "probably", "it seems", "perhaps",
	"likely", "possibly", "might be", "could be", "it appears",
	"in my opinion", "my understanding is", "as far as i know",
}

var speculativePhrases = []string{
	"it is possible that", "one could argue", "there may be",
	"this suggests", "it could mean", "this might indicate",
	"hypothetically", "theoretically", "in theory",
}

var fabricationIndicators = []string{
	"according to studies", "research shows", "experts say",
	"studies have shown", "it is well known", "as everyone knows",
	"statistics show", "data suggests", "surveys indicate",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
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

func wordOverlap(response, evidence string) float64 {
	respWords := strings.Fields(strings.ToLower(response))
	evidWords := strings.Fields(strings.ToLower(evidence))
	if len(respWords) == 0 || len(evidWords) == 0 {
		return 0
	}
	evidSet := map[string]bool{}
	for _, w := range evidWords {
		if len(w) > 3 {
			evidSet[w] = true
		}
	}
	matched := 0
	contentWords := 0
	for _, w := range respWords {
		if len(w) > 3 {
			contentWords++
			if evidSet[w] {
				matched++
			}
		}
	}
	if contentWords == 0 {
		return 0
	}
	return float64(matched) / float64(contentWords)
}

func scoreResponse(response, evidence string) (float64, GroundednessDetail) {
	words := strings.Fields(response)
	sentences := strings.Count(response, ".") + strings.Count(response, "!") + strings.Count(response, "?")
	if sentences == 0 {
		sentences = 1
	}

	hedges := countPhrases(response, hedgePhrases)
	speculative := countPhrases(response, speculativePhrases)
	fabrications := countPhrases(response, fabricationIndicators)

	overlap := wordOverlap(response, evidence)

	evidenceCoverage := 0.5
	if len(evidence) > 0 {
		evidenceCoverage = math.Min(1.0, float64(len(evidence))/float64(max(len(response), 1))*1.5)
	}

	claimCount := sentences
	hallucinatedCount := fabrications + speculative
	groundedCount := claimCount - hallucinatedCount
	if groundedCount < 0 {
		groundedCount = 0
	}

	detail := GroundednessDetail{
		ClaimCount:        claimCount,
		GroundedCount:     groundedCount,
		HallucinatedCount: hallucinatedCount,
		OverlapRatio:      math.Round(overlap*100) / 100,
		EvidenceCoverage:  math.Round(evidenceCoverage*100) / 100,
	}

	score := 30.0
	score += overlap * 25.0
	score += evidenceCoverage * 20.0
	score -= float64(fabrications) * 8.0
	score -= float64(speculative) * 4.0
	score -= float64(hedges) * 2.0

	if claimCount > 0 {
		groundedRatio := float64(groundedCount) / float64(claimCount)
		score += groundedRatio * 25.0
	}

	if len(words) > 0 {
		wasteRatio := float64(hedges+speculative+fabrications) / float64(len(words)) * 100
		score -= wasteRatio * 0.5
	}

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

	var models []ModelGroundedness
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail GroundednessDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.ClaimCount += detail.ClaimCount
			totalDetail.GroundedCount += detail.GroundedCount
			totalDetail.HallucinatedCount += detail.HallucinatedCount
			totalDetail.OverlapRatio += detail.OverlapRatio
			totalDetail.EvidenceCoverage += detail.EvidenceCoverage

			if detail.HallucinatedCount > 2 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.OverlapRatio = math.Round(totalDetail.OverlapRatio/n*100) / 100
		totalDetail.EvidenceCoverage = math.Round(totalDetail.EvidenceCoverage/n*100) / 100

		models = append(models, ModelGroundedness{
			Model:             mn,
			GroundednessScore: math.Round(avgScore*10) / 10,
			Grade:             scoreToGrade(avgScore),
			Detail:            totalDetail,
			SampleCount:       len(entries),
			LabCount:          len(labs),
			Concerns:          totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].GroundednessScore > models[j].GroundednessScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.GroundednessScore
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
	b.WriteString("Response Groundedness Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No groundedness data available.\n")
		return b.String()
	}

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s\n", m.Model)
		fmt.Fprintf(&b, "    Score: %.1f  Grade: %s\n", m.GroundednessScore, m.Grade)
		fmt.Fprintf(&b, "    Claims: %d  Grounded: %d  Hallucinated: %d\n",
			m.Detail.ClaimCount, m.Detail.GroundedCount, m.Detail.HallucinatedCount)
		fmt.Fprintf(&b, "    Overlap: %.2f  Evidence Coverage: %.2f\n",
			m.Detail.OverlapRatio, m.Detail.EvidenceCoverage)
		fmt.Fprintf(&b, "    Samples: %d  Labs: %d  Concerns: %d\n\n",
			m.SampleCount, m.LabCount, m.Concerns)
	}

	fmt.Fprintf(&b, "  Best: %s  Worst: %s  Mean: %.1f\n",
		r.BestModel, r.WorstModel, r.MeanScore)

	return b.String()
}
