package informativeness

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type InformativenessDetail struct {
	ContentScore   float64 `json:"content_score"`
	SpecificScore  float64 `json:"specific_score"`
	CoverageScore  float64 `json:"coverage_score"`
	NoveltyScore   float64 `json:"novelty_score"`
	ContentCount   int     `json:"content_count"`
	VagueCount     int     `json:"vague_count"`
	SpecificCount  int     `json:"specific_count"`
}

type ModelInformativeness struct {
	Model               string                `json:"model"`
	InformativenessScore float64              `json:"informativeness_score"`
	Grade               string                `json:"grade"`
	Detail              InformativenessDetail `json:"detail"`
	SampleCount         int                   `json:"sample_count"`
	LabCount            int                   `json:"lab_count"`
	Concerns            int                   `json:"concerns"`
}

type Report struct {
	Models     []ModelInformativeness `json:"models"`
	BestModel  string                 `json:"best_model"`
	WorstModel string                 `json:"worst_model"`
	MeanScore  float64                `json:"mean_score"`
}

var contentPhrases = []string{
	"the key difference is", "this works by", "the mechanism involves",
	"the architecture consists of", "there are three main",
	"the process involves", "specifically", "in practice",
	"the implementation uses", "the algorithm",
	"the data structure", "the protocol", "technically",
	"the root cause is", "the tradeoff is",
}

var vaguePhrases = []string{
	"it depends", "there are many ways", "it varies",
	"generally speaking", "in most cases", "it could be",
	"there are pros and cons", "it really depends on",
	"hard to say definitively", "there is no single answer",
	"many factors", "it is complicated", "it all depends",
	"various approaches", "different perspectives",
}

var specificPhrases = []string{
	"for example", "such as", "including",
	"measured at", "approximately", "the exact value is",
	"step 1", "step 2", "first", "second", "third",
	"the output is", "the input requires", "returns",
	"the complexity is", "benchmarked at",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, strings.ToLower(p))
	}
	return count
}

func scoreResponse(response, evidence string) (float64, InformativenessDetail) {
	combined := response + " " + evidence

	content := countPhrases(combined, contentPhrases)
	vague := countPhrases(combined, vaguePhrases)
	specific := countPhrases(combined, specificPhrases)

	contentScore := 0.3
	if content > 0 {
		contentScore = math.Min(1.0, float64(content)*0.12+0.3)
	}

	specificScore := 0.3
	if specific > 0 {
		specificScore = math.Min(1.0, float64(specific)*0.12+0.3)
	}

	coverageScore := 0.5
	if vague > 0 {
		coverageScore = math.Max(0, coverageScore-float64(vague)*0.12)
	}
	if content > 0 {
		coverageScore = math.Min(1.0, coverageScore+float64(content)*0.05)
	}

	noveltyScore := 0.5
	if vague > 2 {
		noveltyScore = math.Max(0, noveltyScore-float64(vague)*0.1)
	}
	if specific > 0 {
		noveltyScore = math.Min(1.0, noveltyScore+float64(specific)*0.08)
	}

	detail := InformativenessDetail{
		ContentScore:  math.Round(contentScore*100) / 100,
		SpecificScore: math.Round(specificScore*100) / 100,
		CoverageScore: math.Round(coverageScore*100) / 100,
		NoveltyScore:  math.Round(noveltyScore*100) / 100,
		ContentCount:  content,
		VagueCount:    vague,
		SpecificCount: specific,
	}

	score := 25.0
	score += contentScore * 20.0
	score += specificScore * 15.0
	score += coverageScore * 20.0
	score += noveltyScore * 15.0
	score -= float64(vague) * 4.0

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

	var models []ModelInformativeness
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail InformativenessDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			ev := evidenceText(e)
			score, detail := scoreResponse(e.Summary, ev)
			totalScore += score
			totalDetail.ContentScore += detail.ContentScore
			totalDetail.SpecificScore += detail.SpecificScore
			totalDetail.CoverageScore += detail.CoverageScore
			totalDetail.NoveltyScore += detail.NoveltyScore
			totalDetail.ContentCount += detail.ContentCount
			totalDetail.VagueCount += detail.VagueCount
			totalDetail.SpecificCount += detail.SpecificCount

			if detail.VagueCount > 3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.ContentScore = math.Round(totalDetail.ContentScore/n*100) / 100
		totalDetail.SpecificScore = math.Round(totalDetail.SpecificScore/n*100) / 100
		totalDetail.CoverageScore = math.Round(totalDetail.CoverageScore/n*100) / 100
		totalDetail.NoveltyScore = math.Round(totalDetail.NoveltyScore/n*100) / 100

		models = append(models, ModelInformativeness{
			Model:                mn,
			InformativenessScore: math.Round(avgScore*10) / 10,
			Grade:                scoreToGrade(avgScore),
			Detail:               totalDetail,
			SampleCount:          len(entries),
			LabCount:             len(labs),
			Concerns:             totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].InformativenessScore > models[j].InformativenessScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.InformativenessScore
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
	b.WriteString("Informativeness Quality Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No informativeness data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.InformativenessScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "content_score", m.Detail.ContentScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "specific_score", m.Detail.SpecificScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "coverage_score", m.Detail.CoverageScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "novelty_score", m.Detail.NoveltyScore)
		fmt.Fprintf(&b, "    %-28s %d\n", "content_count", m.Detail.ContentCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "vague_count", m.Detail.VagueCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "specific_count", m.Detail.SpecificCount)
		fmt.Fprintf(&b, "    Concerns: %d\n\n", m.Concerns)
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
