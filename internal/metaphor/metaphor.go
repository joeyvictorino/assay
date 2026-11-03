package metaphor

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type MetaphorDetail struct {
	AptnessScore    float64 `json:"aptness_score"`
	NoveltyScore    float64 `json:"novelty_score"`
	ClarityScore    float64 `json:"clarity_score"`
	ExtensionScore  float64 `json:"extension_score"`
	ConsistencyScore float64 `json:"consistency_score"`
	AptnessCount    int     `json:"aptness_count"`
	NoveltyCount    int     `json:"novelty_count"`
	MixedCount      int     `json:"mixed_count"`
}

type ModelMetaphor struct {
	Model         string         `json:"model"`
	MetaphorScore float64        `json:"metaphor_score"`
	Grade         string         `json:"grade"`
	Detail        MetaphorDetail `json:"detail"`
	SampleCount   int            `json:"sample_count"`
	LabCount      int            `json:"lab_count"`
	Concerns      int            `json:"concerns"`
}

type Report struct {
	Models     []ModelMetaphor `json:"models"`
	BestModel  string          `json:"best_model"`
	WorstModel string          `json:"worst_model"`
	MeanScore  float64         `json:"mean_score"`
}

var aptnessPhrases = []string{
	"apt metaphor", "fitting analogy", "well-chosen comparison",
	"illuminating parallel", "effective metaphor", "suitable analogy",
	"resonant comparison", "apt illustration", "telling metaphor",
	"well-matched", "appropriate figure", "relevant analogy",
	"congruent metaphor", "apt comparison", "pertinent parallel",
}

var noveltyPhrases = []string{
	"creative metaphor", "original analogy", "fresh comparison",
	"inventive imagery", "novel figure", "imaginative parallel",
	"unexpected metaphor", "striking analogy", "unique comparison",
	"innovative imagery", "vivid metaphor", "evocative figure",
	"surprising parallel", "distinctive analogy", "unconventional",
}

var clarityPhrases = []string{
	"clear analogy", "understandable metaphor", "accessible comparison",
	"transparent figure", "intuitive parallel", "self-evident analogy",
	"plain metaphor", "obvious comparison", "lucid imagery",
	"easy to grasp", "straightforward analogy", "unmistakable",
	"intelligible metaphor", "explicit comparison", "comprehensible",
}

var extensionPhrases = []string{
	"extended metaphor", "sustained analogy", "developed comparison",
	"elaborated figure", "thorough parallel", "detailed metaphor",
	"built upon", "carries through", "metaphor extends",
	"analogy develops", "further illustrates", "deepens the comparison",
	"expands the metaphor", "continues the parallel", "layers the analogy",
}

var mixedPhrases = []string{
	"mixed metaphor", "conflicting imagery", "inconsistent analogy",
	"contradictory figures", "jarring comparison", "clashing metaphors",
	"muddled imagery", "tangled analogy", "confused figure",
	"incompatible comparison", "dissonant metaphor", "incoherent imagery",
	"jumbled analogy", "discordant figures", "mismatched comparison",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, MetaphorDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	aptness := countPhrases(combined, aptnessPhrases)
	novelty := countPhrases(combined, noveltyPhrases)
	clarity := countPhrases(combined, clarityPhrases)
	extension := countPhrases(combined, extensionPhrases)
	mixed := countPhrases(combined, mixedPhrases)

	aptnessScore := 0.3
	if words > 0 {
		aptnessScore = math.Min(1.0, float64(aptness)*2.0/float64(words)*10.0+0.3)
	}

	noveltyScore := 0.3
	if novelty > 0 {
		noveltyScore = math.Min(1.0, float64(novelty)*0.12+0.3)
	}

	clarityScore := 0.3
	if clarity > 0 {
		clarityScore = math.Min(1.0, float64(clarity)*0.12+0.3)
	}

	extensionScore := 0.2
	if extension > 0 {
		extensionScore = math.Min(1.0, float64(extension)*0.15+0.2)
	}

	consistencyScore := 0.5
	if mixed > 0 {
		consistencyScore = math.Max(0, consistencyScore-float64(mixed)*0.1)
	}
	if words > 0 && words < 300 {
		consistencyScore = math.Min(1.0, consistencyScore+0.15)
	}

	detail := MetaphorDetail{
		AptnessScore:     math.Round(aptnessScore*100) / 100,
		NoveltyScore:     math.Round(noveltyScore*100) / 100,
		ClarityScore:     math.Round(clarityScore*100) / 100,
		ExtensionScore:   math.Round(extensionScore*100) / 100,
		ConsistencyScore: math.Round(consistencyScore*100) / 100,
		AptnessCount:     aptness,
		NoveltyCount:     novelty,
		MixedCount:       mixed,
	}

	score := 25.0
	score += aptnessScore * 15.0
	score += noveltyScore * 15.0
	score += clarityScore * 15.0
	score += extensionScore * 10.0
	score += consistencyScore * 15.0

	score -= float64(mixed) * 3.0

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

	var models []ModelMetaphor
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail MetaphorDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.AptnessScore += detail.AptnessScore
			totalDetail.NoveltyScore += detail.NoveltyScore
			totalDetail.ClarityScore += detail.ClarityScore
			totalDetail.ExtensionScore += detail.ExtensionScore
			totalDetail.ConsistencyScore += detail.ConsistencyScore
			totalDetail.AptnessCount += detail.AptnessCount
			totalDetail.NoveltyCount += detail.NoveltyCount
			totalDetail.MixedCount += detail.MixedCount

			if detail.MixedCount > 3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.AptnessScore = math.Round(totalDetail.AptnessScore/n*100) / 100
		totalDetail.NoveltyScore = math.Round(totalDetail.NoveltyScore/n*100) / 100
		totalDetail.ClarityScore = math.Round(totalDetail.ClarityScore/n*100) / 100
		totalDetail.ExtensionScore = math.Round(totalDetail.ExtensionScore/n*100) / 100
		totalDetail.ConsistencyScore = math.Round(totalDetail.ConsistencyScore/n*100) / 100

		models = append(models, ModelMetaphor{
			Model:         mn,
			MetaphorScore: math.Round(avgScore*10) / 10,
			Grade:         scoreToGrade(avgScore),
			Detail:        totalDetail,
			SampleCount:   len(entries),
			LabCount:      len(labs),
			Concerns:      totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].MetaphorScore > models[j].MetaphorScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.MetaphorScore
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
	b.WriteString("Metaphor Quality Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No metaphor data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.MetaphorScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "aptness_score", m.Detail.AptnessScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "novelty_score", m.Detail.NoveltyScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "clarity_score", m.Detail.ClarityScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "extension_score", m.Detail.ExtensionScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "consistency_score", m.Detail.ConsistencyScore)
		fmt.Fprintf(&b, "    %-28s %d\n", "aptness_count", m.Detail.AptnessCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "novelty_count", m.Detail.NoveltyCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "mixed_count", m.Detail.MixedCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
