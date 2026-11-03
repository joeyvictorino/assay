package factuality

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type DimensionScore struct {
	Name  string  `json:"name"`
	Score float64 `json:"score"`
}

type ModelFactuality struct {
	Model        string           `json:"model"`
	OverallScore float64          `json:"overall_score"`
	Grade        string           `json:"grade"`
	Dimensions   []DimensionScore `json:"dimensions"`
	ClaimCount   int              `json:"claim_count"`
	SampleCount  int              `json:"sample_count"`
}

type Report struct {
	Models       []ModelFactuality `json:"models"`
	MostFactual  string            `json:"most_factual"`
	LeastFactual string            `json:"least_factual"`
	MeanScore    float64           `json:"mean_score"`
}

var (
	sentenceEnd  = regexp.MustCompile(`[.!?]+\s+|[.!?]+$`)
	claimVerbs   = []string{"is", "are", "was", "were", "has", "have", "had", "will", "can", "does", "contains", "shows", "indicates", "means", "equals", "produces", "causes", "results"}
	hedgeWords   = []string{"might", "may", "could", "possibly", "perhaps", "likely", "probably", "approximately", "roughly", "estimated", "uncertain", "unclear", "debatable", "it seems", "appears to", "I think", "I believe"}
	citationPats = []*regexp.Regexp{
		regexp.MustCompile(`\[\d+\]`),
		regexp.MustCompile(`\((?:(?:19|20)\d{2})\)`),
		regexp.MustCompile(`(?i)according to`),
		regexp.MustCompile(`(?i)source:`),
		regexp.MustCompile(`(?i)(?:doi|isbn|url|https?://)`),
		regexp.MustCompile(`(?i)(?:et al\.|ibid\.|op\.? cit\.?)`),
	}
)

func splitSentences(text string) []string {
	parts := sentenceEnd.Split(text, -1)
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len(p) > 3 {
			result = append(result, p)
		}
	}
	return result
}

func isVerifiableClaim(sentence string) bool {
	lower := strings.ToLower(sentence)
	for _, v := range claimVerbs {
		if strings.Contains(lower, " "+v+" ") {
			return true
		}
	}
	return false
}

func claimDensityScore(texts []string) (float64, int) {
	totalSentences := 0
	totalClaims := 0
	for _, text := range texts {
		sents := splitSentences(text)
		totalSentences += len(sents)
		for _, s := range sents {
			if isVerifiableClaim(s) {
				totalClaims++
			}
		}
	}
	if totalSentences == 0 {
		return 50, 0
	}
	ratio := float64(totalClaims) / float64(totalSentences)
	score := math.Min(100, ratio*100)
	return math.Round(score*10) / 10, totalClaims
}

func internalConsistencyScore(texts []string) float64 {
	if len(texts) == 0 {
		return 50
	}
	negations := []string{"not", "never", "no", "cannot", "isn't", "aren't", "wasn't", "weren't", "doesn't", "don't", "hasn't", "haven't", "won't", "shouldn't"}
	contradictions := 0
	totalChecked := 0

	for _, text := range texts {
		sents := splitSentences(text)
		for i := 0; i < len(sents); i++ {
			for j := i + 1; j < len(sents); j++ {
				totalChecked++
				lowerI := strings.ToLower(sents[i])
				lowerJ := strings.ToLower(sents[j])
				wordsI := strings.Fields(lowerI)
				wordsJ := strings.Fields(lowerJ)
				shared := sharedWords(wordsI, wordsJ)
				if shared < 2 {
					continue
				}
				iHasNeg := containsAny(lowerI, negations)
				jHasNeg := containsAny(lowerJ, negations)
				if iHasNeg != jHasNeg && shared >= 3 {
					contradictions++
				}
			}
		}
	}
	if totalChecked == 0 {
		return 100
	}
	ratio := float64(contradictions) / float64(totalChecked)
	score := 100 - math.Min(100, ratio*500)
	if score < 0 {
		score = 0
	}
	return math.Round(score*10) / 10
}

