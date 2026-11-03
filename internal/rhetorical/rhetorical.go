package rhetorical

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type RhetoricalDetail struct {
	LogosScore     float64 `json:"logos_score"`
	EthosScore     float64 `json:"ethos_score"`
	PathosScore    float64 `json:"pathos_score"`
	StructureScore float64 `json:"structure_score"`
	ClarityScore   float64 `json:"clarity_score"`
	LogosCount     int     `json:"logos_count"`
	EthosCount     int     `json:"ethos_count"`
	PathosCount    int     `json:"pathos_count"`
}

type ModelRhetorical struct {
	Model           string           `json:"model"`
	RhetoricalScore float64          `json:"rhetorical_score"`
	Grade           string           `json:"grade"`
	Detail          RhetoricalDetail `json:"detail"`
	SampleCount     int              `json:"sample_count"`
	LabCount        int              `json:"lab_count"`
	Concerns        int              `json:"concerns"`
}

type Report struct {
	Models     []ModelRhetorical `json:"models"`
	BestModel  string            `json:"best_model"`
	WorstModel string            `json:"worst_model"`
	MeanScore  float64           `json:"mean_score"`
}

var logosPhrases = []string{
	"because", "therefore", "consequently", "as a result",
	"the evidence shows", "data indicates", "research demonstrates",
	"statistically", "logically", "it follows that",
	"based on the data", "empirically", "quantitatively",
	"the analysis shows", "measurements indicate",
}

var ethosPhrases = []string{
	"according to", "experts agree", "research by",
	"studies show", "the literature", "peer-reviewed",
	"authoritative source", "established practice",
	"industry standard", "best practice", "widely accepted",
	"as documented in", "the specification states",
	"the official documentation", "recognized authority",
}

var pathosPhrases = []string{
	"imagine", "consider the impact", "think about",
	"the consequences", "real-world effect", "in practice",
	"users will experience", "the risk is", "importantly",
	"critically", "notably", "significantly",
	"the benefit is", "the advantage", "the danger",
}

var structurePhrases = []string{
	"first", "second", "third", "finally",
	"to begin", "in addition", "moreover",
	"on the other hand", "in contrast", "however",
	"in summary", "to conclude", "in conclusion",
	"the key points are", "to summarize",
}

var clarityPhrases = []string{
	"specifically", "in other words", "for example",
	"to clarify", "that is", "namely",
	"to be precise", "put simply", "concretely",
	"for instance", "such as", "to illustrate",
}

var weakPhrases = []string{
	"sort of", "kind of", "maybe",
	"i guess", "whatever", "or something",
	"you know what i mean", "it's complicated",
	"hard to say", "not sure", "unclear",
	"confusing",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, RhetoricalDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	logos := countPhrases(combined, logosPhrases)
	ethos := countPhrases(combined, ethosPhrases)
	pathos := countPhrases(combined, pathosPhrases)
	structure := countPhrases(combined, structurePhrases)
	clarity := countPhrases(combined, clarityPhrases)
	weak := countPhrases(combined, weakPhrases)

	logosScore := 0.2
	if words > 0 {
		logosScore = math.Min(1.0, float64(logos)*2.0/float64(words)*10.0+0.2)
	}

	ethosScore := 0.2
	if words > 0 {
		ethosScore = math.Min(1.0, float64(ethos)*2.0/float64(words)*10.0+0.2)
	}

	pathosScore := 0.2
	if words > 0 {
		pathosScore = math.Min(1.0, float64(pathos)*2.0/float64(words)*10.0+0.2)
	}

	structureScore := 0.2
	if structure > 0 {
		structureScore = math.Min(1.0, float64(structure)*0.12+0.2)
	}

	clarityScore := 0.3
	if clarity > 0 {
		clarityScore = math.Min(1.0, float64(clarity)*0.15+0.3)
	}

	detail := RhetoricalDetail{
		LogosScore:     math.Round(logosScore*100) / 100,
		EthosScore:     math.Round(ethosScore*100) / 100,
		PathosScore:    math.Round(pathosScore*100) / 100,
		StructureScore: math.Round(structureScore*100) / 100,
		ClarityScore:   math.Round(clarityScore*100) / 100,
		LogosCount:     logos,
		EthosCount:     ethos,
		PathosCount:    pathos,
	}

	score := 30.0
	score += logosScore * 15.0
	score += ethosScore * 12.0
	score += pathosScore * 8.0
	score += structureScore * 15.0
	score += clarityScore * 10.0

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

	var models []ModelRhetorical
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail RhetoricalDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.LogosScore += detail.LogosScore
			totalDetail.EthosScore += detail.EthosScore
			totalDetail.PathosScore += detail.PathosScore
			totalDetail.StructureScore += detail.StructureScore
			totalDetail.ClarityScore += detail.ClarityScore
			totalDetail.LogosCount += detail.LogosCount
			totalDetail.EthosCount += detail.EthosCount
			totalDetail.PathosCount += detail.PathosCount

			if detail.LogosCount == 0 && detail.EthosCount == 0 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.LogosScore = math.Round(totalDetail.LogosScore/n*100) / 100
		totalDetail.EthosScore = math.Round(totalDetail.EthosScore/n*100) / 100
		totalDetail.PathosScore = math.Round(totalDetail.PathosScore/n*100) / 100
		totalDetail.StructureScore = math.Round(totalDetail.StructureScore/n*100) / 100
		totalDetail.ClarityScore = math.Round(totalDetail.ClarityScore/n*100) / 100

		models = append(models, ModelRhetorical{
			Model:           mn,
			RhetoricalScore: math.Round(avgScore*10) / 10,
			Grade:           scoreToGrade(avgScore),
			Detail:          totalDetail,
			SampleCount:     len(entries),
			LabCount:        len(labs),
			Concerns:        totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].RhetoricalScore > models[j].RhetoricalScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.RhetoricalScore
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
	b.WriteString("Rhetorical Quality Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No rhetorical data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.RhetoricalScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "logos_score", m.Detail.LogosScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "ethos_score", m.Detail.EthosScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "pathos_score", m.Detail.PathosScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "structure_score", m.Detail.StructureScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "clarity_score", m.Detail.ClarityScore)
		fmt.Fprintf(&b, "    %-28s %d\n", "logos_count", m.Detail.LogosCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "ethos_count", m.Detail.EthosCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "pathos_count", m.Detail.PathosCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
