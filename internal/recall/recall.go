package recall

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type RecallDetail struct {
	CompletenessCount int     `json:"completeness_count"`
	CoverageCount     int     `json:"coverage_count"`
	SkippedCount      int     `json:"skipped_count"`
	OmittedCount      int     `json:"omitted_count"`
	PartialCount      int     `json:"partial_count"`
	RecallRatio       float64 `json:"recall_ratio"`
	WordCount         int     `json:"word_count"`
}

type ModelRecall struct {
	Model       string       `json:"model"`
	RecallScore float64      `json:"recall_score"`
	Grade       string       `json:"grade"`
	Detail      RecallDetail `json:"detail"`
	SampleCount int          `json:"sample_count"`
	LabCount    int          `json:"lab_count"`
}

type Report struct {
	Models     []ModelRecall `json:"models"`
	BestModel  string        `json:"best_model"`
	WorstModel string        `json:"worst_model"`
	MeanScore  float64       `json:"mean_score"`
}

var completenessPhrases = []string{
	"addressing all", "covering each", "comprehensively",
	"in its entirety", "all aspects", "every component",
	"thoroughly", "exhaustively", "fully addressed",
	"complete analysis", "all points covered", "end to end",
	"holistic", "all facets", "nothing omitted",
}

var coveragePhrases = []string{
	"additionally", "furthermore", "also considering",
	"another important factor", "equally important",
	"not to overlook", "it is also worth noting",
	"in addition", "beyond that", "coupled with",
	"as well as", "together with", "along with",
	"supplementing", "complementing",
}

var skippedPhrases = []string{
	"skipping", "omitting", "leaving out",
	"not addressed", "beyond the scope", "out of scope",
	"will not discuss", "setting aside",
	"ignoring", "disregarding", "passing over",
}

var omittedPhrases = []string{
	"i forgot to", "i should have included",
	"missing from my analysis", "i overlooked",
	"an important omission", "i neglected",
	"left unaddressed", "gap in coverage",
	"incomplete coverage", "not fully explored",
}

var partialPhrases = []string{
	"partially", "to some extent", "in part",
	"only briefly", "a surface-level", "cursory",
	"not in depth", "superficially", "limited coverage",
	"high-level only", "touching on", "briefly mentioned",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, RecallDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	completeness := countPhrases(combined, completenessPhrases)
	coverage := countPhrases(combined, coveragePhrases)
	skipped := countPhrases(combined, skippedPhrases)
	omitted := countPhrases(combined, omittedPhrases)
	partial := countPhrases(combined, partialPhrases)

	recallSignals := float64(completeness + coverage)
	gapSignals := float64(skipped + omitted + partial)

	ratio := 0.5
	if recallSignals+gapSignals > 0 {
		ratio = recallSignals / (recallSignals + gapSignals)
	}

	detail := RecallDetail{
		CompletenessCount: completeness,
		CoverageCount:     coverage,
		SkippedCount:      skipped,
		OmittedCount:      omitted,
		PartialCount:      partial,
		RecallRatio:       math.Round(ratio*100) / 100,
		WordCount:         words,
	}

	score := 40.0
	score += ratio * 35.0
	score += math.Min(float64(completeness)*3.0, 15.0)
	score += math.Min(float64(coverage)*2.5, 12.0)
	score -= float64(skipped) * 4.0
	score -= float64(omitted) * 5.0
	score -= float64(partial) * 2.5

	if words > 200 && coverage > 2 {
		score += 5.0
	}

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

	var models []ModelRecall
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail RecallDetail
		labs := map[string]bool{}

		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.CompletenessCount += detail.CompletenessCount
			totalDetail.CoverageCount += detail.CoverageCount
			totalDetail.SkippedCount += detail.SkippedCount
			totalDetail.OmittedCount += detail.OmittedCount
			totalDetail.PartialCount += detail.PartialCount
			totalDetail.WordCount += detail.WordCount
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.RecallRatio = math.Round(totalDetail.RecallRatio/n*100) / 100

		models = append(models, ModelRecall{
			Model:       mn,
			RecallScore: math.Round(avgScore*10) / 10,
			Grade:       scoreToGrade(avgScore),
			Detail:      totalDetail,
			SampleCount: len(entries),
			LabCount:    len(labs),
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].RecallScore > models[j].RecallScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.RecallScore
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
	b.WriteString("Response Recall Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No recall data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.RecallScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "completeness_count", m.Detail.CompletenessCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "coverage_count", m.Detail.CoverageCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "skipped_count", m.Detail.SkippedCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "omitted_count", m.Detail.OmittedCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "partial_count", m.Detail.PartialCount)
		b.WriteString("\n")
	}
	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
