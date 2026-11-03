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
	TopicCoverage    float64 `json:"topic_coverage"`
	EdgeCaseHandling float64 `json:"edge_case_handling"`
	DepthScore       float64 `json:"depth_score"`
	ExhaustiveScore  float64 `json:"exhaustive_score"`
	GapResistance    float64 `json:"gap_resistance"`
	CoveredCount     int     `json:"covered_count"`
	OmittedCount     int     `json:"omitted_count"`
	PartialCount     int     `json:"partial_count"`
}

type ModelRecall struct {
	Model       string       `json:"model"`
	RecallScore float64      `json:"recall_score"`
	Grade       string       `json:"grade"`
	Detail      RecallDetail `json:"detail"`
	SampleCount int          `json:"sample_count"`
	LabCount    int          `json:"lab_count"`
	Concerns    int          `json:"concerns"`
}

type Report struct {
	Models     []ModelRecall `json:"models"`
	BestModel  string        `json:"best_model"`
	WorstModel string        `json:"worst_model"`
	MeanScore  float64       `json:"mean_score"`
}

var coveragePhrases = []string{
	"first", "second", "third", "additionally",
	"furthermore", "moreover", "also", "in addition",
	"another aspect", "another consideration",
	"on one hand", "on the other hand",
	"regarding", "with respect to", "as for",
}

var edgeCasePhrases = []string{
	"edge case", "corner case", "special case",
	"exception", "caveat", "limitation",
	"however", "note that", "be aware",
	"important to note", "keep in mind",
	"watch out for", "pitfall",
	"unless", "except when",
}

var depthPhrases = []string{
	"in detail", "specifically", "in particular",
	"for example", "for instance", "such as",
	"to elaborate", "more precisely",
	"this means that", "the reason is",
	"because", "therefore", "consequently",
	"the underlying", "fundamentally",
}

var exhaustivePhrases = []string{
	"comprehensive", "thorough", "complete",
	"all aspects", "every", "each of",
	"the full", "entirely", "in its entirety",
	"no exception", "without exception",
	"covers all", "addresses all",
}

var omissionPhrases = []string{
	"i won't cover", "beyond the scope",
	"i'll skip", "not going into",
	"that's a separate topic", "out of scope",
	"let's focus on", "only covering",
	"briefly", "in short", "to summarize",
	"long story short", "tldr",
}

var partialPhrases = []string{
	"partially", "to some extent",
	"not fully", "incomplete",
	"more or less", "roughly",
	"approximately", "sort of",
	"kind of", "in a way",
	"somewhat", "a bit",
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

	coverage := countPhrases(combined, coveragePhrases)
	edgeCases := countPhrases(combined, edgeCasePhrases)
	depth := countPhrases(combined, depthPhrases)
	exhaustive := countPhrases(combined, exhaustivePhrases)
	omission := countPhrases(combined, omissionPhrases)
	partial := countPhrases(combined, partialPhrases)

	covered := coverage + edgeCases + depth + exhaustive

	topicScore := 0.2
	if words > 0 {
		topicScore = math.Min(1.0, float64(coverage)*2.0/float64(words)*10.0+0.2)
	}

	edgeScore := 0.2
	if words > 0 {
		edgeScore = math.Min(1.0, float64(edgeCases)*2.0/float64(words)*10.0+0.2)
	}

	depthScore := 0.3
	if depth > 0 || exhaustive > 0 {
		depthScore = math.Min(1.0, float64(depth+exhaustive)*0.15+0.3)
	}

	exhaustiveScore := 0.2
	if words > 0 {
		exhaustiveScore = math.Min(1.0, float64(exhaustive)*2.0/float64(words)*10.0+0.2)
	}

	gapScore := 0.2
	if omission == 0 && partial == 0 {
		gapScore = 0.8
	} else if omission+partial < 3 {
		gapScore = 0.5
	}

	detail := RecallDetail{
		TopicCoverage:    math.Round(topicScore*100) / 100,
		EdgeCaseHandling: math.Round(edgeScore*100) / 100,
		DepthScore:       math.Round(depthScore*100) / 100,
		ExhaustiveScore:  math.Round(exhaustiveScore*100) / 100,
		GapResistance:    math.Round(gapScore*100) / 100,
		CoveredCount:     covered,
		OmittedCount:     omission,
		PartialCount:     partial,
	}

	score := 30.0
	score += topicScore * 15.0
	score += edgeScore * 15.0
	score += depthScore * 10.0
	score += exhaustiveScore * 10.0
	score += gapScore * 10.0

	score -= float64(omission) * 4.0
	score -= float64(partial) * 5.0

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
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.TopicCoverage += detail.TopicCoverage
			totalDetail.EdgeCaseHandling += detail.EdgeCaseHandling
			totalDetail.DepthScore += detail.DepthScore
			totalDetail.ExhaustiveScore += detail.ExhaustiveScore
			totalDetail.GapResistance += detail.GapResistance
			totalDetail.CoveredCount += detail.CoveredCount
			totalDetail.OmittedCount += detail.OmittedCount
			totalDetail.PartialCount += detail.PartialCount

			if detail.OmittedCount > 3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.TopicCoverage = math.Round(totalDetail.TopicCoverage/n*100) / 100
		totalDetail.EdgeCaseHandling = math.Round(totalDetail.EdgeCaseHandling/n*100) / 100
		totalDetail.DepthScore = math.Round(totalDetail.DepthScore/n*100) / 100
		totalDetail.ExhaustiveScore = math.Round(totalDetail.ExhaustiveScore/n*100) / 100
		totalDetail.GapResistance = math.Round(totalDetail.GapResistance/n*100) / 100

		models = append(models, ModelRecall{
			Model:       mn,
			RecallScore: math.Round(avgScore*10) / 10,
			Grade:       scoreToGrade(avgScore),
			Detail:      totalDetail,
			SampleCount: len(entries),
			LabCount:    len(labs),
			Concerns:    totalConcerns,
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
		fmt.Fprintf(&b, "    %-28s %.2f\n", "topic_coverage", m.Detail.TopicCoverage)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "edge_case_handling", m.Detail.EdgeCaseHandling)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "depth_score", m.Detail.DepthScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "exhaustive_score", m.Detail.ExhaustiveScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "gap_resistance", m.Detail.GapResistance)
		fmt.Fprintf(&b, "    %-28s %d\n", "covered_count", m.Detail.CoveredCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "omitted_count", m.Detail.OmittedCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "partial_count", m.Detail.PartialCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
