package anticipation

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type AnticipationDetail struct {
	WarningCount        int     `json:"warning_count"`
	EdgeCaseCount       int     `json:"edge_case_count"`
	AlternativeCount    int     `json:"alternative_count"`
	TroubleshootCount   int     `json:"troubleshoot_count"`
	FutureCount         int     `json:"future_count"`
	WordCount           int     `json:"word_count"`
	AnticipationRatio   float64 `json:"anticipation_ratio"`
}

type ModelAnticipation struct {
	Model       string             `json:"model"`
	Score       float64            `json:"score"`
	Grade       string             `json:"grade"`
	Detail      AnticipationDetail `json:"detail"`
	SampleCount int                `json:"sample_count"`
	LabCount    int                `json:"lab_count"`
}

type Report struct {
	Models     []ModelAnticipation `json:"models"`
	BestModel  string              `json:"best_model"`
	WorstModel string              `json:"worst_model"`
	MeanScore  float64             `json:"mean_score"`
}

var warningPhrases = []string{
	"note that", "be aware", "keep in mind", "important:",
	"warning:", "caution:", "please note", "be careful",
	"watch out", "heads up", "be mindful", "take note",
	"remember that", "don't forget",
}

var edgeCasePhrases = []string{
	"however,", "except", "unless", "in case",
	"if you", "otherwise", "on the other hand",
	"edge case", "corner case", "special case",
	"but if", "when this", "in some cases", "depending on",
}

var alternativePhrases = []string{
	"alternatively", "another option", "you could also",
	"another approach", "or you can", "a different way",
	"instead, you", "as an alternative", "one more option",
	"you might also", "there's also",
}

var troubleshootPhrases = []string{
	"if this doesn't work", "common issues", "troubleshooting",
	"if you encounter", "if you see an error", "to debug",
	"if it fails", "if the problem persists", "common mistakes",
	"a frequent cause", "often caused by", "make sure that",
}

var futurePhrases = []string{
	"in the future", "going forward", "consider also",
	"for future reference", "down the road", "eventually",
	"as a next step", "in the long run", "for later",
	"you may want to", "it's worth noting for later",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func AnalyzeText(text string) (float64, AnticipationDetail) {
	words := strings.Fields(text)
	wordCount := len(words)

	if wordCount == 0 {
		return 50.0, AnticipationDetail{}
	}

	warnings := countPhrases(text, warningPhrases)
	edgeCases := countPhrases(text, edgeCasePhrases)
	alternatives := countPhrases(text, alternativePhrases)
	troubleshoot := countPhrases(text, troubleshootPhrases)
	future := countPhrases(text, futurePhrases)

	totalSignals := float64(warnings + edgeCases + alternatives + troubleshoot + future)

	anticipationRatio := 0.0
	if float64(wordCount) > 0 {
		anticipationRatio = totalSignals / float64(wordCount)
	}

	score := 50.0
	score += float64(warnings) * 3.0
	score += float64(edgeCases) * 3.5
	score += float64(alternatives) * 4.0
	score += float64(troubleshoot) * 4.0
	score += float64(future) * 3.0

	density := totalSignals / float64(wordCount) * 100
	score += density * 2.0

	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	detail := AnticipationDetail{
		WarningCount:      warnings,
		EdgeCaseCount:     edgeCases,
		AlternativeCount:  alternatives,
		TroubleshootCount: troubleshoot,
		FutureCount:       future,
		WordCount:         wordCount,
		AnticipationRatio: math.Round(anticipationRatio*100) / 100,
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

	var models []ModelAnticipation
	for mn, entries := range modelEntries {
		var totalScore float64
		var aggDetail AnticipationDetail
		labs := map[string]bool{}

		for _, e := range entries {
			labs[e.Lab] = true
			score, detail := AnalyzeText(e.Summary)
			totalScore += score
			aggDetail.WarningCount += detail.WarningCount
			aggDetail.EdgeCaseCount += detail.EdgeCaseCount
			aggDetail.AlternativeCount += detail.AlternativeCount
			aggDetail.TroubleshootCount += detail.TroubleshootCount
			aggDetail.FutureCount += detail.FutureCount
			aggDetail.WordCount += detail.WordCount
			aggDetail.AnticipationRatio += detail.AnticipationRatio
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		aggDetail.AnticipationRatio = math.Round(aggDetail.AnticipationRatio/n*100) / 100

		models = append(models, ModelAnticipation{
			Model:       mn,
			Score:       math.Round(avgScore*10) / 10,
			Grade:       scoreToGrade(avgScore),
			Detail:      aggDetail,
			SampleCount: len(entries),
			LabCount:    len(labs),
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].Score > models[j].Score
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.Score
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
	b.WriteString("Response Anticipation Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No anticipation data available.\n")
		return b.String()
	}

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s\n", m.Model)
		fmt.Fprintf(&b, "    Score: %.1f  Grade: %s\n", m.Score, m.Grade)
		fmt.Fprintf(&b, "    Warnings: %d  Edge Cases: %d  Alternatives: %d\n",
			m.Detail.WarningCount, m.Detail.EdgeCaseCount, m.Detail.AlternativeCount)
		fmt.Fprintf(&b, "    Troubleshoot: %d  Future: %d  Ratio: %.2f\n",
			m.Detail.TroubleshootCount, m.Detail.FutureCount, m.Detail.AnticipationRatio)
		fmt.Fprintf(&b, "    Samples: %d  Labs: %d\n\n",
			m.SampleCount, m.LabCount)
	}

	fmt.Fprintf(&b, "  Best: %s  Worst: %s  Mean: %.1f\n",
		r.BestModel, r.WorstModel, r.MeanScore)

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data)
}
