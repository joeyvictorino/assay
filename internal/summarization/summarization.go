package summarization

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type SummarizationDetail struct {
	KeyPointCoverage   float64 `json:"key_point_coverage"`
	CompressionRatio   float64 `json:"compression_ratio"`
	HallucinatedCount  int     `json:"hallucinated_count"`
	CoherenceScore     float64 `json:"coherence_score"`
	RedundancyCount    int     `json:"redundancy_count"`
	MissedKeyPoints    int     `json:"missed_key_points"`
	FactualErrors      int     `json:"factual_errors"`
	FlowScore          float64 `json:"flow_score"`
}

type ModelSummarization struct {
	Model              string              `json:"model"`
	SummarizationScore float64             `json:"summarization_score"`
	Grade              string              `json:"grade"`
	Detail             SummarizationDetail `json:"detail"`
	SampleCount        int                 `json:"sample_count"`
	LabCount           int                 `json:"lab_count"`
	PoorSummaries      int                 `json:"poor_summaries"`
}

type Report struct {
	Models        []ModelSummarization `json:"models"`
	BestModel     string               `json:"best_model"`
	WorstModel    string               `json:"worst_model"`
	MeanScore     float64              `json:"mean_score"`
}

var hallucinationPhrases = []string{
	"as mentioned earlier", "according to the source",
	"the document states", "it was reported that",
	"studies have shown", "research indicates",
	"experts agree", "it is well known",
	"as everyone knows", "obviously",
	"clearly stated", "the data shows",
}

var coherencePhrases = []string{
	"therefore", "consequently", "as a result",
	"in summary", "to conclude", "overall",
	"in other words", "specifically", "namely",
	"for instance", "in particular", "thus",
	"hence", "accordingly", "furthermore",
}

var redundancyPhrases = []string{
	"as previously mentioned", "as stated above",
	"again", "to reiterate", "once more",
	"as noted", "repeating", "redundantly",
	"in other words", "that is to say",
}

var flowPhrases = []string{
	"first", "second", "third", "next",
	"then", "finally", "additionally",
	"moreover", "however", "meanwhile",
	"subsequently", "afterwards", "later",
	"before that", "following this",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func computeCompressionRatio(summary, source string) float64 {
	sourceWords := len(strings.Fields(source))
	summaryWords := len(strings.Fields(summary))
	if sourceWords == 0 {
		return 1.0
	}
	return float64(summaryWords) / float64(sourceWords)
}

func extractKeyPhrases(text string) []string {
	words := strings.Fields(strings.ToLower(text))
	freq := map[string]int{}
	for _, w := range words {
		cleaned := strings.Trim(w, ".,;:!?\"'()[]{}")
		if len(cleaned) > 4 {
			freq[cleaned]++
		}
	}

	type kv struct {
		k string
		v int
	}
	var sorted []kv
	for k, v := range freq {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].v > sorted[j].v
	})

	var result []string
	limit := 10
	if len(sorted) < limit {
		limit = len(sorted)
	}
	for i := 0; i < limit; i++ {
		result = append(result, sorted[i].k)
	}
	return result
}

func keyPointCoverage(summary string, keyPhrases []string) (float64, int) {
	if len(keyPhrases) == 0 {
		return 1.0, 0
	}
	lower := strings.ToLower(summary)
	covered := 0
	for _, kp := range keyPhrases {
		if strings.Contains(lower, kp) {
			covered++
		}
	}
	missed := len(keyPhrases) - covered
	return float64(covered) / float64(len(keyPhrases)), missed
}

