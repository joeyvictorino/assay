package creativity

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type CreativityDetail struct {
	Novelty       float64 `json:"novelty"`
	Elaboration   float64 `json:"elaboration"`
	Flexibility   float64 `json:"flexibility"`
	Originality   float64 `json:"originality"`
	ExprRange     float64 `json:"expression_range"`
	NovelCount    int     `json:"novel_count"`
	ClicheCount   int     `json:"cliche_count"`
	MetaphorCount int     `json:"metaphor_count"`
}

type ModelCreativity struct {
	Model           string           `json:"model"`
	CreativityScore float64          `json:"creativity_score"`
	Grade           string           `json:"grade"`
	Detail          CreativityDetail `json:"detail"`
	SampleCount     int              `json:"sample_count"`
	LabCount        int              `json:"lab_count"`
	Concerns        int              `json:"concerns"`
}

type Report struct {
	Models     []ModelCreativity `json:"models"`
	BestModel  string            `json:"best_model"`
	WorstModel string            `json:"worst_model"`
	MeanScore  float64           `json:"mean_score"`
}

var novelPhrases = []string{
	"alternatively", "another approach", "what if",
	"consider this", "imagine", "picture this",
	"from a different angle", "unconventional",
	"creative solution", "innovative", "novel approach",
	"fresh perspective", "outside the box",
	"reimagine", "transform",
}

var elaborationPhrases = []string{
	"for example", "to illustrate", "specifically",
	"in detail", "let me elaborate", "expanding on",
	"to break this down", "more precisely",
	"going deeper", "the nuance here",
	"a concrete example", "to paint a picture",
}

var flexibilityPhrases = []string{
	"on one hand", "alternatively", "another way",
	"from this perspective", "looking at it differently",
	"switching gears", "a different lens",
	"conversely", "in contrast", "by comparison",
	"multiple approaches", "several options",
}

var clichePhrases = []string{
	"at the end of the day", "it is what it is",
	"think outside the box", "low-hanging fruit",
	"synergy", "paradigm shift", "move the needle",
	"circle back", "deep dive", "best practices",
	"leverage", "utilize", "streamline",
}

var metaphorPhrases = []string{
	"like a", "as if", "metaphorically",
	"think of it as", "similar to how",
	"just as", "in the same way that",
	"picture", "envision", "akin to",
	"mirrors", "echoes", "resonates with",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, CreativityDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	novel := countPhrases(combined, novelPhrases)
	elab := countPhrases(combined, elaborationPhrases)
	flex := countPhrases(combined, flexibilityPhrases)
	cliche := countPhrases(combined, clichePhrases)
	metaphor := countPhrases(combined, metaphorPhrases)

	noveltyScore := 0.4
	if words > 0 {
		noveltyScore = math.Min(1.0, float64(novel)*2.0/float64(words)*10.0+0.4)
	}

	elabScore := 0.4
	if words > 0 {
		elabScore = math.Min(1.0, float64(elab)*2.0/float64(words)*10.0+0.4)
	}

	flexScore := 0.4
	if flex > 0 {
		flexScore = math.Min(1.0, float64(flex)*0.2+0.4)
	}

	origScore := 0.5
	if cliche > 0 {
		origScore = math.Max(0, 0.5-float64(cliche)*0.1)
	}
	if novel > cliche {
		origScore = math.Min(1.0, origScore+0.3)
	}

	exprScore := 0.4
	if metaphor > 0 {
		exprScore = math.Min(1.0, float64(metaphor)*0.2+0.4)
	}

	detail := CreativityDetail{
		Novelty:       math.Round(noveltyScore*100) / 100,
		Elaboration:   math.Round(elabScore*100) / 100,
		Flexibility:   math.Round(flexScore*100) / 100,
		Originality:   math.Round(origScore*100) / 100,
		ExprRange:     math.Round(exprScore*100) / 100,
		NovelCount:    novel,
		ClicheCount:   cliche,
		MetaphorCount: metaphor,
	}

	score := 35.0
	score += noveltyScore * 13.0
	score += elabScore * 12.0
	score += flexScore * 10.0
	score += origScore * 12.0
	score += exprScore * 10.0

	score -= float64(cliche) * 3.0

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

	var models []ModelCreativity
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail CreativityDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.Novelty += detail.Novelty
			totalDetail.Elaboration += detail.Elaboration
			totalDetail.Flexibility += detail.Flexibility
			totalDetail.Originality += detail.Originality
			totalDetail.ExprRange += detail.ExprRange
			totalDetail.NovelCount += detail.NovelCount
			totalDetail.ClicheCount += detail.ClicheCount
			totalDetail.MetaphorCount += detail.MetaphorCount

			if detail.ClicheCount > 2 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.Novelty = math.Round(totalDetail.Novelty/n*100) / 100
		totalDetail.Elaboration = math.Round(totalDetail.Elaboration/n*100) / 100
		totalDetail.Flexibility = math.Round(totalDetail.Flexibility/n*100) / 100
		totalDetail.Originality = math.Round(totalDetail.Originality/n*100) / 100
		totalDetail.ExprRange = math.Round(totalDetail.ExprRange/n*100) / 100

		models = append(models, ModelCreativity{
			Model:           mn,
			CreativityScore: math.Round(avgScore*10) / 10,
			Grade:           scoreToGrade(avgScore),
			Detail:          totalDetail,
			SampleCount:     len(entries),
			LabCount:        len(labs),
			Concerns:        totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].CreativityScore > models[j].CreativityScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.CreativityScore
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
	b.WriteString("Creative Response Quality Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No creativity data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.CreativityScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "novelty", m.Detail.Novelty)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "elaboration", m.Detail.Elaboration)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "flexibility", m.Detail.Flexibility)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "originality", m.Detail.Originality)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "expression_range", m.Detail.ExprRange)
		fmt.Fprintf(&b, "    %-28s %d\n", "novel_count", m.Detail.NovelCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "cliche_count", m.Detail.ClicheCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "metaphor_count", m.Detail.MetaphorCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
