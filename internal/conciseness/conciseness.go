package conciseness

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type DimensionScore struct {
	Name  string  `json:"name"`
	Score float64 `json:"score"`
}

type ModelConciseness struct {
	Model        string           `json:"model"`
	OverallScore float64          `json:"overall_score"`
	Grade        string           `json:"grade"`
	Dimensions   []DimensionScore `json:"dimensions"`
	SampleCount  int              `json:"sample_count"`
}

type Report struct {
	Models       []ModelConciseness `json:"models"`
	MostConcise  string             `json:"most_concise"`
	LeastConcise string             `json:"least_concise"`
	MeanScore    float64            `json:"mean_score"`
}

var fillerWords = map[string]bool{
	"actually":    true,
	"basically":   true,
	"certainly":   true,
	"definitely":  true,
	"essentially": true,
	"honestly":    true,
	"just":        true,
	"literally":   true,
	"obviously":   true,
	"really":      true,
	"simply":      true,
	"very":        true,
	"quite":       true,
	"rather":      true,
	"somewhat":    true,
	"perhaps":     true,
	"anyway":      true,
	"furthermore": true,
	"moreover":    true,
	"nevertheless": true,
}

func wordEfficiency(texts []string) float64 {
	if len(texts) == 0 {
		return 50
	}
	var ratios []float64
	for _, t := range texts {
		words := strings.Fields(t)
		if len(words) == 0 {
			continue
		}
		unique := map[string]bool{}
		for _, w := range words {
			unique[strings.ToLower(w)] = true
		}
		ratios = append(ratios, float64(len(unique))/float64(len(words)))
	}
	if len(ratios) == 0 {
		return 50
	}
	var sum float64
	for _, r := range ratios {
		sum += r
	}
	avg := sum / float64(len(ratios))
	score := math.Min(100, avg*120)
	return math.Round(score*10) / 10
}

func redundancyScore(texts []string) float64 {
	if len(texts) == 0 {
		return 50
	}
	var ratios []float64
	for _, t := range texts {
		sentences := splitSentences(t)
		if len(sentences) < 2 {
			ratios = append(ratios, 0)
			continue
		}
		total := 0
		repeated := 0
		for i := 0; i < len(sentences); i++ {
			wordsI := sentenceWords(sentences[i])
			for j := i + 1; j < len(sentences); j++ {
				wordsJ := sentenceWords(sentences[j])
				overlap := wordOverlap(wordsI, wordsJ)
				total++
				if overlap > 0.6 {
					repeated++
				}
			}
		}
		if total == 0 {
			ratios = append(ratios, 0)
		} else {
			ratios = append(ratios, float64(repeated)/float64(total))
		}
	}
	var sum float64
	for _, r := range ratios {
		sum += r
	}
	avg := sum / float64(len(ratios))
	score := 100 - math.Min(100, avg*200)
	if score < 0 {
		score = 0
	}
	return math.Round(score*10) / 10
}

func fillerRatio(texts []string) float64 {
	if len(texts) == 0 {
		return 50
	}
	var ratios []float64
	for _, t := range texts {
		words := strings.Fields(strings.ToLower(t))
		if len(words) == 0 {
			continue
		}
		count := 0
		for _, w := range words {
			clean := strings.Trim(w, ".,;:!?\"'()")
			if fillerWords[clean] {
				count++
			}
		}
		ratios = append(ratios, float64(count)/float64(len(words)))
	}
	if len(ratios) == 0 {
		return 50
	}
	var sum float64
	for _, r := range ratios {
		sum += r
	}
	avg := sum / float64(len(ratios))
	score := 100 - math.Min(100, avg*500)
	if score < 0 {
		score = 0
	}
	return math.Round(score*10) / 10
}

func directnessScore(texts []string) float64 {
	if len(texts) == 0 {
		return 50
	}
	var scores []float64
	for _, t := range texts {
		words := strings.Fields(t)
		sentences := splitSentences(t)
		if len(sentences) == 0 || len(words) == 0 {
			scores = append(scores, 50)
			continue
		}
		avgWordsPerSentence := float64(len(words)) / float64(len(sentences))
		s := 100 - math.Min(100, math.Max(0, (avgWordsPerSentence-10)*4))
		scores = append(scores, s)
	}
	var sum float64
	for _, s := range scores {
		sum += s
	}
	avg := sum / float64(len(scores))
	return math.Round(avg*10) / 10
}

func splitSentences(text string) []string {
	var sentences []string
	current := strings.Builder{}
	for _, r := range text {
		current.WriteRune(r)
		if r == '.' || r == '!' || r == '?' {
			s := strings.TrimSpace(current.String())
			if len(s) > 0 {
				sentences = append(sentences, s)
			}
			current.Reset()
		}
	}
	if s := strings.TrimSpace(current.String()); len(s) > 0 {
		sentences = append(sentences, s)
	}
	return sentences
}

func sentenceWords(s string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(strings.ToLower(s)) {
		clean := strings.Trim(w, ".,;:!?\"'()")
		if clean != "" {
			m[clean] = true
		}
	}
	return m
}

func wordOverlap(a, b map[string]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	common := 0
	for w := range a {
		if b[w] {
			common++
		}
	}
	smaller := len(a)
	if len(b) < smaller {
		smaller = len(b)
	}
	return float64(common) / float64(smaller)
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

	var models []ModelConciseness
	for mn, texts := range modelTexts {
		dims := []DimensionScore{
			{Name: "word_efficiency", Score: wordEfficiency(texts)},
			{Name: "redundancy_score", Score: redundancyScore(texts)},
			{Name: "filler_ratio", Score: fillerRatio(texts)},
			{Name: "directness", Score: directnessScore(texts)},
		}

		var total float64
		for _, d := range dims {
			total += d.Score
		}
		overall := total / float64(len(dims))

		models = append(models, ModelConciseness{
			Model:        mn,
			OverallScore: math.Round(overall*10) / 10,
			Grade:        scoreToGrade(overall),
			Dimensions:   dims,
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
		r.MostConcise = models[0].Model
		r.LeastConcise = models[len(models)-1].Model
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
	b.WriteString("Response Conciseness Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No conciseness data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:    %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:          %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Most concise:        %s\n", r.MostConcise)
	fmt.Fprintf(&b, "  Least concise:       %s\n\n", r.LeastConcise)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples)\n",
			m.Model, m.Grade, m.OverallScore, m.SampleCount)
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
