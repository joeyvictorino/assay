package modelprofile

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

type DimensionScore struct {
	Dimension string  `json:"dimension"`
	Score     float64 `json:"score"`
	Grade     string  `json:"grade"`
	Samples   int     `json:"samples"`
}

type Strength struct {
	Dimension   string  `json:"dimension"`
	Score       float64 `json:"score"`
	Description string  `json:"description"`
}

type Weakness struct {
	Dimension   string  `json:"dimension"`
	Score       float64 `json:"score"`
	Description string  `json:"description"`
}

type ProfileSummary struct {
	ModelName      string  `json:"model_name"`
	TotalSamples   int     `json:"total_samples"`
	Dimensions     int     `json:"dimensions"`
	OverallScore   float64 `json:"overall_score"`
	OverallGrade   string  `json:"overall_grade"`
	Strengths      int     `json:"strengths"`
	Weaknesses     int     `json:"weaknesses"`
}

type ModelProfileReport struct {
	Scores     []DimensionScore `json:"scores"`
	Strengths  []Strength       `json:"strengths"`
	Weaknesses []Weakness       `json:"weaknesses"`
	Summary    ProfileSummary   `json:"summary"`
}

type EvalResult struct {
	Dimension string  `json:"dimension"`
	Score     float64 `json:"score"`
}

func Analyze(modelName string, results []EvalResult) ModelProfileReport {
	dimScores := map[string][]float64{}
	for _, r := range results {
		dimScores[r.Dimension] = append(dimScores[r.Dimension], r.Score)
	}

	var scores []DimensionScore
	totalScore := 0.0
	for dim, vals := range dimScores {
		avg := mean(vals)
		scores = append(scores, DimensionScore{
			Dimension: dim,
			Score:     avg,
			Grade:     floatGrade(avg),
			Samples:   len(vals),
		})
		totalScore += avg
	}
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Dimension < scores[j].Dimension
	})

	overall := 0.0
	if len(scores) > 0 {
		overall = totalScore / float64(len(scores))
	}

	var strengths []Strength
	var weaknesses []Weakness

	for _, s := range scores {
		if s.Score >= 0.8 {
			strengths = append(strengths, Strength{
				Dimension:   s.Dimension,
				Score:       s.Score,
				Description: fmt.Sprintf("Strong performance in %s (%.0f%%)", s.Dimension, s.Score*100),
			})
		}
		if s.Score < 0.6 {
			weaknesses = append(weaknesses, Weakness{
				Dimension:   s.Dimension,
				Score:       s.Score,
				Description: fmt.Sprintf("Below threshold in %s (%.0f%%)", s.Dimension, s.Score*100),
			})
		}
	}

	sort.Slice(strengths, func(i, j int) bool {
		return strengths[i].Score > strengths[j].Score
	})
	sort.Slice(weaknesses, func(i, j int) bool {
		return weaknesses[i].Score < weaknesses[j].Score
	})

	return ModelProfileReport{
		Scores:     scores,
		Strengths:  strengths,
		Weaknesses: weaknesses,
		Summary: ProfileSummary{
			ModelName:    modelName,
			TotalSamples: len(results),
			Dimensions:   len(scores),
			OverallScore: math.Round(overall*100) / 100,
			OverallGrade: floatGrade(overall),
			Strengths:    len(strengths),
			Weaknesses:   len(weaknesses),
		},
	}
}

func mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return math.Round(sum/float64(len(vals))*1000) / 1000
}

func floatGrade(score float64) string {
	switch {
	case score >= 0.9:
		return "A"
	case score >= 0.8:
		return "B"
	case score >= 0.7:
		return "C"
	case score >= 0.6:
		return "D"
	default:
		return "F"
	}
}

func FormatText(r ModelProfileReport) string {
	var sb strings.Builder
	sb.WriteString("Model Behavioral Profile\n")
	sb.WriteString(strings.Repeat("=", 50) + "\n\n")
	fmt.Fprintf(&sb, "  Model:           %s\n", r.Summary.ModelName)
	fmt.Fprintf(&sb, "  Samples:         %d\n", r.Summary.TotalSamples)
	fmt.Fprintf(&sb, "  Dimensions:      %d\n", r.Summary.Dimensions)
	fmt.Fprintf(&sb, "  Overall:         %.0f%% (%s)\n", r.Summary.OverallScore*100, r.Summary.OverallGrade)
	fmt.Fprintf(&sb, "  Strengths:       %d\n", r.Summary.Strengths)
	fmt.Fprintf(&sb, "  Weaknesses:      %d\n\n", r.Summary.Weaknesses)

	if len(r.Scores) > 0 {
		sb.WriteString("Dimension Scores:\n")
		for _, s := range r.Scores {
			bar := strings.Repeat("#", int(s.Score*20))
			fmt.Fprintf(&sb, "  %-20s %s %.0f%% (%s, n=%d)\n", s.Dimension, bar, s.Score*100, s.Grade, s.Samples)
		}
		sb.WriteString("\n")
	}

	if len(r.Strengths) > 0 {
		sb.WriteString("Strengths:\n")
		for _, s := range r.Strengths {
			fmt.Fprintf(&sb, "  + %s\n", s.Description)
		}
		sb.WriteString("\n")
	}

	if len(r.Weaknesses) > 0 {
		sb.WriteString("Weaknesses:\n")
		for _, w := range r.Weaknesses {
			fmt.Fprintf(&sb, "  - %s\n", w.Description)
		}
	}

	return sb.String()
}

func FormatJSON(r ModelProfileReport) string {
	b, _ := json.MarshalIndent(r, "", "  ")
	return string(b) + "\n"
}
