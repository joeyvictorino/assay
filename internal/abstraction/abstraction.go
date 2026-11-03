package abstraction

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type AbstractionDetail struct {
	LevelAppropriate float64 `json:"level_appropriate"`
	Generalization   float64 `json:"generalization"`
	Concretization   float64 `json:"concretization"`
	LayerAwareness   float64 `json:"layer_awareness"`
	ScopeControl     float64 `json:"scope_control"`
	AbstractCount    int     `json:"abstract_count"`
	ConcreteCount    int     `json:"concrete_count"`
	MixedCount       int     `json:"mixed_count"`
}

type ModelAbstraction struct {
	Model            string            `json:"model"`
	AbstractionScore float64           `json:"abstraction_score"`
	Grade            string            `json:"grade"`
	Detail           AbstractionDetail `json:"detail"`
	SampleCount      int               `json:"sample_count"`
	LabCount         int               `json:"lab_count"`
	Concerns         int               `json:"concerns"`
}

type Report struct {
	Models     []ModelAbstraction `json:"models"`
	BestModel  string             `json:"best_model"`
	WorstModel string             `json:"worst_model"`
	MeanScore  float64            `json:"mean_score"`
}

var highLevelPhrases = []string{
	"at a high level", "the general idea", "in principle",
	"conceptually", "the overall architecture", "from a design perspective",
	"the big picture", "abstractly speaking", "in broad terms",
	"the key abstraction", "the mental model", "the paradigm",
	"the pattern here", "the underlying concept", "fundamentally",
}

var concretePhrases = []string{
	"for example", "specifically", "in practice",
	"concretely", "the actual implementation", "in this specific case",
	"here is the code", "the exact steps", "literally",
	"the concrete type", "the specific instance", "step by step",
	"the actual output", "running this gives", "the precise value",
}

var layerPhrases = []string{
	"at the application layer", "at the transport layer",
	"the interface level", "the implementation level",
	"from the user's perspective", "from the system's perspective",
	"the api surface", "the internal representation",
	"the public contract", "the private implementation",
	"above the abstraction", "below the abstraction",
	"the domain model", "the infrastructure layer", "the presentation layer",
}

var scopePhrases = []string{
	"zooming in", "zooming out", "to be more specific",
	"more broadly", "narrowing down", "widening the scope",
	"drilling into", "stepping back", "at this level",
	"in this context", "within this boundary", "across boundaries",
	"the scope of", "limited to", "extending beyond",
}

var confusionPhrases = []string{
	"it depends on the level", "mixing concerns",
	"conflating", "the abstraction leaks",
	"too abstract", "too concrete",
	"wrong level of detail", "inappropriate abstraction",
	"over-engineered", "under-specified",
	"leaky abstraction", "premature abstraction",
}

var mixedPhrases = []string{
	"on one hand", "on the other hand",
	"both abstract and concrete", "theory and practice",
	"the concept and the implementation", "design and code",
	"interface and implementation", "model and reality",
	"general case and specific case", "rule and exception",
	"pattern and instance", "class and object",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, AbstractionDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	highLevel := countPhrases(combined, highLevelPhrases)
	concrete := countPhrases(combined, concretePhrases)
	layer := countPhrases(combined, layerPhrases)
	scope := countPhrases(combined, scopePhrases)
	confusion := countPhrases(combined, confusionPhrases)
	mixed := countPhrases(combined, mixedPhrases)

	abstractTotal := highLevel + layer
	concreteTotal := concrete

	levelScore := 0.3
	if abstractTotal > 0 && concreteTotal > 0 {
		levelScore = math.Min(1.0, 0.5+float64(abstractTotal+concreteTotal)*0.05)
	} else if abstractTotal > 0 || concreteTotal > 0 {
		levelScore = 0.4
	}

	genScore := 0.2
	if words > 0 {
		genScore = math.Min(1.0, float64(highLevel)*2.0/float64(words)*10.0+0.2)
	}

	concScore := 0.2
	if words > 0 {
		concScore = math.Min(1.0, float64(concrete)*2.0/float64(words)*10.0+0.2)
	}

	layerScore := 0.2
	if layer > 0 {
		layerScore = math.Min(1.0, float64(layer)*0.15+0.2)
	}

	scopeScore := 0.2
	if scope > 0 {
		scopeScore = math.Min(1.0, float64(scope)*0.12+0.2)
	}

	detail := AbstractionDetail{
		LevelAppropriate: math.Round(levelScore*100) / 100,
		Generalization:   math.Round(genScore*100) / 100,
		Concretization:   math.Round(concScore*100) / 100,
		LayerAwareness:   math.Round(layerScore*100) / 100,
		ScopeControl:     math.Round(scopeScore*100) / 100,
		AbstractCount:    abstractTotal,
		ConcreteCount:    concreteTotal,
		MixedCount:       mixed,
	}

	score := 30.0
	score += levelScore * 20.0
	score += genScore * 12.0
	score += concScore * 12.0
	score += layerScore * 10.0
	score += scopeScore * 8.0

	score -= float64(confusion) * 5.0

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

	var models []ModelAbstraction
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail AbstractionDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.LevelAppropriate += detail.LevelAppropriate
			totalDetail.Generalization += detail.Generalization
			totalDetail.Concretization += detail.Concretization
			totalDetail.LayerAwareness += detail.LayerAwareness
			totalDetail.ScopeControl += detail.ScopeControl
			totalDetail.AbstractCount += detail.AbstractCount
			totalDetail.ConcreteCount += detail.ConcreteCount
			totalDetail.MixedCount += detail.MixedCount

			if detail.AbstractCount == 0 && detail.ConcreteCount == 0 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.LevelAppropriate = math.Round(totalDetail.LevelAppropriate/n*100) / 100
		totalDetail.Generalization = math.Round(totalDetail.Generalization/n*100) / 100
		totalDetail.Concretization = math.Round(totalDetail.Concretization/n*100) / 100
		totalDetail.LayerAwareness = math.Round(totalDetail.LayerAwareness/n*100) / 100
		totalDetail.ScopeControl = math.Round(totalDetail.ScopeControl/n*100) / 100

		models = append(models, ModelAbstraction{
			Model:            mn,
			AbstractionScore: math.Round(avgScore*10) / 10,
			Grade:            scoreToGrade(avgScore),
			Detail:           totalDetail,
			SampleCount:      len(entries),
			LabCount:         len(labs),
			Concerns:         totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].AbstractionScore > models[j].AbstractionScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.AbstractionScore
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
	b.WriteString("Abstraction Quality Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No abstraction data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.AbstractionScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "level_appropriate", m.Detail.LevelAppropriate)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "generalization", m.Detail.Generalization)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "concretization", m.Detail.Concretization)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "layer_awareness", m.Detail.LayerAwareness)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "scope_control", m.Detail.ScopeControl)
		fmt.Fprintf(&b, "    %-28s %d\n", "abstract_count", m.Detail.AbstractCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "concrete_count", m.Detail.ConcreteCount)
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
