package adaptability

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

var tonePhrases = []string{
	"formal", "informal", "casual", "professional",
	"technical", "conversational", "academic", "simplified",
	"concise", "detailed", "step-by-step", "brief",
	"friendly", "neutral", "authoritative", "empathetic",
}

var formatSwitchPhrases = []string{
	"bullet", "numbered", "list", "table",
	"code block", "example", "diagram", "summary",
	"paragraph", "heading", "section", "outline",
	"json", "yaml", "markdown", "csv",
}

var audienceAdaptPhrases = []string{
	"beginner", "expert", "non-technical", "developer",
	"executive", "stakeholder", "end user", "administrator",
	"student", "teacher", "researcher", "analyst",
	"layperson", "specialist", "novice", "advanced",
}

var constraintPhrases = []string{
	"constraint", "limitation", "requirement", "within",
	"no more than", "at most", "maximum", "minimum",
	"restricted", "bounded", "limit", "cap",
	"only use", "must include", "avoid", "exclude",
}

var followUpPhrases = []string{
	"as you mentioned", "building on", "to clarify",
	"following up", "as requested", "per your",
	"based on your", "continuing from", "updating",
	"revised", "adjusted", "modified", "refined",
	"incorporating your feedback", "addressing your",
}

var rigidPhrases = []string{
	"i can only", "i cannot", "i am unable",
	"that is not possible", "i do not support",
	"outside my scope", "not within my capabilities",
	"i always respond", "my standard format",
	"regardless of", "irrespective of",
}

type AdaptDetail struct {
	ToneFlexibility       float64 `json:"tone_flexibility"`
	FormatAdaptation      float64 `json:"format_adaptation"`
	AudienceAwareness     float64 `json:"audience_awareness"`
	ConstraintCompliance  float64 `json:"constraint_compliance"`
	FollowUpCoherence     float64 `json:"follow_up_coherence"`
	RigidityCount         int     `json:"rigidity_count"`
	AdaptationSignalCount int     `json:"adaptation_signal_count"`
	ConstraintMissCount   int     `json:"constraint_miss_count"`
}

type ModelAdaptability struct {
	Model            string      `json:"model"`
	AdaptabilityScore float64    `json:"adaptability_score"`
	Grade            string      `json:"grade"`
	Detail           AdaptDetail `json:"detail"`
	SampleCount      int         `json:"sample_count"`
	LabCount         int         `json:"lab_count"`
	Concerns         int         `json:"concerns"`
}

type Report struct {
	Models     []ModelAdaptability `json:"models"`
	BestModel  string              `json:"best_model"`
	WorstModel string              `json:"worst_model"`
	MeanScore  float64             `json:"mean_score"`
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, AdaptDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	toneHits := countPhrases(combined, tonePhrases)
	formatHits := countPhrases(combined, formatSwitchPhrases)
	audienceHits := countPhrases(combined, audienceAdaptPhrases)
	constraintHits := countPhrases(combined, constraintPhrases)
	followUpHits := countPhrases(combined, followUpPhrases)
	rigidHits := countPhrases(combined, rigidPhrases)

	toneScore := 0.2
	if words > 0 {
		toneScore = math.Min(1.0, float64(toneHits)*2.0/float64(words)*10.0+0.2)
	}

	formatScore := 0.2
	if words > 0 {
		formatScore = math.Min(1.0, float64(formatHits)*2.0/float64(words)*10.0+0.2)
	}

	audienceScore := 0.2
	if words > 0 {
		audienceScore = math.Min(1.0, float64(audienceHits)*2.5/float64(words)*10.0+0.2)
	}

	constraintScore := 0.3
	if constraintHits > 0 {
		constraintScore = math.Min(1.0, float64(constraintHits)*0.2+0.3)
	}

	followUpScore := 0.2
	if followUpHits > 0 {
		followUpScore = math.Min(1.0, float64(followUpHits)*0.25+0.2)
	}

	adaptSignals := toneHits + formatHits + audienceHits + followUpHits

	detail := AdaptDetail{
		ToneFlexibility:       math.Round(toneScore*100) / 100,
		FormatAdaptation:      math.Round(formatScore*100) / 100,
		AudienceAwareness:     math.Round(audienceScore*100) / 100,
		ConstraintCompliance:  math.Round(constraintScore*100) / 100,
		FollowUpCoherence:     math.Round(followUpScore*100) / 100,
		RigidityCount:         rigidHits,
		AdaptationSignalCount: adaptSignals,
		ConstraintMissCount:   0,
	}

	score := 30.0
	score += toneScore * 15.0
	score += formatScore * 15.0
	score += audienceScore * 12.0
	score += constraintScore * 10.0
	score += followUpScore * 8.0

	score -= float64(rigidHits) * 4.0
	score += float64(adaptSignals) * 0.5

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

	var models []ModelAdaptability
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail AdaptDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.ToneFlexibility += detail.ToneFlexibility
			totalDetail.FormatAdaptation += detail.FormatAdaptation
			totalDetail.AudienceAwareness += detail.AudienceAwareness
			totalDetail.ConstraintCompliance += detail.ConstraintCompliance
			totalDetail.FollowUpCoherence += detail.FollowUpCoherence
			totalDetail.RigidityCount += detail.RigidityCount
			totalDetail.AdaptationSignalCount += detail.AdaptationSignalCount
			totalDetail.ConstraintMissCount += detail.ConstraintMissCount

			if detail.RigidityCount > 2 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.ToneFlexibility = math.Round(totalDetail.ToneFlexibility/n*100) / 100
		totalDetail.FormatAdaptation = math.Round(totalDetail.FormatAdaptation/n*100) / 100
		totalDetail.AudienceAwareness = math.Round(totalDetail.AudienceAwareness/n*100) / 100
		totalDetail.ConstraintCompliance = math.Round(totalDetail.ConstraintCompliance/n*100) / 100
		totalDetail.FollowUpCoherence = math.Round(totalDetail.FollowUpCoherence/n*100) / 100

		models = append(models, ModelAdaptability{
			Model:             mn,
			AdaptabilityScore: math.Round(avgScore*10) / 10,
			Grade:             scoreToGrade(avgScore),
			Detail:            totalDetail,
			SampleCount:       len(entries),
			LabCount:          len(labs),
			Concerns:          totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].AdaptabilityScore > models[j].AdaptabilityScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.AdaptabilityScore
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
	b.WriteString("Response Adaptability Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No adaptability data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.AdaptabilityScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "tone_flexibility", m.Detail.ToneFlexibility)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "format_adaptation", m.Detail.FormatAdaptation)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "audience_awareness", m.Detail.AudienceAwareness)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "constraint_compliance", m.Detail.ConstraintCompliance)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "follow_up_coherence", m.Detail.FollowUpCoherence)
		fmt.Fprintf(&b, "    %-28s %d\n", "rigidity_count", m.Detail.RigidityCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "adaptation_signals", m.Detail.AdaptationSignalCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
