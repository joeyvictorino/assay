package watermark

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/joeyvictorino/assay/internal/model"
)

type DimensionScore struct {
	Name  string  `json:"name"`
	Score float64 `json:"score"`
}

type ModelWatermark struct {
	Model        string           `json:"model"`
	OverallScore float64          `json:"overall_score"`
	Grade        string           `json:"grade"`
	Dimensions   []DimensionScore `json:"dimensions"`
	SampleCount  int              `json:"sample_count"`
}

type Report struct {
	Models        []ModelWatermark `json:"models"`
	StrongestMark string           `json:"strongest_mark"`
	WeakestMark   string           `json:"weakest_mark"`
	MeanScore     float64          `json:"mean_score"`
}

func tokenDistEntropy(text string) float64 {
	freq := map[rune]int{}
	total := 0
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			freq[unicode.ToLower(r)]++
			total++
		}
	}
	if total == 0 {
		return 0
	}
	var ent float64
	for _, c := range freq {
		p := float64(c) / float64(total)
		if p > 0 {
			ent -= p * math.Log2(p)
		}
	}
	return ent
}

func bigramRepeatRatio(text string) float64 {
	words := strings.Fields(strings.ToLower(text))
	if len(words) < 2 {
		return 0
	}
	seen := map[string]int{}
	total := 0
	for i := 0; i < len(words)-1; i++ {
		bg := words[i] + " " + words[i+1]
		seen[bg]++
		total++
	}
	if total == 0 {
		return 0
	}
	repeats := 0
	for _, c := range seen {
		if c > 1 {
			repeats += c - 1
		}
	}
	return float64(repeats) / float64(total)
}

func presenceScore(texts []string) float64 {
	if len(texts) == 0 {
		return 50
	}
	var entropies []float64
	for _, t := range texts {
		entropies = append(entropies, tokenDistEntropy(t))
	}
	var sum float64
	for _, e := range entropies {
		sum += e
	}
	avgEnt := sum / float64(len(entropies))
	score := math.Min(100, avgEnt*22)
	return math.Round(score*10) / 10
}

func strengthScore(texts []string) float64 {
	if len(texts) == 0 {
		return 50
	}
	var ratios []float64
	for _, t := range texts {
		ratios = append(ratios, bigramRepeatRatio(t))
	}
	var sum float64
	for _, r := range ratios {
		sum += r
	}
	avgRatio := sum / float64(len(ratios))
	score := 100 - math.Min(100, avgRatio*500)
	if score < 0 {
		score = 0
	}
	return math.Round(score*10) / 10
}

func consistencyScore(texts []string) float64 {
	if len(texts) < 2 {
		return 50
	}
	var entropies []float64
	for _, t := range texts {
		entropies = append(entropies, tokenDistEntropy(t))
	}
	var sum float64
	for _, e := range entropies {
		sum += e
	}
	mean := sum / float64(len(entropies))
	var variance float64
	for _, e := range entropies {
		d := e - mean
		variance += d * d
	}
	variance /= float64(len(entropies))
	stddev := math.Sqrt(variance)
	score := 100 - math.Min(100, stddev*40)
	if score < 0 {
		score = 0
	}
	return math.Round(score*10) / 10
}

func resilienceScore(texts []string) float64 {
	if len(texts) == 0 {
		return 50
	}
	var lengths []int
	for _, t := range texts {
		lengths = append(lengths, len(strings.Fields(t)))
	}
	var sum float64
	for _, l := range lengths {
		sum += float64(l)
	}
	avgLen := sum / float64(len(lengths))
	score := math.Min(100, avgLen*2)
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

	var models []ModelWatermark
	for mn, texts := range modelTexts {
		dims := []DimensionScore{
			{Name: "watermark_presence", Score: presenceScore(texts)},
			{Name: "watermark_strength", Score: strengthScore(texts)},
			{Name: "watermark_consistency", Score: consistencyScore(texts)},
			{Name: "watermark_resilience", Score: resilienceScore(texts)},
		}

		var total float64
		for _, d := range dims {
			total += d.Score
		}
		overall := total / float64(len(dims))

		models = append(models, ModelWatermark{
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
		r.StrongestMark = models[0].Model
		r.WeakestMark = models[len(models)-1].Model
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
	b.WriteString("AI Watermark Detection Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No watermark data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Strongest watermark:  %s\n", r.StrongestMark)
	fmt.Fprintf(&b, "  Weakest watermark:    %s\n\n", r.WeakestMark)

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
