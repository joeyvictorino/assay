package pragmatism

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type PragmatismDetail struct {
	ActionabilityScore float64 `json:"actionability_score"`
	ConcreteSteps      float64 `json:"concrete_steps"`
	ImplementReady     float64 `json:"implement_ready"`
	RealWorldFit       float64 `json:"real_world_fit"`
	ConstraintAware    float64 `json:"constraint_aware"`
	ActionVerbCount    int     `json:"action_verb_count"`
	VagueCount         int     `json:"vague_count"`
	ExampleCount       int     `json:"example_count"`
}

type ModelPragmatism struct {
	Model           string           `json:"model"`
	PragmatismScore float64          `json:"pragmatism_score"`
	Grade           string           `json:"grade"`
	Detail          PragmatismDetail `json:"detail"`
	SampleCount     int              `json:"sample_count"`
	LabCount        int              `json:"lab_count"`
	Concerns        int              `json:"concerns"`
}

type Report struct {
	Models     []ModelPragmatism `json:"models"`
	BestModel  string            `json:"best_model"`
	WorstModel string            `json:"worst_model"`
	MeanScore  float64           `json:"mean_score"`
}

var actionPhrases = []string{
	"run", "execute", "install", "configure",
	"create", "set up", "deploy", "implement",
	"add", "remove", "update", "modify",
	"open", "navigate to", "click", "type",
	"copy", "paste", "download", "upload",
}

var concretePhrases = []string{
	"step 1", "step 2", "step 3",
	"first,", "second,", "third,",
	"then", "next", "after that",
	"finally", "once complete", "when done",
	"here is how", "to do this", "follow these",
}

var implementPhrases = []string{
	"code example", "snippet", "sample code",
	"template", "boilerplate", "starter",
	"command", "script", "config file",
	"the following code", "paste this",
	"save as", "name it", "in the file",
}

var realWorldPhrases = []string{
	"in practice", "in production", "real-world",
	"typical use case", "common scenario",
	"for example", "such as", "like when",
	"a practical approach", "hands-on",
	"working solution", "ready to use",
}

var constraintPhrases = []string{
	"keep in mind", "note that", "be aware",
	"limitation", "caveat", "trade-off",
	"depending on your", "if you have",
	"make sure", "prerequisite", "requirement",
	"before you start", "you'll need",
}

var vaguePhrases = []string{
	"it depends", "there are many ways",
	"you could try", "one option might be",
	"it's possible", "there are various",
	"some people", "in some cases",
	"it varies", "that's subjective",
	"hard to say", "no clear answer",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, PragmatismDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	actions := countPhrases(combined, actionPhrases)
	concrete := countPhrases(combined, concretePhrases)
	impl := countPhrases(combined, implementPhrases)
	realW := countPhrases(combined, realWorldPhrases)
	constrt := countPhrases(combined, constraintPhrases)
	vague := countPhrases(combined, vaguePhrases)

	actScore := 0.2
	if words > 0 {
		actScore = math.Min(1.0, float64(actions)*2.0/float64(words)*10.0+0.2)
	}

	concreteScore := 0.2
	if words > 0 {
		concreteScore = math.Min(1.0, float64(concrete)*2.0/float64(words)*10.0+0.2)
	}

	implScore := 0.2
	if impl > 0 {
		implScore = math.Min(1.0, float64(impl)*0.25+0.2)
	}

	realScore := 0.3
	if realW > 0 {
		realScore = math.Min(1.0, float64(realW)*0.2+0.3)
	}

	constScore := 0.3
	if constrt > 0 {
		constScore = math.Min(1.0, float64(constrt)*0.2+0.3)
	}

	examples := actions + concrete

	detail := PragmatismDetail{
		ActionabilityScore: math.Round(actScore*100) / 100,
		ConcreteSteps:      math.Round(concreteScore*100) / 100,
		ImplementReady:     math.Round(implScore*100) / 100,
		RealWorldFit:       math.Round(realScore*100) / 100,
		ConstraintAware:    math.Round(constScore*100) / 100,
		ActionVerbCount:    actions,
		VagueCount:         vague,
		ExampleCount:       examples,
	}

	score := 30.0
	score += actScore * 15.0
	score += concreteScore * 15.0
	score += implScore * 15.0
	score += realScore * 10.0
	score += constScore * 10.0

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

	var models []ModelPragmatism
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail PragmatismDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.ActionabilityScore += detail.ActionabilityScore
			totalDetail.ConcreteSteps += detail.ConcreteSteps
			totalDetail.ImplementReady += detail.ImplementReady
			totalDetail.RealWorldFit += detail.RealWorldFit
			totalDetail.ConstraintAware += detail.ConstraintAware
			totalDetail.ActionVerbCount += detail.ActionVerbCount
			totalDetail.VagueCount += detail.VagueCount
			totalDetail.ExampleCount += detail.ExampleCount

			if detail.VagueCount > 3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.ActionabilityScore = math.Round(totalDetail.ActionabilityScore/n*100) / 100
		totalDetail.ConcreteSteps = math.Round(totalDetail.ConcreteSteps/n*100) / 100
		totalDetail.ImplementReady = math.Round(totalDetail.ImplementReady/n*100) / 100
		totalDetail.RealWorldFit = math.Round(totalDetail.RealWorldFit/n*100) / 100
		totalDetail.ConstraintAware = math.Round(totalDetail.ConstraintAware/n*100) / 100

		models = append(models, ModelPragmatism{
			Model:           mn,
			PragmatismScore: math.Round(avgScore*10) / 10,
			Grade:           scoreToGrade(avgScore),
			Detail:          totalDetail,
			SampleCount:     len(entries),
			LabCount:        len(labs),
			Concerns:        totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].PragmatismScore > models[j].PragmatismScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.PragmatismScore
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
	b.WriteString("Response Pragmatism and Actionability Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No pragmatism data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.PragmatismScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "actionability_score", m.Detail.ActionabilityScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "concrete_steps", m.Detail.ConcreteSteps)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "implement_ready", m.Detail.ImplementReady)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "real_world_fit", m.Detail.RealWorldFit)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "constraint_aware", m.Detail.ConstraintAware)
		fmt.Fprintf(&b, "    %-28s %d\n", "action_verb_count", m.Detail.ActionVerbCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "vague_count", m.Detail.VagueCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "example_count", m.Detail.ExampleCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
