package hypothetical

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type HypotheticalDetail struct {
	ScenarioConstruction  float64 `json:"scenario_construction"`
	CounterfactualDepth   float64 `json:"counterfactual_depth"`
	ConstraintHandling    float64 `json:"constraint_handling"`
	CreativeExtrapolation float64 `json:"creative_extrapolation"`
	LogicalConsistency    float64 `json:"logical_consistency"`
	HypotheticalCount     int     `json:"hypothetical_count"`
	WeakReasoningCount    int     `json:"weak_reasoning_count"`
	VagueCount            int     `json:"vague_count"`
}

type ModelHypothetical struct {
	Model             string              `json:"model"`
	HypotheticalScore float64             `json:"hypothetical_score"`
	Grade             string              `json:"grade"`
	Detail            HypotheticalDetail  `json:"detail"`
	SampleCount       int                 `json:"sample_count"`
	LabCount          int                 `json:"lab_count"`
	Concerns          int                 `json:"concerns"`
}

type Report struct {
	Models     []ModelHypothetical `json:"models"`
	BestModel  string              `json:"best_model"`
	WorstModel string              `json:"worst_model"`
	MeanScore  float64             `json:"mean_score"`
}

var scenarioPhrases = []string{
	"imagine if", "suppose that", "consider a scenario",
	"what if", "in a hypothetical", "let us assume",
	"for the sake of argument", "picture this",
	"envision a situation", "hypothetically speaking",
	"thought experiment", "if we were to",
	"in theory", "a possible scenario",
}

var counterfactualPhrases = []string{
	"had this been", "if instead", "alternatively",
	"were it not for", "in the absence of",
	"without this constraint", "under different conditions",
	"if the opposite were true", "reversing the assumption",
	"contrary to", "in contrast to the actual",
	"had we chosen differently", "if this had not occurred",
}

var constraintPhrases = []string{
	"given the constraint", "within the bounds of",
	"subject to the limitation", "assuming the restriction",
	"under the condition that", "provided that",
	"with the caveat", "bounded by",
	"constrained to", "limited by the requirement",
	"respecting the boundary", "factoring in the restriction",
}

var speculationPhrases = []string{
	"this could lead to", "one possibility is",
	"extrapolating from this", "projecting forward",
	"this implies that", "by extension",
	"taking this further", "the logical consequence",
	"following this thread", "building on this premise",
	"the downstream effect", "cascading from this",
	"a plausible outcome", "this opens the door to",
}

var logicalPhrases = []string{
	"therefore", "consequently", "it follows that",
	"this entails", "by logical extension",
	"the necessary conclusion", "deductively",
	"from this premise", "given these axioms",
	"the inference is", "this validates",
	"consistent with", "logically coherent",
	"this contradicts", "the implication holds",
}

var weakReasoningPhrases = []string{
	"i'm not sure what would happen",
	"it's hard to say", "anything could happen",
	"impossible to predict", "who knows",
	"your guess is as good as mine",
	"there's no way to tell",
	"i can't really speculate",
	"that's beyond prediction",
	"too many variables",
	"completely unpredictable",
}

