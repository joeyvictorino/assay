package outputconsistency

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

var stopWords = map[string]bool{
	"a": true, "an": true, "the": true, "is": true, "are": true,
	"was": true, "were": true, "be": true, "been": true, "being": true,
	"have": true, "has": true, "had": true, "do": true, "does": true,
	"did": true, "will": true, "would": true, "could": true, "should": true,
	"may": true, "might": true, "shall": true, "can": true, "need": true,
	"to": true, "of": true, "in": true, "for": true, "on": true,
	"with": true, "at": true, "by": true, "from": true, "as": true,
	"into": true, "through": true, "during": true, "before": true,
	"after": true, "above": true, "below": true, "between": true,
	"and": true, "but": true, "or": true, "nor": true, "not": true,
	"so": true, "yet": true, "both": true, "either": true, "neither": true,
	"this": true, "that": true, "these": true, "those": true,
	"it": true, "its": true, "i": true, "we": true, "you": true,
	"he": true, "she": true, "they": true, "me": true, "him": true,
	"her": true, "us": true, "them": true, "my": true, "your": true,
	"his": true, "our": true, "their": true,
}

type ConsistencyDetail struct {
	SemanticOverlap  float64 `json:"semantic_overlap"`
	FormatStability  float64 `json:"format_stability"`
	LengthVariance   float64 `json:"length_variance"`
	KeywordStability float64 `json:"keyword_stability"`
}

type ModelConsistency struct {
	Model             string            `json:"model"`
	ConsistencyScore  float64           `json:"consistency_score"`
	Grade             string            `json:"grade"`
	Detail            ConsistencyDetail `json:"detail"`
	SampleCount       int               `json:"sample_count"`
	LabCount          int               `json:"lab_count"`
	HighDriftCount    int               `json:"high_drift_count"`
	FormatChanges     int               `json:"format_changes"`
}

type Report struct {
	Models         []ModelConsistency `json:"models"`
	MostConsistent string             `json:"most_consistent"`
	LeastConsistent string            `json:"least_consistent"`
	MeanScore      float64            `json:"mean_score"`
}

func extractKeywords(text string) map[string]int {
	words := strings.Fields(strings.ToLower(text))
	kw := map[string]int{}
	for _, w := range words {
		clean := strings.Trim(w, ".,;:!?\"'()[]{}")
		if len(clean) < 2 || stopWords[clean] {
			continue
		}
		kw[clean]++
	}
	return kw
}

func jaccardSimilarity(a, b map[string]int) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 100
	}
	union := map[string]bool{}
	for k := range a {
		union[k] = true
	}
	for k := range b {
		union[k] = true
	}
	if len(union) == 0 {
		return 100
	}
	inter := 0
	for k := range union {
		if a[k] > 0 && b[k] > 0 {
			inter++
		}
	}
	return float64(inter) / float64(len(union)) * 100
}

func semanticOverlap(summaries []string) float64 {
	if len(summaries) < 2 {
		return 100
	}
	kwSets := make([]map[string]int, len(summaries))
	for i, s := range summaries {
		kwSets[i] = extractKeywords(s)
	}
	var total float64
	pairs := 0
	for i := 0; i < len(kwSets); i++ {
		for j := i + 1; j < len(kwSets); j++ {
			total += jaccardSimilarity(kwSets[i], kwSets[j])
			pairs++
		}
	}
	if pairs == 0 {
		return 100
	}
	return total / float64(pairs)
}

func detectFormat(text string) string {
	lower := strings.ToLower(text)
	hasBullets := strings.Contains(lower, "- ") || strings.Contains(lower, "* ")
	hasNumbered := false
	for _, line := range strings.Split(lower, "\n") {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 2 && trimmed[0] >= '1' && trimmed[0] <= '9' && (trimmed[1] == '.' || trimmed[1] == ')') {
			hasNumbered = true
			break
		}
	}
	hasHeaders := strings.Contains(text, "# ") || strings.Contains(text, "## ")
	hasCode := strings.Contains(text, "```") || strings.Contains(text, "`")

	parts := []string{}
	if hasBullets {
		parts = append(parts, "bullets")
	}
	if hasNumbered {
		parts = append(parts, "numbered")
	}
	if hasHeaders {
		parts = append(parts, "headers")
	}
	if hasCode {
		parts = append(parts, "code")
	}
	if len(parts) == 0 {
		return "prose"
	}
	sort.Strings(parts)
	return strings.Join(parts, "+")
}

func formatStability(summaries []string) float64 {
	if len(summaries) < 2 {
		return 100
	}
	formats := map[string]int{}
	for _, s := range summaries {
		formats[detectFormat(s)]++
	}
	maxCount := 0
	for _, c := range formats {
		if c > maxCount {
			maxCount = c
		}
	}
	return float64(maxCount) / float64(len(summaries)) * 100
}

