package completeness

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

type CompletenessDetail struct {
	ComponentCoverage float64 `json:"component_coverage"`
	DepthScore        float64 `json:"depth_score"`
	TrailOffPenalty   float64 `json:"trail_off_penalty"`
	StructureScore    float64 `json:"structure_score"`
}

type ModelCompleteness struct {
	Model            string             `json:"model"`
	CompletenessScore float64           `json:"completeness_score"`
	Grade            string             `json:"grade"`
	Detail           CompletenessDetail `json:"detail"`
	SampleCount      int                `json:"sample_count"`
	PartialCount     int                `json:"partial_count"`
	TruncatedCount   int                `json:"truncated_count"`
}

type Report struct {
	Models        []ModelCompleteness `json:"models"`
	MostComplete  string              `json:"most_complete"`
	LeastComplete string              `json:"least_complete"`
	MeanScore     float64             `json:"mean_score"`
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

func componentCoverage(lab, summary string) float64 {
	labKW := extractKeywords(lab)
	sumKW := extractKeywords(summary)
	if len(labKW) == 0 {
		return 50
	}
	covered := 0
	for k := range labKW {
		if sumKW[k] > 0 {
			covered++
			continue
		}
		for sk := range sumKW {
			if strings.Contains(sk, k) || strings.Contains(k, sk) {
				covered++
				break
			}
		}
	}
	return math.Min(100, float64(covered)/float64(len(labKW))*100)
}

func depthScore(summary string) float64 {
	words := strings.Fields(summary)
	wordCount := len(words)
	if wordCount == 0 {
		return 0
	}

	depthIndicators := []string{
		"because", "therefore", "specifically", "additionally",
		"furthermore", "moreover", "example", "instance",
		"detail", "explains", "caused", "results", "leads",
		"step", "first", "second", "third", "finally",
	}

	lower := strings.ToLower(summary)
	indicatorCount := 0
	for _, ind := range depthIndicators {
		if strings.Contains(lower, ind) {
			indicatorCount++
		}
	}

	lengthScore := math.Min(100, float64(wordCount)/50*100)
	indicatorScore := math.Min(100, float64(indicatorCount)/3*100)

	return math.Round((lengthScore*0.4+indicatorScore*0.6)*10) / 10
}

func trailOffPenalty(summary string) float64 {
	lower := strings.ToLower(strings.TrimSpace(summary))
	score := 100.0

	trailOffPhrases := []string{
		"etc", "and so on", "and more", "among others",
		"...", "to name a few", "and similar",
	}
	for _, p := range trailOffPhrases {
		if strings.HasSuffix(lower, p) || strings.HasSuffix(lower, p+".") {
			score -= 20
			break
		}
	}

	if strings.Count(lower, "...") > 0 {
		score -= float64(strings.Count(lower, "...")) * 10
	}

	words := strings.Fields(summary)
	if len(words) > 3 {
		last3 := strings.ToLower(strings.Join(words[len(words)-3:], " "))
		abruptEndings := []string{"but i", "however i", "although", "but we", "however we"}
		for _, ae := range abruptEndings {
			if strings.Contains(last3, ae) {
				score -= 25
				break
			}
		}
	}

	if score < 0 {
		score = 0
	}
	return math.Round(score*10) / 10
}

func structureScore(summary string) float64 {
	if len(strings.TrimSpace(summary)) == 0 {
		return 0
	}

	sentences := 0
	for _, c := range summary {
		if c == '.' || c == '!' || c == '?' {
			sentences++
		}
	}
	if sentences == 0 {
		sentences = 1
	}

	score := 50.0

	if sentences >= 2 {
		score += 20
	}
	if sentences >= 3 {
		score += 10
	}

	words := strings.Fields(summary)
	if len(words) >= 10 {
		score += 10
	}
	if len(words) >= 25 {
		score += 10
	}

	if score > 100 {
		score = 100
	}
	return math.Round(score*10) / 10
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

func isPartial(lab, summary string) bool {
	labKW := extractKeywords(lab)
	sumKW := extractKeywords(summary)
	if len(labKW) == 0 {
		return false
	}
	covered := 0
	for k := range labKW {
		if sumKW[k] > 0 {
			covered++
		}
	}
	ratio := float64(covered) / float64(len(labKW))
	return ratio > 0 && ratio < 0.5
}

func isTruncated(summary string) bool {
	trimmed := strings.TrimSpace(summary)
	if len(trimmed) == 0 {
		return false
	}
	last := trimmed[len(trimmed)-1]
	if last != '.' && last != '!' && last != '?' && last != '"' && last != ')' && last != ']' {
		words := strings.Fields(trimmed)
		if len(words) > 5 {
			return true
		}
	}
	if strings.HasSuffix(trimmed, "...") {
		return true
	}
	return false
}

func Analyze(findings []model.Finding) Report {
	if len(findings) == 0 {
		return Report{}
	}

	type pair struct {
		lab     string
		summary string
	}
	modelPairs := map[string][]pair{}
	for _, f := range findings {
		mn := f.Model.Model
		if mn == "" {
			mn = "unknown"
		}
		modelPairs[mn] = append(modelPairs[mn], pair{lab: f.Lab, summary: f.Summary})
	}

	var models []ModelCompleteness
	for mn, pairs := range modelPairs {
		var sumCov, sumDepth, sumTrail, sumStruct float64
		partialCount := 0
		truncCount := 0

		for _, p := range pairs {
			cov := componentCoverage(p.lab, p.summary)
			depth := depthScore(p.summary)
			trail := trailOffPenalty(p.summary)
			structure := structureScore(p.summary)

			sumCov += cov
			sumDepth += depth
			sumTrail += trail
			sumStruct += structure

			if isPartial(p.lab, p.summary) {
				partialCount++
			}
			if isTruncated(p.summary) {
				truncCount++
			}
		}

		n := float64(len(pairs))
		avgCov := sumCov / n
		avgDepth := sumDepth / n
		avgTrail := sumTrail / n
		avgStruct := sumStruct / n

		overall := (avgCov*0.35 + avgDepth*0.25 + avgTrail*0.20 + avgStruct*0.20)

		models = append(models, ModelCompleteness{
			Model:            mn,
			CompletenessScore: math.Round(overall*10) / 10,
			Grade:            scoreToGrade(overall),
			Detail: CompletenessDetail{
				ComponentCoverage: math.Round(avgCov*10) / 10,
				DepthScore:        math.Round(avgDepth*10) / 10,
				TrailOffPenalty:   math.Round(avgTrail*10) / 10,
				StructureScore:    math.Round(avgStruct*10) / 10,
			},
			SampleCount:    len(pairs),
			PartialCount:   partialCount,
			TruncatedCount: truncCount,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].CompletenessScore > models[j].CompletenessScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.CompletenessScore
	}

	r := Report{Models: models}
	if len(models) > 0 {
		r.MostComplete = models[0].Model
		r.LeastComplete = models[len(models)-1].Model
		r.MeanScore = math.Round(sumScore/float64(len(models))*10) / 10
	}
	return r
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Response Completeness Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No completeness data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:    %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:          %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Most complete:       %s\n", r.MostComplete)
	fmt.Fprintf(&b, "  Least complete:      %s\n\n", r.LeastComplete)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples)\n",
			m.Model, m.Grade, m.CompletenessScore, m.SampleCount)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "component_coverage", m.Detail.ComponentCoverage)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "depth_score", m.Detail.DepthScore)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "trail_off_penalty", m.Detail.TrailOffPenalty)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "structure_score", m.Detail.StructureScore)
		fmt.Fprintf(&b, "    Partial answers: %d  Truncated: %d\n",
			m.PartialCount, m.TruncatedCount)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