var vaguePhrases = []string{
	"something might happen", "things could change",
	"it depends on many factors", "hard to say exactly",
	"results may vary", "it's complicated",
	"there are many possibilities", "anything is possible",
	"it could go either way", "not entirely clear",
	"somewhat uncertain", "roughly speaking",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, HypotheticalDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	scenario := countPhrases(combined, scenarioPhrases)
	counterfactual := countPhrases(combined, counterfactualPhrases)
	constraint := countPhrases(combined, constraintPhrases)
	speculation := countPhrases(combined, speculationPhrases)
	logical := countPhrases(combined, logicalPhrases)
	weak := countPhrases(combined, weakReasoningPhrases)
	vague := countPhrases(combined, vaguePhrases)

	hypothetical := scenario + counterfactual + constraint + speculation + logical

	scenarioScore := 0.2
	if words > 0 {
		scenarioScore = math.Min(1.0, float64(scenario)*2.0/float64(words)*10.0+0.2)
	}

	counterfactualScore := 0.2
	if words > 0 {
		counterfactualScore = math.Min(1.0, float64(counterfactual)*2.0/float64(words)*10.0+0.2)
	}

	constraintScore := 0.3
	if constraint > 0 || logical > 0 {
		constraintScore = math.Min(1.0, float64(constraint+logical)*0.15+0.3)
	}

	extrapolationScore := 0.2
	if words > 0 {
		extrapolationScore = math.Min(1.0, float64(speculation)*2.0/float64(words)*10.0+0.2)
	}

	consistencyScore := 0.2
	if weak == 0 && vague == 0 {
		consistencyScore = 0.8
	} else if weak+vague < 3 {
		consistencyScore = 0.5
	}

	detail := HypotheticalDetail{
		ScenarioConstruction:  math.Round(scenarioScore*100) / 100,
		CounterfactualDepth:   math.Round(counterfactualScore*100) / 100,
		ConstraintHandling:    math.Round(constraintScore*100) / 100,
		CreativeExtrapolation: math.Round(extrapolationScore*100) / 100,
		LogicalConsistency:    math.Round(consistencyScore*100) / 100,
		HypotheticalCount:     hypothetical,
		WeakReasoningCount:    weak,
		VagueCount:            vague,
	}

	score := 30.0
	score += scenarioScore * 15.0
	score += counterfactualScore * 15.0
	score += constraintScore * 10.0
	score += extrapolationScore * 10.0
	score += consistencyScore * 10.0

	score -= float64(weak) * 4.0
	score -= float64(vague) * 5.0

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

	var models []ModelHypothetical
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail HypotheticalDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.ScenarioConstruction += detail.ScenarioConstruction
			totalDetail.CounterfactualDepth += detail.CounterfactualDepth
			totalDetail.ConstraintHandling += detail.ConstraintHandling
			totalDetail.CreativeExtrapolation += detail.CreativeExtrapolation
			totalDetail.LogicalConsistency += detail.LogicalConsistency
			totalDetail.HypotheticalCount += detail.HypotheticalCount
			totalDetail.WeakReasoningCount += detail.WeakReasoningCount
			totalDetail.VagueCount += detail.VagueCount

			if detail.WeakReasoningCount > 3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.ScenarioConstruction = math.Round(totalDetail.ScenarioConstruction/n*100) / 100
		totalDetail.CounterfactualDepth = math.Round(totalDetail.CounterfactualDepth/n*100) / 100
		totalDetail.ConstraintHandling = math.Round(totalDetail.ConstraintHandling/n*100) / 100
		totalDetail.CreativeExtrapolation = math.Round(totalDetail.CreativeExtrapolation/n*100) / 100
		totalDetail.LogicalConsistency = math.Round(totalDetail.LogicalConsistency/n*100) / 100

		models = append(models, ModelHypothetical{
			Model:             mn,
			HypotheticalScore: math.Round(avgScore*10) / 10,
			Grade:             scoreToGrade(avgScore),
			Detail:            totalDetail,
			SampleCount:       len(entries),
			LabCount:          len(labs),
			Concerns:          totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].HypotheticalScore > models[j].HypotheticalScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.HypotheticalScore
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
	b.WriteString("Hypothetical Reasoning Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No hypothetical reasoning data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.HypotheticalScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "scenario_construction", m.Detail.ScenarioConstruction)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "counterfactual_depth", m.Detail.CounterfactualDepth)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "constraint_handling", m.Detail.ConstraintHandling)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "creative_extrapolation", m.Detail.CreativeExtrapolation)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "logical_consistency", m.Detail.LogicalConsistency)
		fmt.Fprintf(&b, "    %-28s %d\n", "hypothetical_count", m.Detail.HypotheticalCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "weak_reasoning_count", m.Detail.WeakReasoningCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "vague_count", m.Detail.VagueCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