func scoreSummary(summary, evidence string) (float64, SummarizationDetail) {
	hallucinated := countPhrases(summary, hallucinationPhrases)
	coherence := countPhrases(summary, coherencePhrases)
	redundancy := countPhrases(summary, redundancyPhrases)
	flow := countPhrases(summary, flowPhrases)

	compression := computeCompressionRatio(summary, evidence)
	keyPhrases := extractKeyPhrases(evidence)
	coverage, missed := keyPointCoverage(summary, keyPhrases)

	summaryWords := len(strings.Fields(summary))
	factualErrors := 0
	if summaryWords > 0 {
		factualErrors = hallucinated
	}

	coherenceScore := 0.0
	if summaryWords > 0 {
		coherenceScore = math.Min(1.0, float64(coherence)*5.0/float64(summaryWords)*10.0)
	}
	flowScore := 0.0
	if summaryWords > 0 {
		flowScore = math.Min(1.0, float64(flow)*5.0/float64(summaryWords)*10.0)
	}

	detail := SummarizationDetail{
		KeyPointCoverage:  math.Round(coverage*100) / 100,
		CompressionRatio:  math.Round(compression*100) / 100,
		HallucinatedCount: hallucinated,
		CoherenceScore:    math.Round(coherenceScore*100) / 100,
		RedundancyCount:   redundancy,
		MissedKeyPoints:   missed,
		FactualErrors:     factualErrors,
		FlowScore:         math.Round(flowScore*100) / 100,
	}

	score := 50.0
	score += coverage * 25.0

	if compression > 0 && compression < 0.5 {
		score += 10.0
	} else if compression >= 0.5 && compression < 0.8 {
		score += 5.0
	} else if compression >= 0.8 {
		score -= 5.0
	}

	score -= float64(hallucinated) * 3.0
	score -= float64(redundancy) * 2.0
	score += coherenceScore * 5.0
	score += flowScore * 5.0

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

	var models []ModelSummarization
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail SummarizationDetail
		poorSummaries := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreSummary(e.Summary, evidence)
			totalScore += score
			totalDetail.KeyPointCoverage += detail.KeyPointCoverage
			totalDetail.CompressionRatio += detail.CompressionRatio
			totalDetail.HallucinatedCount += detail.HallucinatedCount
			totalDetail.CoherenceScore += detail.CoherenceScore
			totalDetail.RedundancyCount += detail.RedundancyCount
			totalDetail.MissedKeyPoints += detail.MissedKeyPoints
			totalDetail.FactualErrors += detail.FactualErrors
			totalDetail.FlowScore += detail.FlowScore

			if score < 50 {
				poorSummaries++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.KeyPointCoverage = math.Round(totalDetail.KeyPointCoverage/n*100) / 100
		totalDetail.CompressionRatio = math.Round(totalDetail.CompressionRatio/n*100) / 100
		totalDetail.CoherenceScore = math.Round(totalDetail.CoherenceScore/n*100) / 100
		totalDetail.FlowScore = math.Round(totalDetail.FlowScore/n*100) / 100

		models = append(models, ModelSummarization{
			Model:              mn,
			SummarizationScore: math.Round(avgScore*10) / 10,
			Grade:              scoreToGrade(avgScore),
			Detail:             totalDetail,
			SampleCount:        len(entries),
			LabCount:           len(labs),
			PoorSummaries:      poorSummaries,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].SummarizationScore > models[j].SummarizationScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.SummarizationScore
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
	b.WriteString("Summarization Quality Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No summarization data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.SummarizationScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "key_point_coverage", m.Detail.KeyPointCoverage)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "compression_ratio", m.Detail.CompressionRatio)
		fmt.Fprintf(&b, "    %-28s %d\n", "hallucinated_count", m.Detail.HallucinatedCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "coherence_score", m.Detail.CoherenceScore)
		fmt.Fprintf(&b, "    %-28s %d\n", "redundancy_count", m.Detail.RedundancyCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "missed_key_points", m.Detail.MissedKeyPoints)
		fmt.Fprintf(&b, "    %-28s %d\n", "factual_errors", m.Detail.FactualErrors)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "flow_score", m.Detail.FlowScore)
		fmt.Fprintf(&b, "    Poor summaries: %d\n", m.PoorSummaries)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
