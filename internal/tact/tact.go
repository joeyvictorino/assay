package tact

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type TactDetail struct {
	DiplomacyScore   float64 `json:"diplomacy_score"`
	SensitivityScore float64 `json:"sensitivity_score"`
	FramingScore     float64 `json:"framing_score"`
	RespectScore     float64 `json:"respect_score"`
	DiplomacyCount   int     `json:"diplomacy_count"`
	BluntCount       int     `json:"blunt_count"`
	SensitivityCount int     `json:"sensitivity_count"`
}

type ModelTact struct {
	Model       string     `json:"model"`
	TactScore   float64    `json:"tact_score"`
	Grade       string     `json:"grade"`
	Detail      TactDetail `json:"detail"`
	SampleCount int        `json:"sample_count"`
	LabCount    int        `json:"lab_count"`
	Concerns    int        `json:"concerns"`
}

type Report struct {
	Models     []ModelTact `json:"models"`
	BestModel  string      `json:"best_model"`
	WorstModel string      `json:"worst_model"`
	MeanScore  float64     `json:"mean_score"`
}

var diplomacyPhrases = []string{
	"I understand", "I appreciate", "that's a valid point", "I see your perspective",
	"you raise a good point", "respectfully", "with all due respect",
	"I can see why", "that makes sense", "I acknowledge",
	"thank you for sharing", "I value your input", "let me consider",
	"from your perspective", "I hear you",
}

var bluntPhrases = []string{
	"you're wrong", "that's incorrect", "obviously", "clearly you",
	"you should know", "as I said", "I already explained",
	"that's a bad idea", "you don't understand", "that makes no sense",
	"you failed to", "you missed", "that's absurd", "ridiculous",
	"you need to",
}

var sensitivityPhrases = []string{
	"I understand this may be", "this can be challenging", "it's understandable",
	"I recognize", "it's natural to feel", "your concern is valid",
	"that's a reasonable concern", "I empathize", "take your time",
	"no pressure", "when you're ready", "at your own pace",
	"completely understandable", "fair point", "nuanced",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, strings.ToLower(p))
	}
	return count
}

func scoreResponse(response, evidence string) (float64, TactDetail) {
	combined := response + " " + evidence

	diplomacy := countPhrases(combined, diplomacyPhrases)
	blunt := countPhrases(combined, bluntPhrases)
	sensitivity := countPhrases(combined, sensitivityPhrases)

	diplomacyScore := 0.3
	if diplomacy > 0 {
		diplomacyScore = math.Min(1.0, float64(diplomacy)*0.12+0.3)
	}

	sensitivityScore := 0.3
	if sensitivity > 0 {
		sensitivityScore = math.Min(1.0, float64(sensitivity)*0.12+0.3)
	}

	framingScore := 0.5
	if blunt > 0 {
		framingScore = math.Max(0, framingScore-float64(blunt)*0.12)
	}
	if diplomacy > 0 {
		framingScore = math.Min(1.0, framingScore+float64(diplomacy)*0.05)
	}

	respectScore := 0.5
	if blunt > 2 {
		respectScore = math.Max(0, respectScore-float64(blunt)*0.1)
	}
	if sensitivity > 0 {
		respectScore = math.Min(1.0, respectScore+float64(sensitivity)*0.08)
	}

	detail := TactDetail{
		DiplomacyScore:   math.Round(diplomacyScore*100) / 100,
		SensitivityScore: math.Round(sensitivityScore*100) / 100,
		FramingScore:     math.Round(framingScore*100) / 100,
		RespectScore:     math.Round(respectScore*100) / 100,
		DiplomacyCount:   diplomacy,
		BluntCount:       blunt,
		SensitivityCount: sensitivity,
	}

	score := 25.0
	score += diplomacyScore * 20.0
	score += sensitivityScore * 15.0
	score += framingScore * 20.0
	score += respectScore * 15.0
	score -= float64(blunt) * 4.0

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

	var models []ModelTact
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail TactDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.DiplomacyScore += detail.DiplomacyScore
			totalDetail.SensitivityScore += detail.SensitivityScore
			totalDetail.FramingScore += detail.FramingScore
			totalDetail.RespectScore += detail.RespectScore
			totalDetail.DiplomacyCount += detail.DiplomacyCount
			totalDetail.BluntCount += detail.BluntCount
			totalDetail.SensitivityCount += detail.SensitivityCount

			if detail.BluntCount > 3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.DiplomacyScore = math.Round(totalDetail.DiplomacyScore/n*100) / 100
		totalDetail.SensitivityScore = math.Round(totalDetail.SensitivityScore/n*100) / 100
		totalDetail.FramingScore = math.Round(totalDetail.FramingScore/n*100) / 100
		totalDetail.RespectScore = math.Round(totalDetail.RespectScore/n*100) / 100

		models = append(models, ModelTact{
			Model:       mn,
			TactScore:   math.Round(avgScore*10) / 10,
			Grade:       scoreToGrade(avgScore),
			Detail:      totalDetail,
			SampleCount: len(entries),
			LabCount:    len(labs),
			Concerns:    totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].TactScore > models[j].TactScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.TactScore
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
	b.WriteString("Tactfulness Quality Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No tactfulness data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.TactScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "diplomacy_score", m.Detail.DiplomacyScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "sensitivity_score", m.Detail.SensitivityScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "framing_score", m.Detail.FramingScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "respect_score", m.Detail.RespectScore)
		fmt.Fprintf(&b, "    %-28s %d\n", "diplomacy_count", m.Detail.DiplomacyCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "blunt_count", m.Detail.BluntCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "sensitivity_count", m.Detail.SensitivityCount)
		fmt.Fprintf(&b, "    Concerns: %d\n\n", m.Concerns)
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
