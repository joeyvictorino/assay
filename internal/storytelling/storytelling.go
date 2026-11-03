package storytelling

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type StorytellingDetail struct {
	NarrativeArcScore float64 `json:"narrative_arc_score"`
	EngagementScore   float64 `json:"engagement_score"`
	IllustrationScore float64 `json:"illustration_score"`
	VividScore        float64 `json:"vivid_score"`
	CoherenceScore    float64 `json:"coherence_score"`
	NarrativeCount    int     `json:"narrative_count"`
	IllustrationCount int     `json:"illustration_count"`
	VividCount        int     `json:"vivid_count"`
}

type ModelStorytelling struct {
	Model             string             `json:"model"`
	StorytellingScore float64            `json:"storytelling_score"`
	Grade             string             `json:"grade"`
	Detail            StorytellingDetail `json:"detail"`
	SampleCount       int                `json:"sample_count"`
	LabCount          int                `json:"lab_count"`
	Concerns          int                `json:"concerns"`
}

type Report struct {
	Models     []ModelStorytelling `json:"models"`
	BestModel  string              `json:"best_model"`
	WorstModel string              `json:"worst_model"`
	MeanScore  float64             `json:"mean_score"`
}

var narrativeArcPhrases = []string{
	"narrative structure", "story arc", "beginning middle end",
	"narrative flow", "plot structure", "story progression",
	"narrative sequence", "logical progression", "in conclusion",
	"to begin with", "first", "finally", "ultimately",
	"as a result", "in summary",
}

var engagementPhrases = []string{
	"engaging", "compelling", "captivating",
	"draws the reader", "hooks the audience", "maintains interest",
	"attention-grabbing", "riveting", "fascinating",
	"intriguing", "thought-provoking", "stimulating",
	"absorbing", "gripping", "enthralling",
}

var illustrationPhrases = []string{
	"for example", "such as", "analogous to",
	"consider the scenario", "to illustrate", "case in point",
	"imagine", "picture this", "for instance",
	"take the case of", "consider how", "as demonstrated by",
	"as shown by", "a good example", "in practice",
}

var vividPhrases = []string{
	"vivid", "descriptive", "evocative",
	"paints a picture", "imagery", "colorful language",
	"sensory detail", "expressive", "picturesque",
	"striking", "dramatic", "atmospheric",
	"visceral", "palpable", "tangible",
}

var coherencePhrases = []string{
	"coherent narrative", "smooth transition", "logical flow",
	"well-connected", "seamless progression", "unified story",
	"consistent thread", "flowing narrative", "therefore",
	"consequently", "moreover", "furthermore",
	"building on", "as a result", "this leads to",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, StorytellingDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	narrative := countPhrases(combined, narrativeArcPhrases)
	engagement := countPhrases(combined, engagementPhrases)
	illustration := countPhrases(combined, illustrationPhrases)
	vivid := countPhrases(combined, vividPhrases)
	coherence := countPhrases(combined, coherencePhrases)

	narrativeScore := 0.2
	if narrative > 0 {
		narrativeScore = math.Min(1.0, float64(narrative)*0.12+0.2)
	}

	engagementScore := 0.2
	if engagement > 0 {
		engagementScore = math.Min(1.0, float64(engagement)*0.15+0.2)
	}

	illustrationScore := 0.2
	if words > 0 {
		illustrationScore = math.Min(1.0, float64(illustration)*2.0/float64(words)*10.0+0.2)
	}

	vividScore := 0.2
	if vivid > 0 {
		vividScore = math.Min(1.0, float64(vivid)*0.15+0.2)
	}

	coherenceScore := 0.2
	if coherence > 0 {
		coherenceScore = math.Min(1.0, float64(coherence)*0.12+0.2)
	}

	detail := StorytellingDetail{
		NarrativeArcScore: math.Round(narrativeScore*100) / 100,
		EngagementScore:   math.Round(engagementScore*100) / 100,
		IllustrationScore: math.Round(illustrationScore*100) / 100,
		VividScore:        math.Round(vividScore*100) / 100,
		CoherenceScore:    math.Round(coherenceScore*100) / 100,
		NarrativeCount:    narrative,
		IllustrationCount: illustration,
		VividCount:        vivid,
	}

	score := 20.0
	score += narrativeScore * 15.0
	score += engagementScore * 15.0
	score += illustrationScore * 15.0
	score += vividScore * 15.0
	score += coherenceScore * 15.0

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

	var models []ModelStorytelling
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail StorytellingDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.NarrativeArcScore += detail.NarrativeArcScore
			totalDetail.EngagementScore += detail.EngagementScore
			totalDetail.IllustrationScore += detail.IllustrationScore
			totalDetail.VividScore += detail.VividScore
			totalDetail.CoherenceScore += detail.CoherenceScore
			totalDetail.NarrativeCount += detail.NarrativeCount
			totalDetail.IllustrationCount += detail.IllustrationCount
			totalDetail.VividCount += detail.VividCount

			if detail.NarrativeCount == 0 && detail.IllustrationCount == 0 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.NarrativeArcScore = math.Round(totalDetail.NarrativeArcScore/n*100) / 100
		totalDetail.EngagementScore = math.Round(totalDetail.EngagementScore/n*100) / 100
		totalDetail.IllustrationScore = math.Round(totalDetail.IllustrationScore/n*100) / 100
		totalDetail.VividScore = math.Round(totalDetail.VividScore/n*100) / 100
		totalDetail.CoherenceScore = math.Round(totalDetail.CoherenceScore/n*100) / 100

		models = append(models, ModelStorytelling{
			Model:             mn,
			StorytellingScore: math.Round(avgScore*10) / 10,
			Grade:             scoreToGrade(avgScore),
			Detail:            totalDetail,
			SampleCount:       len(entries),
			LabCount:          len(labs),
			Concerns:          totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].StorytellingScore > models[j].StorytellingScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.StorytellingScore
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
	b.WriteString("Storytelling Quality Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No storytelling data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.StorytellingScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "narrative_arc_score", m.Detail.NarrativeArcScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "engagement_score", m.Detail.EngagementScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "illustration_score", m.Detail.IllustrationScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "vivid_score", m.Detail.VividScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "coherence_score", m.Detail.CoherenceScore)
		fmt.Fprintf(&b, "    %-28s %d\n", "narrative_count", m.Detail.NarrativeCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "illustration_count", m.Detail.IllustrationCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "vivid_count", m.Detail.VividCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
