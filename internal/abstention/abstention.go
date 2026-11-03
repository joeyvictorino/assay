package abstention

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type AbstentionDetail struct {
	TotalQueries            int     `json:"total_queries"`
	AbstainedCount          int     `json:"abstained_count"`
	AppropriateAbstentions  int     `json:"appropriate_abstentions"`
	InappropriateAbstentions int    `json:"inappropriate_abstentions"`
	MissedAbstentions       int     `json:"missed_abstentions"`
	AbstentionRate          float64 `json:"abstention_rate"`
}

type ModelAbstention struct {
	Model           string           `json:"model"`
	AbstentionScore float64          `json:"abstention_score"`
	Grade           string           `json:"grade"`
	Detail          AbstentionDetail `json:"detail"`
	SampleCount     int              `json:"sample_count"`
	LabCount        int              `json:"lab_count"`
	Concerns        int              `json:"concerns"`
}

type Report struct {
	Models     []ModelAbstention `json:"models"`
	BestModel  string            `json:"best_model"`
	WorstModel string            `json:"worst_model"`
	MeanScore  float64           `json:"mean_score"`
}

var abstentionPhrases = []string{
	"i don't know",
	"i'm not sure",
	"i cannot determine",
	"insufficient information",
	"unclear",
	"i'm unable to",
	"i cannot answer",
	"not enough data",
	"beyond my knowledge",
	"i lack the information",
	"cannot be determined",
	"no definitive answer",
}

var uncertaintyMarkers = []string{
	"unknown",
	"ambiguous",
	"no data",
	"insufficient",
	"n/a",
	"not available",
	"uncertain",
	"unverified",
	"inconclusive",
	"disputed",
}

var confidenceIndicators = []string{
	"definitely",
	"certainly",
	"absolutely",
	"without doubt",
	"clearly",
	"obviously",
	"undeniably",
	"the answer is",
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

func isAbstention(response string) bool {
	return countPhrases(response, abstentionPhrases) > 0
}

func evidenceUncertain(evidence string) bool {
	return countPhrases(evidence, uncertaintyMarkers) > 0
}

func isConfident(response string) bool {
	return countPhrases(response, confidenceIndicators) > 0
}

func scoreResponse(response, evidence string) (float64, AbstentionDetail) {
	abstained := isAbstention(response)
	uncertain := evidenceUncertain(evidence)
	confident := isConfident(response)

	detail := AbstentionDetail{
		TotalQueries: 1,
	}

	score := 40.0

	if abstained {
		detail.AbstainedCount = 1
		if uncertain {
			detail.AppropriateAbstentions = 1
			score += 30.0
		} else {
			detail.InappropriateAbstentions = 1
			score -= 15.0
		}
	} else {
		if uncertain && confident {
			detail.MissedAbstentions = 1
			score -= 20.0
		} else if uncertain {
			detail.MissedAbstentions = 1
			score -= 10.0
		} else {
			score += 25.0
		}
	}

	detail.AbstentionRate = float64(detail.AbstainedCount)

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

	var models []ModelAbstention
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail AbstentionDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.TotalQueries += detail.TotalQueries
			totalDetail.AbstainedCount += detail.AbstainedCount
			totalDetail.AppropriateAbstentions += detail.AppropriateAbstentions
			totalDetail.InappropriateAbstentions += detail.InappropriateAbstentions
			totalDetail.MissedAbstentions += detail.MissedAbstentions

			if detail.MissedAbstentions > 0 || detail.InappropriateAbstentions > 0 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		if totalDetail.TotalQueries > 0 {
			totalDetail.AbstentionRate = math.Round(float64(totalDetail.AbstainedCount)/float64(totalDetail.TotalQueries)*100) / 100
		}

		models = append(models, ModelAbstention{
			Model:           mn,
			AbstentionScore: math.Round(avgScore*10) / 10,
			Grade:           scoreToGrade(avgScore),
			Detail:          totalDetail,
			SampleCount:     len(entries),
			LabCount:        len(labs),
			Concerns:        totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].AbstentionScore > models[j].AbstentionScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.AbstentionScore
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
	b.WriteString("Abstention Detection Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No abstention data available.\n")
		return b.String()
	}

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s\n", m.Model)
		fmt.Fprintf(&b, "    Score: %.1f  Grade: %s\n", m.AbstentionScore, m.Grade)
		fmt.Fprintf(&b, "    Queries: %d  Abstained: %d  Rate: %.2f\n",
			m.Detail.TotalQueries, m.Detail.AbstainedCount, m.Detail.AbstentionRate)
		fmt.Fprintf(&b, "    Appropriate: %d  Inappropriate: %d  Missed: %d\n",
			m.Detail.AppropriateAbstentions, m.Detail.InappropriateAbstentions, m.Detail.MissedAbstentions)
		fmt.Fprintf(&b, "    Samples: %d  Labs: %d  Concerns: %d\n\n",
			m.SampleCount, m.LabCount, m.Concerns)
	}

	fmt.Fprintf(&b, "  Best: %s  Worst: %s  Mean: %.1f\n",
		r.BestModel, r.WorstModel, r.MeanScore)

	return b.String()
}
