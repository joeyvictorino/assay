package analogical

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type AnalogDetail struct {
	AnalogyFit       float64 `json:"analogy_fit"`
	MappingPrecision float64 `json:"mapping_precision"`
	AbstractionLevel float64 `json:"abstraction_level"`
	TransferQuality  float64 `json:"transfer_quality"`
	LimitAwareness   float64 `json:"limit_awareness"`
	AnalogyCount     int     `json:"analogy_count"`
	WeakAnalogyCount int     `json:"weak_analogy_count"`
	ForcedCount      int     `json:"forced_count"`
}

type ModelAnalog struct {
	Model        string       `json:"model"`
	AnalogScore  float64      `json:"analogical_score"`
	Grade        string       `json:"grade"`
	Detail       AnalogDetail `json:"detail"`
	SampleCount  int          `json:"sample_count"`
	LabCount     int          `json:"lab_count"`
	Concerns     int          `json:"concerns"`
}

type Report struct {
	Models     []ModelAnalog `json:"models"`
	BestModel  string        `json:"best_model"`
	WorstModel string        `json:"worst_model"`
	MeanScore  float64       `json:"mean_score"`
}

var analogyPhrases = []string{
	"is like", "similar to", "analogous to",
	"think of it as", "imagine it as",
	"just as", "in the same way that",
	"comparable to", "resembles",
	"mirrors", "parallels",
	"can be thought of as", "is akin to",
	"much like", "functions like",
}

var mappingPhrases = []string{
	"corresponds to", "maps to",
	"the equivalent of", "plays the role of",
	"serves as", "represents",
	"stands for", "symbolizes",
	"acts as", "fills the same role",
	"takes the place of", "in this analogy",
	"the counterpart", "this element",
	"structurally similar",
}

var abstractionPhrases = []string{
	"at a higher level", "abstractly speaking",
	"the underlying principle", "the general pattern",
	"more broadly", "the common structure",
	"the shared mechanism", "fundamentally",
	"the key insight", "the deep similarity",
	"beyond the surface", "the essence of",
	"the core idea", "structural parallel",
	"the abstract form",
}

var transferPhrases = []string{
	"applying this to", "transferring this idea",
	"using this framework", "by extension",
	"this principle applies", "the same logic holds",
	"we can apply", "extending this reasoning",
	"this carries over", "the lesson transfers",
	"importing this concept", "adapting this model",
	"the same pattern applies", "reusing this approach",
	"leveraging this insight",
}

var weakPhrases = []string{
	"sort of like", "kind of similar",
	"loosely related", "vaguely resembles",
	"not exactly but", "a rough comparison",
	"imperfect analogy", "crude comparison",
	"oversimplified", "stretching the analogy",
	"a bit of a stretch", "tenuous connection",
	"weak parallel",
}

var forcedPhrases = []string{
	"forced comparison", "false equivalence",
	"apples and oranges", "misleading analogy",
	"doesn't quite work", "breaks down because",
	"the analogy fails", "not really comparable",
	"fundamentally different", "superficial similarity",
	"coincidental resemblance", "category error",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, AnalogDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	analogy := countPhrases(combined, analogyPhrases)
	mapping := countPhrases(combined, mappingPhrases)
	abstraction := countPhrases(combined, abstractionPhrases)
	transfer := countPhrases(combined, transferPhrases)
	weak := countPhrases(combined, weakPhrases)
	forced := countPhrases(combined, forcedPhrases)

	totalAnalogy := analogy + mapping + abstraction + transfer

	fitScore := 0.2
	if words > 0 {
		fitScore = math.Min(1.0, float64(analogy)*2.0/float64(words)*10.0+0.2)
	}

	mappingScore := 0.2
	if words > 0 {
		mappingScore = math.Min(1.0, float64(mapping)*2.0/float64(words)*10.0+0.2)
	}

	abstractionScore := 0.2
	if abstraction > 0 || transfer > 0 {
		abstractionScore = math.Min(1.0, float64(abstraction+transfer)*0.12+0.2)
	}

	transferScore := 0.3
	if words > 0 {
		transferScore = math.Min(1.0, float64(transfer)*2.0/float64(words)*10.0+0.3)
	}

	limitScore := 0.2
	if weak == 0 && forced == 0 {
		limitScore = 0.8
	} else if weak+forced < 3 {
		limitScore = 0.5
	}

	detail := AnalogDetail{
		AnalogyFit:       math.Round(fitScore*100) / 100,
		MappingPrecision: math.Round(mappingScore*100) / 100,
		AbstractionLevel: math.Round(abstractionScore*100) / 100,
		TransferQuality:  math.Round(transferScore*100) / 100,
		LimitAwareness:   math.Round(limitScore*100) / 100,
		AnalogyCount:     totalAnalogy,
		WeakAnalogyCount: weak,
		ForcedCount:      forced,
	}

	score := 30.0
	score += fitScore * 15.0
	score += mappingScore * 15.0
	score += abstractionScore * 10.0
	score += transferScore * 10.0
	score += limitScore * 10.0

	score -= float64(weak) * 3.0
	score -= float64(forced) * 5.0

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

	var models []ModelAnalog
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail AnalogDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.AnalogyFit += detail.AnalogyFit
			totalDetail.MappingPrecision += detail.MappingPrecision
			totalDetail.AbstractionLevel += detail.AbstractionLevel
			totalDetail.TransferQuality += detail.TransferQuality
			totalDetail.LimitAwareness += detail.LimitAwareness
			totalDetail.AnalogyCount += detail.AnalogyCount
			totalDetail.WeakAnalogyCount += detail.WeakAnalogyCount
			totalDetail.ForcedCount += detail.ForcedCount

			if detail.WeakAnalogyCount > 3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.AnalogyFit = math.Round(totalDetail.AnalogyFit/n*100) / 100
		totalDetail.MappingPrecision = math.Round(totalDetail.MappingPrecision/n*100) / 100
		totalDetail.AbstractionLevel = math.Round(totalDetail.AbstractionLevel/n*100) / 100
		totalDetail.TransferQuality = math.Round(totalDetail.TransferQuality/n*100) / 100
		totalDetail.LimitAwareness = math.Round(totalDetail.LimitAwareness/n*100) / 100

		models = append(models, ModelAnalog{
			Model:       mn,
			AnalogScore: math.Round(avgScore*10) / 10,
			Grade:       scoreToGrade(avgScore),
			Detail:      totalDetail,
			SampleCount: len(entries),
			LabCount:    len(labs),
			Concerns:    totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].AnalogScore > models[j].AnalogScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.AnalogScore
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
	b.WriteString("Analogical Reasoning Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No analogical reasoning data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.AnalogScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "analogy_fit", m.Detail.AnalogyFit)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "mapping_precision", m.Detail.MappingPrecision)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "abstraction_level", m.Detail.AbstractionLevel)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "transfer_quality", m.Detail.TransferQuality)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "limit_awareness", m.Detail.LimitAwareness)
		fmt.Fprintf(&b, "    %-28s %d\n", "analogy_count", m.Detail.AnalogyCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "weak_analogy_count", m.Detail.WeakAnalogyCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "forced_count", m.Detail.ForcedCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