func sharedWords(a, b []string) int {
	set := map[string]bool{}
	for _, w := range a {
		if len(w) > 2 {
			set[w] = true
		}
	}
	count := 0
	for _, w := range b {
		if set[w] {
			count++
		}
	}
	return count
}

func containsAny(text string, terms []string) bool {
	for _, t := range terms {
		if strings.Contains(text, t) {
			return true
		}
	}
	return false
}

func citationPresenceScore(texts []string) float64 {
	if len(texts) == 0 {
		return 50
	}
	cited := 0
	for _, text := range texts {
		for _, pat := range citationPats {
			if pat.MatchString(text) {
				cited++
				break
			}
		}
	}
	ratio := float64(cited) / float64(len(texts))
	return math.Round(ratio * 100 * 10) / 10
}

func hedgingAppropriatenessScore(texts []string) float64 {
	if len(texts) == 0 {
		return 50
	}
	totalSentences := 0
	hedgedSentences := 0
	claimSentences := 0

	for _, text := range texts {
		sents := splitSentences(text)
		totalSentences += len(sents)
		for _, s := range sents {
			lower := strings.ToLower(s)
			isHedged := containsAny(lower, hedgeWords)
			isClaim := isVerifiableClaim(s)
			if isHedged {
				hedgedSentences++
			}
			if isClaim {
				claimSentences++
			}
		}
	}
	if totalSentences == 0 {
		return 50
	}
	hedgeRatio := float64(hedgedSentences) / float64(totalSentences)
	score := 50.0
	if hedgeRatio > 0.05 && hedgeRatio < 0.5 {
		score = 80 + (0.25-math.Abs(hedgeRatio-0.25))*80
	} else if hedgeRatio <= 0.05 && claimSentences > 0 {
		score = 30
	} else if hedgeRatio >= 0.5 {
		score = 40
	}
	if score > 100 {
		score = 100
	}
	return math.Round(score*10) / 10
}

func Analyze(findings []model.Finding) Report {
	if len(findings) == 0 {
		return Report{}
	}

	modelTexts := map[string][]string{}
	for _, f := range findings {
		mn := f.Model.Model
		if mn == "" {
			mn = "unknown"
		}
		modelTexts[mn] = append(modelTexts[mn], f.Summary)
	}

	var models []ModelFactuality
	for mn, texts := range modelTexts {
		cd, claims := claimDensityScore(texts)
		ic := internalConsistencyScore(texts)
		cp := citationPresenceScore(texts)
		ha := hedgingAppropriatenessScore(texts)

		dims := []DimensionScore{
			{Name: "claim_density", Score: cd},
			{Name: "internal_consistency", Score: ic},
			{Name: "citation_presence", Score: cp},
			{Name: "hedging_appropriateness", Score: ha},
		}

		var total float64
		for _, d := range dims {
			total += d.Score
		}
		overall := total / float64(len(dims))

		models = append(models, ModelFactuality{
			Model:        mn,
			OverallScore: math.Round(overall*10) / 10,
			Grade:        scoreToGrade(overall),
			Dimensions:   dims,
			ClaimCount:   claims,
			SampleCount:  len(texts),
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].OverallScore > models[j].OverallScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.OverallScore
	}

	r := Report{Models: models}
	if len(models) > 0 {
		r.MostFactual = models[0].Model
		r.LeastFactual = models[len(models)-1].Model
		r.MeanScore = math.Round(sumScore/float64(len(models))*10) / 10
	}
	return r
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

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Factual Accuracy Validation\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No factuality data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:   %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:         %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Most factual:       %s\n", r.MostFactual)
	fmt.Fprintf(&b, "  Least factual:      %s\n\n", r.LeastFactual)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (claims=%d, samples=%d)\n",
			m.Model, m.Grade, m.OverallScore, m.ClaimCount, m.SampleCount)
		for _, d := range m.Dimensions {
			fmt.Fprintf(&b, "    %-28s %.1f\n", d.Name, d.Score)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