func lengthVarianceScore(summaries []string) float64 {
	if len(summaries) < 2 {
		return 100
	}
	lengths := make([]float64, len(summaries))
	var sum float64
	for i, s := range summaries {
		lengths[i] = float64(len(strings.Fields(s)))
		sum += lengths[i]
	}
	mean := sum / float64(len(lengths))
	if mean == 0 {
		return 100
	}
	var variance float64
	for _, l := range lengths {
		diff := l - mean
		variance += diff * diff
	}
	variance /= float64(len(lengths))
	cv := math.Sqrt(variance) / mean
	score := 100 - math.Min(100, cv*200)
	if score < 0 {
		score = 0
	}
	return score
}

func keywordStability(summaries []string) float64 {
	if len(summaries) < 2 {
		return 100
	}
	allKW := map[string]int{}
	kwSets := make([]map[string]int, len(summaries))
	for i, s := range summaries {
		kwSets[i] = extractKeywords(s)
		for k := range kwSets[i] {
			allKW[k]++
		}
	}
	if len(allKW) == 0 {
		return 100
	}
	stableCount := 0
	threshold := len(summaries) / 2
	if threshold < 1 {
		threshold = 1
	}
	for _, count := range allKW {
		if count > threshold {
			stableCount++
		}
	}
	return math.Min(100, float64(stableCount)/float64(len(allKW))*100)
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

	type labEntry struct {
		lab     string
		summary string
	}
	modelLabs := map[string][]labEntry{}
	for _, f := range findings {
		mn := f.Model.Model
		if mn == "" {
			mn = "unknown"
		}
		modelLabs[mn] = append(modelLabs[mn], labEntry{lab: f.Lab, summary: f.Summary})
	}

	var models []ModelConsistency
	for mn, entries := range modelLabs {
		byLab := map[string][]string{}
		for _, e := range entries {
			byLab[e.lab] = append(byLab[e.lab], e.summary)
		}

		var totalOverlap, totalFormat, totalLength, totalKeyword float64
		labsWithMultiple := 0
		highDrift := 0
		fmtChanges := 0

		for _, summaries := range byLab {
			if len(summaries) < 2 {
				continue
			}
			labsWithMultiple++

			overlap := semanticOverlap(summaries)
			fmtScore := formatStability(summaries)
			lenScore := lengthVarianceScore(summaries)
			kwScore := keywordStability(summaries)

			totalOverlap += overlap
			totalFormat += fmtScore
			totalLength += lenScore
			totalKeyword += kwScore

			if overlap < 40 {
				highDrift++
			}
			if fmtScore < 80 {
				fmtChanges++
			}
		}

		if labsWithMultiple == 0 {
			models = append(models, ModelConsistency{
				Model:            mn,
				ConsistencyScore: 50,
				Grade:            "N/A",
				Detail: ConsistencyDetail{
					SemanticOverlap:  50,
					FormatStability:  50,
					LengthVariance:   50,
					KeywordStability: 50,
				},
				SampleCount: len(entries),
				LabCount:    len(byLab),
			})
			continue
		}

		n := float64(labsWithMultiple)
		avgOverlap := totalOverlap / n
		avgFormat := totalFormat / n
		avgLength := totalLength / n
		avgKeyword := totalKeyword / n

		overall := (avgOverlap*0.35 + avgFormat*0.2 + avgLength*0.15 + avgKeyword*0.3)

		models = append(models, ModelConsistency{
			Model:            mn,
			ConsistencyScore: math.Round(overall*10) / 10,
			Grade:            scoreToGrade(overall),
			Detail: ConsistencyDetail{
				SemanticOverlap:  math.Round(avgOverlap*10) / 10,
				FormatStability:  math.Round(avgFormat*10) / 10,
				LengthVariance:   math.Round(avgLength*10) / 10,
				KeywordStability: math.Round(avgKeyword*10) / 10,
			},
			SampleCount:    len(entries),
			LabCount:       len(byLab),
			HighDriftCount: highDrift,
			FormatChanges:  fmtChanges,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].ConsistencyScore > models[j].ConsistencyScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.ConsistencyScore
	}

	r := Report{Models: models}
	if len(models) > 0 {
		r.MostConsistent = models[0].Model
		r.LeastConsistent = models[len(models)-1].Model
		r.MeanScore = math.Round(sumScore/float64(len(models))*10) / 10
	}
	return r
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Output Consistency Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No consistency data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Most consistent:      %s\n", r.MostConsistent)
	fmt.Fprintf(&b, "  Least consistent:     %s\n\n", r.LeastConsistent)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.ConsistencyScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "semantic_overlap", m.Detail.SemanticOverlap)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "format_stability", m.Detail.FormatStability)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "length_variance", m.Detail.LengthVariance)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "keyword_stability", m.Detail.KeywordStability)
		fmt.Fprintf(&b, "    High-drift labs: %d  Format changes: %d\n",
			m.HighDriftCount, m.FormatChanges)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
