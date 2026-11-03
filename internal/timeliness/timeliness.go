package timeliness

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type TimelinessDetail struct {
	ResponseSpeed    float64 `json:"response_speed"`
	ContextFreshness float64 `json:"context_freshness"`
	TemporalAccuracy float64 `json:"temporal_accuracy"`
	UpdateAwareness  float64 `json:"update_awareness"`
	DecayResistance  float64 `json:"decay_resistance"`
	TimelyCount      int     `json:"timely_count"`
	StaleCount       int     `json:"stale_count"`
	AmbiguousCount   int     `json:"ambiguous_count"`
}

type ModelTimeliness struct {
	Model          string           `json:"model"`
	TimelinessScore float64         `json:"timeliness_score"`
	Grade          string           `json:"grade"`
	Detail         TimelinessDetail `json:"detail"`
	SampleCount    int              `json:"sample_count"`
	LabCount       int              `json:"lab_count"`
	Concerns       int              `json:"concerns"`
}

type Report struct {
	Models     []ModelTimeliness `json:"models"`
	BestModel  string            `json:"best_model"`
	WorstModel string            `json:"worst_model"`
	MeanScore  float64           `json:"mean_score"`
}

var freshPhrases = []string{
	"as of", "recently", "currently",
	"the latest", "up to date", "new version",
	"latest release", "recent update", "just released",
	"now available", "current version", "updated",
	"this week", "this month", "today",
}

var temporalPhrases = []string{
	"since version", "was introduced in", "deprecated in",
	"removed in", "added in", "changed in",
	"starting from", "beginning with", "as of version",
	"release notes", "changelog", "migration guide",
	"breaking change", "backward compatible", "upgrade path",
}

var accuratePhrases = []string{
	"according to the documentation", "the official docs state",
	"per the specification", "the api reference shows",
	"as documented", "the changelog notes",
	"verified against", "confirmed in",
	"tested with", "compatible with version",
	"supports version", "requires version",
}

var awarenessPhrases = []string{
	"note that this has changed", "be aware that",
	"this was updated", "the behavior changed",
	"previously this was", "in older versions",
	"the new approach", "the recommended way now",
	"this replaces", "superseded by",
	"no longer supported", "end of life",
}

var stalePhrases = []string{
	"i believe", "i think this might",
	"i'm not sure if", "this may have changed",
	"last time i checked", "as far as i know",
	"typically", "usually", "in most cases",
	"it depends", "generally speaking",
	"historically", "traditionally",
}

var ambiguousPhrases = []string{
	"probably", "perhaps", "maybe",
	"could be", "might be", "possibly",
	"seems like", "appears to", "looks like",
	"i would guess", "my understanding is",
	"not entirely sure",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, TimelinessDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	fresh := countPhrases(combined, freshPhrases)
	temporal := countPhrases(combined, temporalPhrases)
	accurate := countPhrases(combined, accuratePhrases)
	awareness := countPhrases(combined, awarenessPhrases)
	stale := countPhrases(combined, stalePhrases)
	ambiguous := countPhrases(combined, ambiguousPhrases)

	timely := fresh + temporal + accurate + awareness

	speedScore := 0.2
	if words > 0 {
		speedScore = math.Min(1.0, float64(fresh)*2.0/float64(words)*10.0+0.2)
	}

	freshnessScore := 0.2
	if words > 0 {
		freshnessScore = math.Min(1.0, float64(temporal)*2.0/float64(words)*10.0+0.2)
	}

	accuracyScore := 0.3
	if accurate > 0 || temporal > 0 {
		accuracyScore = math.Min(1.0, float64(accurate+temporal)*0.15+0.3)
	}

	awarenessScore := 0.2
	if words > 0 {
		awarenessScore = math.Min(1.0, float64(awareness)*2.0/float64(words)*10.0+0.2)
	}

	decayScore := 0.2
	if stale == 0 && ambiguous == 0 {
		decayScore = 0.8
	} else if stale+ambiguous < 3 {
		decayScore = 0.5
	}

	detail := TimelinessDetail{
		ResponseSpeed:    math.Round(speedScore*100) / 100,
		ContextFreshness: math.Round(freshnessScore*100) / 100,
		TemporalAccuracy: math.Round(accuracyScore*100) / 100,
		UpdateAwareness:  math.Round(awarenessScore*100) / 100,
		DecayResistance:  math.Round(decayScore*100) / 100,
		TimelyCount:      timely,
		StaleCount:       stale,
		AmbiguousCount:   ambiguous,
	}

	score := 30.0
	score += speedScore * 15.0
	score += freshnessScore * 15.0
	score += accuracyScore * 10.0
	score += awarenessScore * 10.0
	score += decayScore * 10.0

	score -= float64(stale) * 4.0
	score -= float64(ambiguous) * 5.0

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

	var models []ModelTimeliness
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail TimelinessDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.ResponseSpeed += detail.ResponseSpeed
			totalDetail.ContextFreshness += detail.ContextFreshness
			totalDetail.TemporalAccuracy += detail.TemporalAccuracy
			totalDetail.UpdateAwareness += detail.UpdateAwareness
			totalDetail.DecayResistance += detail.DecayResistance
			totalDetail.TimelyCount += detail.TimelyCount
			totalDetail.StaleCount += detail.StaleCount
			totalDetail.AmbiguousCount += detail.AmbiguousCount

			if detail.StaleCount > 3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.ResponseSpeed = math.Round(totalDetail.ResponseSpeed/n*100) / 100
		totalDetail.ContextFreshness = math.Round(totalDetail.ContextFreshness/n*100) / 100
		totalDetail.TemporalAccuracy = math.Round(totalDetail.TemporalAccuracy/n*100) / 100
		totalDetail.UpdateAwareness = math.Round(totalDetail.UpdateAwareness/n*100) / 100
		totalDetail.DecayResistance = math.Round(totalDetail.DecayResistance/n*100) / 100

		models = append(models, ModelTimeliness{
			Model:           mn,
			TimelinessScore: math.Round(avgScore*10) / 10,
			Grade:           scoreToGrade(avgScore),
			Detail:          totalDetail,
			SampleCount:     len(entries),
			LabCount:        len(labs),
			Concerns:        totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].TimelinessScore > models[j].TimelinessScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.TimelinessScore
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
	b.WriteString("Response Timeliness Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No timeliness data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.TimelinessScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "response_speed", m.Detail.ResponseSpeed)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "context_freshness", m.Detail.ContextFreshness)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "temporal_accuracy", m.Detail.TemporalAccuracy)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "update_awareness", m.Detail.UpdateAwareness)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "decay_resistance", m.Detail.DecayResistance)
		fmt.Fprintf(&b, "    %-28s %d\n", "timely_count", m.Detail.TimelyCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "stale_count", m.Detail.StaleCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "ambiguous_count", m.Detail.AmbiguousCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
