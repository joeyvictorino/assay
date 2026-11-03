package persuasion

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type PersuasionDetail struct {
	ArgumentStrength float64 `json:"argument_strength"`
	EvidenceUsage    float64 `json:"evidence_usage"`
	LogicalFlow      float64 `json:"logical_flow"`
	CallToAction     float64 `json:"call_to_action"`
	CredibilityBuild float64 `json:"credibility_build"`
	EvidenceCount    int     `json:"evidence_count"`
	FallacyCount     int     `json:"fallacy_count"`
	TransitionCount  int     `json:"transition_count"`
}

type ModelPersuasion struct {
	Model           string           `json:"model"`
	PersuasionScore float64          `json:"persuasion_score"`
	Grade           string           `json:"grade"`
	Detail          PersuasionDetail `json:"detail"`
	SampleCount     int              `json:"sample_count"`
	LabCount        int              `json:"lab_count"`
	Concerns        int              `json:"concerns"`
}

type Report struct {
	Models     []ModelPersuasion `json:"models"`
	BestModel  string            `json:"best_model"`
	WorstModel string            `json:"worst_model"`
	MeanScore  float64           `json:"mean_score"`
}

var argumentPhrases = []string{
	"because", "therefore", "consequently",
	"as a result", "for this reason", "thus",
	"hence", "it follows that", "given that",
	"since", "due to", "owing to",
	"the reason is", "this demonstrates",
	"this shows that", "this proves",
}

var evidencePhrases = []string{
	"according to", "research shows", "studies indicate",
	"data suggests", "evidence shows", "for example",
	"for instance", "such as", "specifically",
	"in particular", "notably", "a case in point",
	"statistics show", "the data indicates",
	"based on", "supported by",
}

var logicalPhrases = []string{
	"first", "second", "third",
	"firstly", "secondly", "thirdly",
	"in addition", "furthermore", "moreover",
	"additionally", "also", "next",
	"finally", "in conclusion", "to summarize",
	"on balance", "overall", "in summary",
}

var ctaPhrases = []string{
	"i recommend", "you should", "consider",
	"i suggest", "the best approach", "try",
	"start by", "begin with", "take action",
	"implement", "adopt", "use",
	"go with", "opt for", "choose",
}

var credibilityPhrases = []string{
	"in my experience", "experts agree",
	"best practice", "industry standard",
	"widely accepted", "well-established",
	"proven approach", "time-tested",
	"recognized", "authoritative",
	"peer-reviewed", "well-documented",
}

var fallacyPhrases = []string{
	"everyone knows", "obviously",
	"it goes without saying", "clearly",
	"no one would", "any reasonable person",
	"common sense", "self-evident",
	"needless to say", "of course everyone",
	"it's just natural", "that's just how",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, PersuasionDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	args := countPhrases(combined, argumentPhrases)
	evid := countPhrases(combined, evidencePhrases)
	logic := countPhrases(combined, logicalPhrases)
	cta := countPhrases(combined, ctaPhrases)
	cred := countPhrases(combined, credibilityPhrases)
	fallacy := countPhrases(combined, fallacyPhrases)

	argScore := 0.2
	if words > 0 {
		argScore = math.Min(1.0, float64(args)*2.0/float64(words)*10.0+0.2)
	}

	evidScore := 0.2
	if words > 0 {
		evidScore = math.Min(1.0, float64(evid)*2.0/float64(words)*10.0+0.2)
	}

	logicScore := 0.2
	if words > 0 {
		logicScore = math.Min(1.0, float64(logic)*2.0/float64(words)*10.0+0.2)
	}

	ctaScore := 0.3
	if cta > 0 {
		ctaScore = math.Min(1.0, float64(cta)*0.2+0.3)
	}

	credScore := 0.2
	if cred > 0 {
		credScore = math.Min(1.0, float64(cred)*0.25+0.2)
	}

	detail := PersuasionDetail{
		ArgumentStrength: math.Round(argScore*100) / 100,
		EvidenceUsage:    math.Round(evidScore*100) / 100,
		LogicalFlow:      math.Round(logicScore*100) / 100,
		CallToAction:     math.Round(ctaScore*100) / 100,
		CredibilityBuild: math.Round(credScore*100) / 100,
		EvidenceCount:    evid,
		FallacyCount:     fallacy,
		TransitionCount:  logic,
	}

	score := 30.0
	score += argScore * 15.0
	score += evidScore * 15.0
	score += logicScore * 15.0
	score += ctaScore * 10.0
	score += credScore * 10.0

	score -= float64(fallacy) * 4.0

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

	var models []ModelPersuasion
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail PersuasionDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.ArgumentStrength += detail.ArgumentStrength
			totalDetail.EvidenceUsage += detail.EvidenceUsage
			totalDetail.LogicalFlow += detail.LogicalFlow
			totalDetail.CallToAction += detail.CallToAction
			totalDetail.CredibilityBuild += detail.CredibilityBuild
			totalDetail.EvidenceCount += detail.EvidenceCount
			totalDetail.FallacyCount += detail.FallacyCount
			totalDetail.TransitionCount += detail.TransitionCount

			if detail.FallacyCount > 2 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.ArgumentStrength = math.Round(totalDetail.ArgumentStrength/n*100) / 100
		totalDetail.EvidenceUsage = math.Round(totalDetail.EvidenceUsage/n*100) / 100
		totalDetail.LogicalFlow = math.Round(totalDetail.LogicalFlow/n*100) / 100
		totalDetail.CallToAction = math.Round(totalDetail.CallToAction/n*100) / 100
		totalDetail.CredibilityBuild = math.Round(totalDetail.CredibilityBuild/n*100) / 100

		models = append(models, ModelPersuasion{
			Model:           mn,
			PersuasionScore: math.Round(avgScore*10) / 10,
			Grade:           scoreToGrade(avgScore),
			Detail:          totalDetail,
			SampleCount:     len(entries),
			LabCount:        len(labs),
			Concerns:        totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].PersuasionScore > models[j].PersuasionScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.PersuasionScore
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
	b.WriteString("Response Persuasion Quality Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No persuasion data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.PersuasionScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "argument_strength", m.Detail.ArgumentStrength)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "evidence_usage", m.Detail.EvidenceUsage)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "logical_flow", m.Detail.LogicalFlow)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "call_to_action", m.Detail.CallToAction)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "credibility_build", m.Detail.CredibilityBuild)
		fmt.Fprintf(&b, "    %-28s %d\n", "evidence_count", m.Detail.EvidenceCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "fallacy_count", m.Detail.FallacyCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "transition_count", m.Detail.TransitionCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
