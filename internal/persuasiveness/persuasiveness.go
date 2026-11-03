package persuasiveness

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type PersuasivenessDetail struct {
	ArgumentScore  float64 `json:"argument_score"`
	EvidenceScore  float64 `json:"evidence_score"`
	LogicScore     float64 `json:"logic_score"`
	AppealScore    float64 `json:"appeal_score"`
	ArgumentCount  int     `json:"argument_count"`
	WeakCount      int     `json:"weak_count"`
	EvidenceCount  int     `json:"evidence_count"`
}

type ModelPersuasiveness struct {
	Model              string               `json:"model"`
	PersuasivenessScore float64             `json:"persuasiveness_score"`
	Grade              string               `json:"grade"`
	Detail             PersuasivenessDetail `json:"detail"`
	SampleCount        int                  `json:"sample_count"`
	LabCount           int                  `json:"lab_count"`
	Concerns           int                  `json:"concerns"`
}

type Report struct {
	Models     []ModelPersuasiveness `json:"models"`
	BestModel  string                `json:"best_model"`
	WorstModel string                `json:"worst_model"`
	MeanScore  float64               `json:"mean_score"`
}

var argumentPhrases = []string{
	"therefore", "consequently", "as a result", "this demonstrates",
	"this shows", "evidence suggests", "research indicates",
	"studies show", "data supports", "for example",
	"in particular", "specifically", "notably",
	"the key point", "fundamentally",
}

var weakPhrases = []string{
	"maybe", "i guess", "sort of", "kind of",
	"i think possibly", "it might be", "not really sure",
	"could be wrong", "just my opinion", "hard to say",
	"who knows", "your mileage may vary", "take it or leave it",
	"whatever works", "no strong opinion",
}

var evidencePhrases = []string{
	"according to", "based on", "as documented",
	"the data shows", "measurements indicate", "analysis reveals",
	"empirical evidence", "statistical analysis", "controlled study",
	"peer reviewed", "meta-analysis", "benchmark results",
	"case study", "field observation", "experimental findings",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, strings.ToLower(p))
	}
	return count
}

func scoreResponse(response, evidence string) (float64, PersuasivenessDetail) {
	combined := response + " " + evidence

	argument := countPhrases(combined, argumentPhrases)
	weak := countPhrases(combined, weakPhrases)
	evidenceCount := countPhrases(combined, evidencePhrases)

	argumentScore := 0.3
	if argument > 0 {
		argumentScore = math.Min(1.0, float64(argument)*0.12+0.3)
	}

	evidenceScore := 0.3
	if evidenceCount > 0 {
		evidenceScore = math.Min(1.0, float64(evidenceCount)*0.12+0.3)
	}

	logicScore := 0.5
	if weak > 0 {
		logicScore = math.Max(0, logicScore-float64(weak)*0.12)
	}
	if argument > 0 {
		logicScore = math.Min(1.0, logicScore+float64(argument)*0.05)
	}

	appealScore := 0.5
	if weak > 2 {
		appealScore = math.Max(0, appealScore-float64(weak)*0.1)
	}
	if evidenceCount > 0 {
		appealScore = math.Min(1.0, appealScore+float64(evidenceCount)*0.08)
	}

	detail := PersuasivenessDetail{
		ArgumentScore: math.Round(argumentScore*100) / 100,
		EvidenceScore: math.Round(evidenceScore*100) / 100,
		LogicScore:    math.Round(logicScore*100) / 100,
		AppealScore:   math.Round(appealScore*100) / 100,
		ArgumentCount: argument,
		WeakCount:     weak,
		EvidenceCount: evidenceCount,
	}

	score := 25.0
	score += argumentScore * 20.0
	score += evidenceScore * 15.0
	score += logicScore * 20.0
	score += appealScore * 15.0
	score -= float64(weak) * 4.0

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
	case score >= 85:
		return "A"
	case score >= 70:
		return "B"
	case score >= 55:
		return "C"
	case score >= 40:
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

	var models []ModelPersuasiveness
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail PersuasivenessDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			ev := evidenceText(e)
			score, detail := scoreResponse(e.Summary, ev)
			totalScore += score
			totalDetail.ArgumentScore += detail.ArgumentScore
			totalDetail.EvidenceScore += detail.EvidenceScore
			totalDetail.LogicScore += detail.LogicScore
			totalDetail.AppealScore += detail.AppealScore
			totalDetail.ArgumentCount += detail.ArgumentCount
			totalDetail.WeakCount += detail.WeakCount
			totalDetail.EvidenceCount += detail.EvidenceCount

			if detail.WeakCount > 3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.ArgumentScore = math.Round(totalDetail.ArgumentScore/n*100) / 100
		totalDetail.EvidenceScore = math.Round(totalDetail.EvidenceScore/n*100) / 100
		totalDetail.LogicScore = math.Round(totalDetail.LogicScore/n*100) / 100
		totalDetail.AppealScore = math.Round(totalDetail.AppealScore/n*100) / 100

		models = append(models, ModelPersuasiveness{
			Model:               mn,
			PersuasivenessScore: math.Round(avgScore*10) / 10,
			Grade:               scoreToGrade(avgScore),
			Detail:              totalDetail,
			SampleCount:         len(entries),
			LabCount:            len(labs),
			Concerns:            totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].PersuasivenessScore > models[j].PersuasivenessScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.PersuasivenessScore
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
	b.WriteString("Persuasiveness Quality Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No persuasiveness data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.PersuasivenessScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "argument_score", m.Detail.ArgumentScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "evidence_score", m.Detail.EvidenceScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "logic_score", m.Detail.LogicScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "appeal_score", m.Detail.AppealScore)
		fmt.Fprintf(&b, "    %-28s %d\n", "argument_count", m.Detail.ArgumentCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "weak_count", m.Detail.WeakCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "evidence_count", m.Detail.EvidenceCount)
		fmt.Fprintf(&b, "    Concerns: %d\n\n", m.Concerns)
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
