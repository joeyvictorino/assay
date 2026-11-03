package temporal

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type TemporalDetail struct {
	ChronologicalAccuracy float64 `json:"chronological_accuracy"`
	SequenceUnderstanding float64 `json:"sequence_understanding"`
	DurationEstimation    float64 `json:"duration_estimation"`
	TemporalCausality     float64 `json:"temporal_causality"`
	TimeframeConsistency  float64 `json:"timeframe_consistency"`
	TemporalCount         int     `json:"temporal_count"`
	AnachronismCount      int     `json:"anachronism_count"`
	VagueCount            int     `json:"vague_count"`
}

type ModelTemporal struct {
	Model         string         `json:"model"`
	TemporalScore float64        `json:"temporal_score"`
	Grade         string         `json:"grade"`
	Detail        TemporalDetail `json:"detail"`
	SampleCount   int            `json:"sample_count"`
	LabCount      int            `json:"lab_count"`
	Concerns      int            `json:"concerns"`
}

type Report struct {
	Models     []ModelTemporal `json:"models"`
	BestModel  string          `json:"best_model"`
	WorstModel string          `json:"worst_model"`
	MeanScore  float64         `json:"mean_score"`
}

var chronologicalPhrases = []string{
	"before this", "after that", "previously",
	"subsequently", "in chronological order",
	"first occurred", "then followed",
	"preceded by", "followed by", "prior to",
	"dating back to", "earlier than",
	"in sequence", "the timeline shows", "historically",
}

var sequencePhrases = []string{
	"step by step", "in order", "the first step",
	"the next phase", "sequential", "one after another",
	"the progression", "stage by stage",
	"the workflow begins", "proceeds to",
	"the pipeline", "ordered process",
	"chain of events", "cascade of", "from start to finish",
}

var durationPhrases = []string{
	"lasted for", "took approximately",
	"over a period of", "within the timeframe",
	"the duration was", "spanning",
	"elapsed time", "how long it took",
	"time required", "the interval between",
	"for the duration", "throughout the period",
	"the window of time", "time elapsed", "time span",
}

var causalityPhrases = []string{
	"because of the timing", "the delay caused",
	"as a consequence of the sequence",
	"the temporal relationship", "timing led to",
	"the order matters because", "due to the timing",
	"causally linked in time", "the lag between",
	"triggered by the preceding", "time-dependent",
	"sequential causation", "the timing implies",
	"temporal dependency", "ordered by cause",
}

var anachronismPhrases = []string{
	"at the same time but also before",
	"simultaneously yet preceded",
	"both before and after",
	"contradictory timeline", "temporal inconsistency",
	"time paradox", "anachronistic",
	"out of chronological order",
	"conflicting dates", "impossible sequence",
	"timeline contradiction", "misordered events",
	"retroactively but previously",
	"the dates don't align",
}

var vaguePhrases = []string{
	"at some point", "sometime around",
	"roughly when", "vaguely timed",
	"at an unspecified time", "whenever it happened",
	"around that era", "in that general period",
	"not precisely dated", "the exact timing is unclear",
	"approximately then", "loosely timed",
	"no specific date",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, TemporalDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	chronological := countPhrases(combined, chronologicalPhrases)
	sequence := countPhrases(combined, sequencePhrases)
	duration := countPhrases(combined, durationPhrases)
	causality := countPhrases(combined, causalityPhrases)
	anachronism := countPhrases(combined, anachronismPhrases)
	vague := countPhrases(combined, vaguePhrases)

	temporal := chronological + sequence + duration + causality

	chronoScore := 0.2
	if words > 0 {
		chronoScore = math.Min(1.0, float64(chronological)*2.0/float64(words)*10.0+0.2)
	}

	seqScore := 0.2
	if words > 0 {
		seqScore = math.Min(1.0, float64(sequence)*2.0/float64(words)*10.0+0.2)
	}

	durScore := 0.2
	if duration > 0 || sequence > 0 {
		durScore = math.Min(1.0, float64(duration+sequence)*0.12+0.2)
	}

	causalScore := 0.3
	if words > 0 {
		causalScore = math.Min(1.0, float64(causality)*2.0/float64(words)*10.0+0.3)
	}

	consistencyScore := 0.2
	if anachronism == 0 && vague == 0 {
		consistencyScore = 0.8
	} else if anachronism+vague < 3 {
		consistencyScore = 0.5
	}

	detail := TemporalDetail{
		ChronologicalAccuracy: math.Round(chronoScore*100) / 100,
		SequenceUnderstanding: math.Round(seqScore*100) / 100,
		DurationEstimation:    math.Round(durScore*100) / 100,
		TemporalCausality:     math.Round(causalScore*100) / 100,
		TimeframeConsistency:  math.Round(consistencyScore*100) / 100,
		TemporalCount:         temporal,
		AnachronismCount:      anachronism,
		VagueCount:            vague,
	}

	score := 30.0
	score += chronoScore * 15.0
	score += seqScore * 15.0
	score += durScore * 10.0
	score += causalScore * 10.0
	score += consistencyScore * 10.0

	score -= float64(anachronism) * 3.0
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

	var models []ModelTemporal
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail TemporalDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.ChronologicalAccuracy += detail.ChronologicalAccuracy
			totalDetail.SequenceUnderstanding += detail.SequenceUnderstanding
			totalDetail.DurationEstimation += detail.DurationEstimation
			totalDetail.TemporalCausality += detail.TemporalCausality
			totalDetail.TimeframeConsistency += detail.TimeframeConsistency
			totalDetail.TemporalCount += detail.TemporalCount
			totalDetail.AnachronismCount += detail.AnachronismCount
			totalDetail.VagueCount += detail.VagueCount

			if detail.AnachronismCount > 3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.ChronologicalAccuracy = math.Round(totalDetail.ChronologicalAccuracy/n*100) / 100
		totalDetail.SequenceUnderstanding = math.Round(totalDetail.SequenceUnderstanding/n*100) / 100
		totalDetail.DurationEstimation = math.Round(totalDetail.DurationEstimation/n*100) / 100
		totalDetail.TemporalCausality = math.Round(totalDetail.TemporalCausality/n*100) / 100
		totalDetail.TimeframeConsistency = math.Round(totalDetail.TimeframeConsistency/n*100) / 100

		models = append(models, ModelTemporal{
			Model:         mn,
			TemporalScore: math.Round(avgScore*10) / 10,
			Grade:         scoreToGrade(avgScore),
			Detail:        totalDetail,
			SampleCount:   len(entries),
			LabCount:      len(labs),
			Concerns:      totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].TemporalScore > models[j].TemporalScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.TemporalScore
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
	b.WriteString("Temporal Reasoning Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No temporal reasoning data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.TemporalScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "chronological_accuracy", m.Detail.ChronologicalAccuracy)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "sequence_understanding", m.Detail.SequenceUnderstanding)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "duration_estimation", m.Detail.DurationEstimation)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "temporal_causality", m.Detail.TemporalCausality)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "timeframe_consistency", m.Detail.TimeframeConsistency)
		fmt.Fprintf(&b, "    %-28s %d\n", "temporal_count", m.Detail.TemporalCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "anachronism_count", m.Detail.AnachronismCount)
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
