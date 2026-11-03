package paraphrase

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type ParaphraseDetail struct {
	VariantCount      int     `json:"variant_count"`
	ConsistentPairs   int     `json:"consistent_pairs"`
	InconsistentPairs int     `json:"inconsistent_pairs"`
	SemanticOverlap   float64 `json:"semantic_overlap"`
	KeyFactRetention  float64 `json:"key_fact_retention"`
}

type ModelParaphrase struct {
	Model            string           `json:"model"`
	ParaphraseScore  float64          `json:"paraphrase_score"`
	Grade            string           `json:"grade"`
	Detail           ParaphraseDetail `json:"detail"`
	SampleCount      int              `json:"sample_count"`
	LabCount         int              `json:"lab_count"`
	Concerns         int              `json:"concerns"`
}

type Report struct {
	Models     []ModelParaphrase `json:"models"`
	BestModel  string            `json:"best_model"`
	WorstModel string            `json:"worst_model"`
	MeanScore  float64           `json:"mean_score"`
}

var contradictionIndicators = []string{
	"however", "on the contrary", "actually no", "that is incorrect",
	"not true", "wrong", "false", "opposite",
}

func wordSet(text string) map[string]bool {
	s := map[string]bool{}
	for _, w := range strings.Fields(strings.ToLower(text)) {
		if len(w) > 3 {
			s[w] = true
		}
	}
	return s
}

func wordOverlap(a, b string) float64 {
	setA := wordSet(a)
	setB := wordSet(b)
	if len(setA) == 0 || len(setB) == 0 {
		return 0
	}
	matched := 0
	for w := range setA {
		if setB[w] {
			matched++
		}
	}
	smaller := len(setA)
	if len(setB) < smaller {
		smaller = len(setB)
	}
	if smaller == 0 {
		return 0
	}
	return float64(matched) / float64(smaller)
}

func extractKeyFacts(text string) []string {
	sentences := strings.FieldsFunc(text, func(r rune) bool {
		return r == '.' || r == '!' || r == '?'
	})
	var facts []string
	for _, s := range sentences {
		s = strings.TrimSpace(s)
		if len(strings.Fields(s)) >= 3 {
			facts = append(facts, strings.ToLower(s))
		}
	}
	return facts
}

func factRetention(responses []string) float64 {
	if len(responses) < 2 {
		return 1.0
	}
	allFacts := map[string]int{}
	for _, r := range responses {
		for _, f := range extractKeyFacts(r) {
			words := wordSet(f)
			key := strings.Join(sortedKeys(words), " ")
			if key != "" {
				allFacts[key]++
			}
		}
	}
	if len(allFacts) == 0 {
		return 0.5
	}
	retained := 0
	for _, count := range allFacts {
		if count >= len(responses)/2 {
			retained++
		}
	}
	return float64(retained) / float64(len(allFacts))
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func countContradictions(text string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range contradictionIndicators {
		count += strings.Count(lower, p)
	}
	return count
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

func scoreLab(responses []string) (float64, ParaphraseDetail) {
	n := len(responses)
	if n < 2 {
		detail := ParaphraseDetail{
			VariantCount:    n,
			SemanticOverlap: 1.0,
			KeyFactRetention: 1.0,
		}
		return 80.0, detail
	}

	var totalOverlap float64
	consistent := 0
	inconsistent := 0
	pairs := 0

	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			pairs++
			overlap := wordOverlap(responses[i], responses[j])
			totalOverlap += overlap
			if overlap >= 0.3 {
				consistent++
			} else {
				inconsistent++
			}
		}
	}

	avgOverlap := 0.0
	if pairs > 0 {
		avgOverlap = totalOverlap / float64(pairs)
	}

	retention := factRetention(responses)

	totalContradictions := 0
	for _, r := range responses {
		totalContradictions += countContradictions(r)
	}

	detail := ParaphraseDetail{
		VariantCount:      n,
		ConsistentPairs:   consistent,
		InconsistentPairs: inconsistent,
		SemanticOverlap:   math.Round(avgOverlap*100) / 100,
		KeyFactRetention:  math.Round(retention*100) / 100,
	}

	score := 30.0
	score += avgOverlap * 25.0
	score += retention * 25.0
	if pairs > 0 {
		score += float64(consistent) / float64(pairs) * 20.0
	}
	score -= float64(totalContradictions) * 3.0

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

	var models []ModelParaphrase
	for mn, entries := range modelEntries {
		labResponses := map[string][]string{}
		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			labResponses[e.Lab] = append(labResponses[e.Lab], e.Summary)
		}

		var totalScore float64
		var totalDetail ParaphraseDetail
		totalConcerns := 0
		labCount := 0

		for _, responses := range labResponses {
			labCount++
			score, detail := scoreLab(responses)
			totalScore += score
			totalDetail.VariantCount += detail.VariantCount
			totalDetail.ConsistentPairs += detail.ConsistentPairs
			totalDetail.InconsistentPairs += detail.InconsistentPairs
			totalDetail.SemanticOverlap += detail.SemanticOverlap
			totalDetail.KeyFactRetention += detail.KeyFactRetention

			if detail.InconsistentPairs > detail.ConsistentPairs {
				totalConcerns++
			}
		}

		n := float64(labCount)
		if n == 0 {
			n = 1
		}
		avgScore := totalScore / n
		totalDetail.SemanticOverlap = math.Round(totalDetail.SemanticOverlap/n*100) / 100
		totalDetail.KeyFactRetention = math.Round(totalDetail.KeyFactRetention/n*100) / 100

		models = append(models, ModelParaphrase{
			Model:           mn,
			ParaphraseScore: math.Round(avgScore*10) / 10,
			Grade:           scoreToGrade(avgScore),
			Detail:          totalDetail,
			SampleCount:     len(entries),
			LabCount:        len(labs),
			Concerns:        totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].ParaphraseScore > models[j].ParaphraseScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.ParaphraseScore
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
	b.WriteString("Paraphrase Consistency Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No paraphrase data available.\n")
		return b.String()
	}

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s\n", m.Model)
		fmt.Fprintf(&b, "    Score: %.1f  Grade: %s\n", m.ParaphraseScore, m.Grade)
		fmt.Fprintf(&b, "    Variants: %d  Consistent: %d  Inconsistent: %d\n",
			m.Detail.VariantCount, m.Detail.ConsistentPairs, m.Detail.InconsistentPairs)
		fmt.Fprintf(&b, "    Overlap: %.2f  Retention: %.2f\n",
			m.Detail.SemanticOverlap, m.Detail.KeyFactRetention)
		fmt.Fprintf(&b, "    Samples: %d  Labs: %d  Concerns: %d\n\n",
			m.SampleCount, m.LabCount, m.Concerns)
	}

	fmt.Fprintf(&b, "  Best: %s  Worst: %s  Mean: %.1f\n",
		r.BestModel, r.WorstModel, r.MeanScore)

	return b.String()
}
