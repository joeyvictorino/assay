package faithfulness

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

type FaithfulnessDetail struct {
	ClaimCoverage    float64 `json:"claim_coverage"`
	FactualAlignment float64 `json:"factual_alignment"`
	HallucinationPen float64 `json:"hallucination_penalty"`
	SourceGrounding  float64 `json:"source_grounding"`
}

type ModelFaithfulness struct {
	Model            string              `json:"model"`
	FaithfulnessScore float64            `json:"faithfulness_score"`
	Grade            string              `json:"grade"`
	Detail           FaithfulnessDetail  `json:"detail"`
	SampleCount      int                 `json:"sample_count"`
	HallucinationCount int              `json:"hallucination_count"`
	UnsupportedClaims  int              `json:"unsupported_claims"`
}

type Report struct {
	Models        []ModelFaithfulness `json:"models"`
	MostFaithful  string             `json:"most_faithful"`
	LeastFaithful string             `json:"least_faithful"`
	MeanScore     float64            `json:"mean_score"`
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

func claimCoverage(lab, summary string) float64 {
	labKW := extractKeywords(lab)
	sumKW := extractKeywords(summary)
	if len(labKW) == 0 {
		return 50
	}
	covered := 0
	for k := range labKW {
		if sumKW[k] > 0 {
			covered++
		}
	}
	return math.Min(100, float64(covered)/float64(len(labKW))*100)
}

func factualAlignment(lab, summary string) float64 {
	labKW := extractKeywords(lab)
	sumKW := extractKeywords(summary)
	if len(sumKW) == 0 {
		return 0
	}
	aligned := 0
	for k := range sumKW {
		if labKW[k] > 0 {
			aligned++
		}
	}
	ratio := float64(aligned) / float64(len(sumKW))
	return math.Round(ratio * 1000) / 10
}

func hallucinationPenalty(lab, summary string) float64 {
	labKW := extractKeywords(lab)
	sumKW := extractKeywords(summary)
	if len(sumKW) == 0 {
		return 100
	}

	fabricated := 0
	for w := range sumKW {
		found := false
		for lw := range labKW {
			if w == lw || strings.Contains(w, lw) || strings.Contains(lw, w) {
				found = true
				break
			}
		}
		if !found {
			fabricated++
		}
	}
	ratio := float64(fabricated) / float64(len(sumKW))
	score := 100 - math.Min(100, ratio*130)
	if score < 0 {
		score = 0
	}
	return math.Round(score*10) / 10
}

func sourceGrounding(summary string) float64 {
	lower := strings.ToLower(summary)
	speculativePhrases := []string{
		"i think", "probably", "maybe", "perhaps",
		"it seems", "it appears", "i believe", "i assume",
		"might be", "could be", "possibly", "presumably",
		"i would guess", "speculation", "hypothetically",
	}
	specCount := 0
	for _, p := range speculativePhrases {
		if strings.Contains(lower, p) {
			specCount++
		}
	}

	words := strings.Fields(summary)
	if len(words) == 0 {
		return 50
	}

	score := 100 - math.Min(100, float64(specCount)*18)
	if score < 0 {
		score = 0
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

	var models []ModelFaithfulness
	for mn, pairs := range modelPairs {
		var sumCoverage, sumAlignment, sumHalluc, sumGrounding float64
		hallucCount := 0
		unsupported := 0

		for _, p := range pairs {
			coverage := claimCoverage(p.lab, p.summary)
			alignment := factualAlignment(p.lab, p.summary)
			halluc := hallucinationPenalty(p.lab, p.summary)
			grounding := sourceGrounding(p.summary)

			sumCoverage += coverage
			sumAlignment += alignment
			sumHalluc += halluc
			sumGrounding += grounding

			if halluc < 50 {
				hallucCount++
			}
			if coverage < 20 {
				unsupported++
			}
		}

		n := float64(len(pairs))
		avgCoverage := sumCoverage / n
		avgAlignment := sumAlignment / n
		avgHalluc := sumHalluc / n
		avgGrounding := sumGrounding / n

		overall := (avgCoverage + avgAlignment + avgHalluc + avgGrounding) / 4

		models = append(models, ModelFaithfulness{
			Model:             mn,
			FaithfulnessScore: math.Round(overall*10) / 10,
			Grade:             scoreToGrade(overall),
			Detail: FaithfulnessDetail{
				ClaimCoverage:    math.Round(avgCoverage*10) / 10,
				FactualAlignment: math.Round(avgAlignment*10) / 10,
				HallucinationPen: math.Round(avgHalluc*10) / 10,
				SourceGrounding:  math.Round(avgGrounding*10) / 10,
			},
			SampleCount:       len(pairs),
			HallucinationCount: hallucCount,
			UnsupportedClaims:  unsupported,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].FaithfulnessScore > models[j].FaithfulnessScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.FaithfulnessScore
	}

	r := Report{Models: models}
	if len(models) > 0 {
		r.MostFaithful = models[0].Model
		r.LeastFaithful = models[len(models)-1].Model
		r.MeanScore = math.Round(sumScore/float64(len(models))*10) / 10
	}
	return r
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Response Faithfulness Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No faithfulness data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:    %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:          %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Most faithful:       %s\n", r.MostFaithful)
	fmt.Fprintf(&b, "  Least faithful:      %s\n\n", r.LeastFaithful)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples)\n",
			m.Model, m.Grade, m.FaithfulnessScore, m.SampleCount)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "claim_coverage", m.Detail.ClaimCoverage)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "factual_alignment", m.Detail.FactualAlignment)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "hallucination_penalty", m.Detail.HallucinationPen)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "source_grounding", m.Detail.SourceGrounding)
		fmt.Fprintf(&b, "    Hallucination warnings: %d  Unsupported claims: %d\n",
			m.HallucinationCount, m.UnsupportedClaims)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
