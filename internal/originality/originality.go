package originality

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type OriginalityDetail struct {
	NoveltyScore     float64 `json:"novelty_score"`
	CreativeExpr     float64 `json:"creative_expression"`
	UniqueInsight    float64 `json:"unique_insight"`
	AvoidsCliche     float64 `json:"avoids_cliche"`
	PerspectiveScore float64 `json:"perspective_score"`
	VarietyScore     float64 `json:"variety_score"`
	WordCount        int     `json:"word_count"`
	UniqueWords      int     `json:"unique_words"`
}

type ModelOriginality struct {
	Model            string            `json:"model"`
	OriginalityScore float64           `json:"originality_score"`
	Grade            string            `json:"grade"`
	Detail           OriginalityDetail `json:"detail"`
	SampleCount      int               `json:"sample_count"`
	LabCount         int               `json:"lab_count"`
	Concerns         int               `json:"concerns"`
}

type Report struct {
	Models     []ModelOriginality `json:"models"`
	BestModel  string             `json:"best_model"`
	WorstModel string             `json:"worst_model"`
	MeanScore  float64            `json:"mean_score"`
}

var noveltyIndicators = []string{
	"novel approach", "unique perspective", "innovative",
	"unconventional", "creative solution", "fresh take",
	"original idea", "new angle", "alternative approach",
	"rethink", "reimagine", "paradigm shift",
	"breakthrough", "cutting-edge", "pioneering",
}

var creativeIndicators = []string{
	"imagine", "envision", "what if",
	"thought experiment", "hypothetically", "consider this",
	"metaphor", "analogy", "picture this",
	"story", "narrative", "scenario",
	"let me paint", "visualization", "mental model",
}

var insightIndicators = []string{
	"insight", "observation", "key takeaway",
	"underlying pattern", "deeper meaning", "root cause",
	"hidden connection", "counterintuitive", "surprising",
	"overlooked", "nuanced", "subtle point",
	"not immediately obvious", "often missed", "critical distinction",
}

var clichePhrases = []string{
	"at the end of the day", "it is what it is",
	"think outside the box", "low-hanging fruit",
	"move the needle", "circle back", "synergy",
	"paradigm", "leverage", "best practices",
	"deep dive", "touch base", "run it up the flagpole",
	"boil the ocean", "game changer",
}

var perspectiveIndicators = []string{
	"on the other hand", "from another angle",
	"contrasting view", "devil's advocate", "playing both sides",
	"reframing", "through the lens", "considered differently",
	"alternative interpretation", "multiple perspectives",
	"viewed from", "another way to see", "flip side",
}

var genericPhrases = []string{
	"as we all know", "it goes without saying",
	"needless to say", "obviously", "of course",
	"everyone knows", "it's common knowledge",
	"as you know", "goes without saying",
	"the fact of the matter", "at this point in time",
	"in today's world", "in this day and age",
}

func countIndicators(text string, indicators []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, ind := range indicators {
		count += strings.Count(lower, ind)
	}
	return count
}

func countUniqueWords(text string) int {
	words := strings.Fields(strings.ToLower(text))
	seen := map[string]bool{}
	for _, w := range words {
		seen[w] = true
	}
	return len(seen)
}

func scoreResponse(response, evidence string) (float64, OriginalityDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))
	unique := countUniqueWords(combined)

	novelty := countIndicators(combined, noveltyIndicators)
	creative := countIndicators(combined, creativeIndicators)
	insight := countIndicators(combined, insightIndicators)
	cliches := countIndicators(combined, clichePhrases)
	perspective := countIndicators(combined, perspectiveIndicators)
	generic := countIndicators(combined, genericPhrases)

	noveltyScore := math.Min(1.0, float64(novelty)*0.15+0.2)
	creativeScore := math.Min(1.0, float64(creative)*0.12+0.2)
	insightScore := math.Min(1.0, float64(insight)*0.15+0.2)

	avoidsCliche := 0.8
	if cliches > 0 {
		avoidsCliche = math.Max(0, avoidsCliche-float64(cliches)*0.15)
	}
	if generic > 0 {
		avoidsCliche = math.Max(0, avoidsCliche-float64(generic)*0.1)
	}

	perspectiveScore := math.Min(1.0, float64(perspective)*0.15+0.2)

	varietyScore := 0.3
	if words > 0 {
		ratio := float64(unique) / float64(words)
		varietyScore = math.Min(1.0, ratio+0.1)
	}

	detail := OriginalityDetail{
		NoveltyScore:     math.Round(noveltyScore*100) / 100,
		CreativeExpr:     math.Round(creativeScore*100) / 100,
		UniqueInsight:    math.Round(insightScore*100) / 100,
		AvoidsCliche:     math.Round(avoidsCliche*100) / 100,
		PerspectiveScore: math.Round(perspectiveScore*100) / 100,
		VarietyScore:     math.Round(varietyScore*100) / 100,
		WordCount:        words,
		UniqueWords:      unique,
	}

	score := 15.0
	score += noveltyScore * 18.0
	score += creativeScore * 15.0
	score += insightScore * 18.0
	score += avoidsCliche * 15.0
	score += perspectiveScore * 10.0
	score += varietyScore * 9.0

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

	var models []ModelOriginality
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail OriginalityDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.NoveltyScore += detail.NoveltyScore
			totalDetail.CreativeExpr += detail.CreativeExpr
			totalDetail.UniqueInsight += detail.UniqueInsight
			totalDetail.AvoidsCliche += detail.AvoidsCliche
			totalDetail.PerspectiveScore += detail.PerspectiveScore
			totalDetail.VarietyScore += detail.VarietyScore
			totalDetail.WordCount += detail.WordCount
			totalDetail.UniqueWords += detail.UniqueWords

			if detail.NoveltyScore < 0.3 && detail.CreativeExpr < 0.3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.NoveltyScore = math.Round(totalDetail.NoveltyScore/n*100) / 100
		totalDetail.CreativeExpr = math.Round(totalDetail.CreativeExpr/n*100) / 100
		totalDetail.UniqueInsight = math.Round(totalDetail.UniqueInsight/n*100) / 100
		totalDetail.AvoidsCliche = math.Round(totalDetail.AvoidsCliche/n*100) / 100
		totalDetail.PerspectiveScore = math.Round(totalDetail.PerspectiveScore/n*100) / 100
		totalDetail.VarietyScore = math.Round(totalDetail.VarietyScore/n*100) / 100

		models = append(models, ModelOriginality{
			Model:            mn,
			OriginalityScore: math.Round(avgScore*10) / 10,
			Grade:            scoreToGrade(avgScore),
			Detail:           totalDetail,
			SampleCount:      len(entries),
			LabCount:         len(labs),
			Concerns:         totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].OriginalityScore > models[j].OriginalityScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.OriginalityScore
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
	b.WriteString("Response Originality Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No originality data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.OriginalityScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "novelty_score", m.Detail.NoveltyScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "creative_expression", m.Detail.CreativeExpr)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "unique_insight", m.Detail.UniqueInsight)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "avoids_cliche", m.Detail.AvoidsCliche)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "perspective_score", m.Detail.PerspectiveScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "variety_score", m.Detail.VarietyScore)
		fmt.Fprintf(&b, "    %-28s %d\n", "word_count", m.Detail.WordCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "unique_words", m.Detail.UniqueWords)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
