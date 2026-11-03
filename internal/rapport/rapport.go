package rapport

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type RapportDetail struct {
	WarmthScore      float64 `json:"warmth_score"`
	PersonalizeScore float64 `json:"personalize_score"`
	EngageScore      float64 `json:"engage_score"`
	MirrorScore      float64 `json:"mirror_score"`
	WarmthCount      int     `json:"warmth_count"`
	ColdCount        int     `json:"cold_count"`
	PersonalizeCount int     `json:"personalize_count"`
}

type ModelRapport struct {
	Model        string        `json:"model"`
	RapportScore float64       `json:"rapport_score"`
	Grade        string        `json:"grade"`
	Detail       RapportDetail `json:"detail"`
	SampleCount  int           `json:"sample_count"`
	LabCount     int           `json:"lab_count"`
	Concerns     int           `json:"concerns"`
}

type Report struct {
	Models     []ModelRapport `json:"models"`
	BestModel  string         `json:"best_model"`
	WorstModel string         `json:"worst_model"`
	MeanScore  float64        `json:"mean_score"`
}

var warmthPhrases = []string{
	"great question", "happy to help", "glad you asked", "wonderful",
	"excellent point", "I'm here to help", "that's exciting", "love that",
	"fantastic", "absolutely", "of course", "I'd be happy to",
	"what a great", "I'm glad", "that's a great",
}

var coldPhrases = []string{
	"as stated", "per my previous", "as mentioned", "I already told you",
	"refer to the documentation", "that's not my problem",
	"figure it out", "not my concern", "use the manual",
	"read the docs", "google it", "that's trivial",
	"not relevant", "beyond scope", "not applicable",
}

var personalizePhrases = []string{
	"in your case", "for your specific", "given your situation",
	"based on what you've described", "for your project",
	"tailored to your", "specific to your needs",
	"considering your requirements", "in your context",
	"for your use case", "your particular",
	"as you mentioned", "building on your idea",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, RapportDetail) {
	combined := response + " " + evidence

	warmth := countPhrases(combined, warmthPhrases)
	cold := countPhrases(combined, coldPhrases)
	personalize := countPhrases(combined, personalizePhrases)

	warmthScore := 0.3
	if warmth > 0 {
		warmthScore = math.Min(1.0, float64(warmth)*0.12+0.3)
	}

	personalizeScore := 0.2
	if personalize > 0 {
		personalizeScore = math.Min(1.0, float64(personalize)*0.15+0.2)
	}

	engageScore := 0.4
	if warmth > 0 {
		engageScore = math.Min(1.0, engageScore+float64(warmth)*0.06)
	}
	if cold > 0 {
		engageScore = math.Max(0, engageScore-float64(cold)*0.1)
	}

	mirrorScore := 0.3
	if personalize > 0 {
		mirrorScore = math.Min(1.0, mirrorScore+float64(personalize)*0.1)
	}

	detail := RapportDetail{
		WarmthScore:      math.Round(warmthScore*100) / 100,
		PersonalizeScore: math.Round(personalizeScore*100) / 100,
		EngageScore:      math.Round(engageScore*100) / 100,
		MirrorScore:      math.Round(mirrorScore*100) / 100,
		WarmthCount:      warmth,
		ColdCount:        cold,
		PersonalizeCount: personalize,
	}

	score := 25.0
	score += warmthScore * 20.0
	score += personalizeScore * 15.0
	score += engageScore * 20.0
	score += mirrorScore * 15.0
	score -= float64(cold) * 4.0

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

	var models []ModelRapport
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail RapportDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.WarmthScore += detail.WarmthScore
			totalDetail.PersonalizeScore += detail.PersonalizeScore
			totalDetail.EngageScore += detail.EngageScore
			totalDetail.MirrorScore += detail.MirrorScore
			totalDetail.WarmthCount += detail.WarmthCount
			totalDetail.ColdCount += detail.ColdCount
			totalDetail.PersonalizeCount += detail.PersonalizeCount

			if detail.ColdCount > 3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.WarmthScore = math.Round(totalDetail.WarmthScore/n*100) / 100
		totalDetail.PersonalizeScore = math.Round(totalDetail.PersonalizeScore/n*100) / 100
		totalDetail.EngageScore = math.Round(totalDetail.EngageScore/n*100) / 100
		totalDetail.MirrorScore = math.Round(totalDetail.MirrorScore/n*100) / 100

		models = append(models, ModelRapport{
			Model:        mn,
			RapportScore: math.Round(avgScore*10) / 10,
			Grade:        scoreToGrade(avgScore),
			Detail:       totalDetail,
			SampleCount:  len(entries),
			LabCount:     len(labs),
			Concerns:     totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].RapportScore > models[j].RapportScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.RapportScore
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
	b.WriteString("Rapport-Building Quality Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No rapport data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.RapportScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "warmth_score", m.Detail.WarmthScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "personalize_score", m.Detail.PersonalizeScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "engage_score", m.Detail.EngageScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "mirror_score", m.Detail.MirrorScore)
		fmt.Fprintf(&b, "    %-28s %d\n", "warmth_count", m.Detail.WarmthCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "cold_count", m.Detail.ColdCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "personalize_count", m.Detail.PersonalizeCount)
		fmt.Fprintf(&b, "    Concerns: %d\n\n", m.Concerns)
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
