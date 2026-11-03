package prioritization

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type PrioritizationDetail struct {
	CriticalFirst    float64 `json:"critical_first"`
	RiskWeighting    float64 `json:"risk_weighting"`
	ActionableOrder  float64 `json:"actionable_order"`
	SeverityClarity  float64 `json:"severity_clarity"`
	TriageAccuracy   float64 `json:"triage_accuracy"`
	PriorityCount    int     `json:"priority_count"`
	ScatteredCount   int     `json:"scattered_count"`
	VagueCount       int     `json:"vague_count"`
}

type ModelPrioritization struct {
	Model               string               `json:"model"`
	PrioritizationScore float64              `json:"prioritization_score"`
	Grade               string               `json:"grade"`
	Detail              PrioritizationDetail `json:"detail"`
	SampleCount         int                  `json:"sample_count"`
	LabCount            int                  `json:"lab_count"`
	Concerns            int                  `json:"concerns"`
}

type Report struct {
	Models     []ModelPrioritization `json:"models"`
	BestModel  string                `json:"best_model"`
	WorstModel string                `json:"worst_model"`
	MeanScore  float64               `json:"mean_score"`
}

var criticalPhrases = []string{
	"critical", "highest priority", "must fix immediately",
	"severity critical", "p0", "blocking issue",
	"top priority", "urgent", "emergency",
	"most important first", "address immediately",
	"before anything else", "highest risk",
	"the primary concern", "number one issue",
}

var riskPhrases = []string{
	"high risk", "medium risk", "low risk",
	"risk level", "severity", "impact level",
	"likelihood", "exposure", "threat level",
	"risk score", "cvss", "risk rating",
	"risk assessment", "vulnerability severity",
	"attack surface", "blast radius",
}

var actionablePhrases = []string{
	"step one", "first action", "next step",
	"recommended fix", "remediation", "mitigation",
	"action item", "to resolve", "workaround",
	"patch available", "upgrade to", "apply fix",
	"disable the", "configure the", "implement",
	"the fix is", "to address this",
}

var severityPhrases = []string{
	"classified as", "rated as", "scored at",
	"severity level", "priority level", "tier",
	"categorized as", "marked as", "flagged as",
	"critical finding", "high finding", "medium finding",
	"low finding", "informational", "ranked by severity",
}

var scatteredPhrases = []string{
	"also worth noting", "by the way",
	"another thing", "on a different note",
	"unrelated but", "tangentially",
	"while we are at it", "incidentally",
	"as a side note", "off topic but",
	"not directly related", "separately",
}

var vaguePhrases = []string{
	"something might be wrong", "could be an issue",
	"not sure how important", "maybe look into",
	"possibly a concern", "it depends on context",
	"hard to say", "unclear severity",
	"needs more investigation", "further analysis needed",
	"tbd", "to be determined",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, PrioritizationDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	critical := countPhrases(combined, criticalPhrases)
	risk := countPhrases(combined, riskPhrases)
	actionable := countPhrases(combined, actionablePhrases)
	severity := countPhrases(combined, severityPhrases)
	scattered := countPhrases(combined, scatteredPhrases)
	vague := countPhrases(combined, vaguePhrases)

	priorityTotal := critical + risk + actionable + severity

	criticalScore := 0.2
	if words > 0 {
		criticalScore = math.Min(1.0, float64(critical)*2.0/float64(words)*10.0+0.2)
	}

	riskScore := 0.2
	if words > 0 {
		riskScore = math.Min(1.0, float64(risk)*2.0/float64(words)*10.0+0.2)
	}

	actionableScore := 0.2
	if words > 0 {
		actionableScore = math.Min(1.0, float64(actionable)*2.0/float64(words)*10.0+0.2)
	}

	severityScore := 0.3
	if severity > 0 || critical > 0 {
		severityScore = math.Min(1.0, float64(severity+critical)*0.15+0.3)
	}

	triageScore := 0.2
	if scattered == 0 && vague == 0 {
		triageScore = 0.8
	} else if scattered+vague < 3 {
		triageScore = 0.5
	}

	detail := PrioritizationDetail{
		CriticalFirst:   math.Round(criticalScore*100) / 100,
		RiskWeighting:   math.Round(riskScore*100) / 100,
		ActionableOrder: math.Round(actionableScore*100) / 100,
		SeverityClarity: math.Round(severityScore*100) / 100,
		TriageAccuracy:  math.Round(triageScore*100) / 100,
		PriorityCount:   priorityTotal,
		ScatteredCount:  scattered,
		VagueCount:      vague,
	}

	score := 30.0
	score += criticalScore * 15.0
	score += riskScore * 15.0
	score += actionableScore * 10.0
	score += severityScore * 10.0
	score += triageScore * 10.0

	score -= float64(scattered) * 4.0
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

	var models []ModelPrioritization
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail PrioritizationDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.CriticalFirst += detail.CriticalFirst
			totalDetail.RiskWeighting += detail.RiskWeighting
			totalDetail.ActionableOrder += detail.ActionableOrder
			totalDetail.SeverityClarity += detail.SeverityClarity
			totalDetail.TriageAccuracy += detail.TriageAccuracy
			totalDetail.PriorityCount += detail.PriorityCount
			totalDetail.ScatteredCount += detail.ScatteredCount
			totalDetail.VagueCount += detail.VagueCount

			if detail.ScatteredCount > 3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.CriticalFirst = math.Round(totalDetail.CriticalFirst/n*100) / 100
		totalDetail.RiskWeighting = math.Round(totalDetail.RiskWeighting/n*100) / 100
		totalDetail.ActionableOrder = math.Round(totalDetail.ActionableOrder/n*100) / 100
		totalDetail.SeverityClarity = math.Round(totalDetail.SeverityClarity/n*100) / 100
		totalDetail.TriageAccuracy = math.Round(totalDetail.TriageAccuracy/n*100) / 100

		models = append(models, ModelPrioritization{
			Model:               mn,
			PrioritizationScore: math.Round(avgScore*10) / 10,
			Grade:               scoreToGrade(avgScore),
			Detail:              totalDetail,
			SampleCount:         len(entries),
			LabCount:            len(labs),
			Concerns:            totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].PrioritizationScore > models[j].PrioritizationScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.PrioritizationScore
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
	b.WriteString("Response Prioritization Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No prioritization data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.PrioritizationScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "critical_first", m.Detail.CriticalFirst)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "risk_weighting", m.Detail.RiskWeighting)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "actionable_order", m.Detail.ActionableOrder)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "severity_clarity", m.Detail.SeverityClarity)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "triage_accuracy", m.Detail.TriageAccuracy)
		fmt.Fprintf(&b, "    %-28s %d\n", "priority_count", m.Detail.PriorityCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "scattered_count", m.Detail.ScatteredCount)
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
