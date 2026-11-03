package taskfollow

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type FollowDetail struct {
	InstructionCoverage float64 `json:"instruction_coverage"`
	ConstraintAdherence float64 `json:"constraint_adherence"`
	FormatCompliance    float64 `json:"format_compliance"`
	StepOrdering        float64 `json:"step_ordering"`
	OutputCompleteness  float64 `json:"output_completeness"`
	InstructionsFound   int     `json:"instructions_found"`
	ConstraintsMet      int     `json:"constraints_met"`
	FormatErrors        int     `json:"format_errors"`
}

type ModelFollow struct {
	Model       string       `json:"model"`
	FollowScore float64      `json:"follow_score"`
	Grade       string       `json:"grade"`
	Detail      FollowDetail `json:"detail"`
	SampleCount int          `json:"sample_count"`
	LabCount    int          `json:"lab_count"`
	Deviations  int          `json:"deviations"`
}

type Report struct {
	Models     []ModelFollow `json:"models"`
	BestModel  string        `json:"best_model"`
	WorstModel string        `json:"worst_model"`
	MeanScore  float64       `json:"mean_score"`
}

var instructionPhrases = []string{
	"as requested", "as you asked", "per your instructions",
	"following your", "according to your", "as specified",
	"as directed", "you mentioned", "per your request",
	"based on your requirements", "as outlined",
	"as described", "you asked for", "your request",
}

var constraintPhrases = []string{
	"within the limit", "no more than", "at most",
	"keeping it under", "limited to", "constrained to",
	"restricted to", "up to", "maximum of",
	"not exceeding", "capped at", "bounded by",
}

var formatPhrases = []string{
	"in the format", "formatted as", "structured as",
	"using the template", "in json", "in markdown",
	"as a list", "as a table", "bulleted",
	"numbered", "in csv", "in yaml",
	"code block", "heading", "paragraph",
}

var stepPhrases = []string{
	"first", "second", "third", "then", "next",
	"after that", "finally", "step 1", "step 2",
	"step 3", "following step", "subsequently",
	"to begin", "moving on", "lastly",
}

var completenessPhrases = []string{
	"in summary", "to summarize", "in conclusion",
	"covering all", "addressing each", "all points",
	"comprehensive", "thorough", "complete",
	"everything you asked", "all requirements",
	"nothing was missed", "fully addressed",
}

var deviationPhrases = []string{
	"instead of", "rather than", "i deviated",
	"i changed", "i modified the approach",
	"not exactly what you asked", "slight variation",
	"i took a different", "alternative approach",
	"i adjusted", "i adapted",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, FollowDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	instrs := countPhrases(combined, instructionPhrases)
	constr := countPhrases(combined, constraintPhrases)
	fmtHit := countPhrases(combined, formatPhrases)
	steps := countPhrases(combined, stepPhrases)
	compl := countPhrases(combined, completenessPhrases)
	deviat := countPhrases(combined, deviationPhrases)

	instrScore := 0.3
	if words > 0 {
		instrScore = math.Min(1.0, float64(instrs)*2.5/float64(words)*10.0+0.3)
	}

	constraintScore := 0.4
	if constr > 0 {
		constraintScore = math.Min(1.0, float64(constr)*0.2+0.4)
	}

	formatScore := 0.4
	if fmtHit > 0 {
		formatScore = math.Min(1.0, float64(fmtHit)*0.15+0.4)
	}

	stepScore := 0.3
	if steps > 0 {
		stepScore = math.Min(1.0, float64(steps)*0.12+0.3)
	}

	complScore := 0.3
	if compl > 0 {
		complScore = math.Min(1.0, float64(compl)*0.2+0.3)
	}

	fmtErrors := deviat

	detail := FollowDetail{
		InstructionCoverage: math.Round(instrScore*100) / 100,
		ConstraintAdherence: math.Round(constraintScore*100) / 100,
		FormatCompliance:    math.Round(formatScore*100) / 100,
		StepOrdering:        math.Round(stepScore*100) / 100,
		OutputCompleteness:  math.Round(complScore*100) / 100,
		InstructionsFound:   instrs + constr,
		ConstraintsMet:      constr + fmtHit,
		FormatErrors:        fmtErrors,
	}

	score := 45.0
	score += instrScore * 12.0
	score += constraintScore * 10.0
	score += formatScore * 10.0
	score += stepScore * 8.0
	score += complScore * 10.0

	score -= float64(deviat) * 3.0

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

	var models []ModelFollow
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail FollowDetail
		totalDeviations := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.InstructionCoverage += detail.InstructionCoverage
			totalDetail.ConstraintAdherence += detail.ConstraintAdherence
			totalDetail.FormatCompliance += detail.FormatCompliance
			totalDetail.StepOrdering += detail.StepOrdering
			totalDetail.OutputCompleteness += detail.OutputCompleteness
			totalDetail.InstructionsFound += detail.InstructionsFound
			totalDetail.ConstraintsMet += detail.ConstraintsMet
			totalDetail.FormatErrors += detail.FormatErrors

			if detail.FormatErrors > 0 {
				totalDeviations++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.InstructionCoverage = math.Round(totalDetail.InstructionCoverage/n*100) / 100
		totalDetail.ConstraintAdherence = math.Round(totalDetail.ConstraintAdherence/n*100) / 100
		totalDetail.FormatCompliance = math.Round(totalDetail.FormatCompliance/n*100) / 100
		totalDetail.StepOrdering = math.Round(totalDetail.StepOrdering/n*100) / 100
		totalDetail.OutputCompleteness = math.Round(totalDetail.OutputCompleteness/n*100) / 100

		models = append(models, ModelFollow{
			Model:       mn,
			FollowScore: math.Round(avgScore*10) / 10,
			Grade:       scoreToGrade(avgScore),
			Detail:      totalDetail,
			SampleCount: len(entries),
			LabCount:    len(labs),
			Deviations:  totalDeviations,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].FollowScore > models[j].FollowScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.FollowScore
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
	b.WriteString("Task Following and Instruction Compliance Analysis\n")
	b.WriteString(strings.Repeat("=", 55) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No task following data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.FollowScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "instruction_coverage", m.Detail.InstructionCoverage)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "constraint_adherence", m.Detail.ConstraintAdherence)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "format_compliance", m.Detail.FormatCompliance)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "step_ordering", m.Detail.StepOrdering)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "output_completeness", m.Detail.OutputCompleteness)
		fmt.Fprintf(&b, "    %-28s %d\n", "instructions_found", m.Detail.InstructionsFound)
		fmt.Fprintf(&b, "    %-28s %d\n", "constraints_met", m.Detail.ConstraintsMet)
		fmt.Fprintf(&b, "    %-28s %d\n", "format_errors", m.Detail.FormatErrors)
		fmt.Fprintf(&b, "    Deviations: %d\n", m.Deviations)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
