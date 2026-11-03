package chainquality

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

type DimensionScore struct {
	Name  string  `json:"name"`
	Score float64 `json:"score"`
	Grade string  `json:"grade"`
	Notes string  `json:"notes"`
}

type ChainQualityReport struct {
	OverallScore float64          `json:"overall_score"`
	Grade        string           `json:"grade"`
	Dimensions   []DimensionScore `json:"dimensions"`
	WeakestLink  string           `json:"weakest_link"`
	StepCount    int              `json:"step_count"`
}

type BatchReport struct {
	Results  []ChainQualityReport `json:"results"`
	MeanScore float64             `json:"mean_score"`
	BestGrade string              `json:"best_grade"`
	WorstGrade string             `json:"worst_grade"`
}

var transitionWords = []string{
	"therefore", "because", "since", "thus", "hence",
	"consequently", "so", "as a result", "it follows",
	"this means", "which means", "given that", "due to",
	"first", "second", "third", "next", "then", "finally",
	"step", "moreover", "furthermore", "additionally",
	"in conclusion", "to summarize", "in summary",
}

var evidenceMarkers = []string{
	"according to", "based on", "as stated", "the data shows",
	"evidence suggests", "from the", "as mentioned",
	"the context", "the provided", "given the",
	"the document", "the text", "the passage",
	"we can see", "it states", "it says",
}

func countSteps(text string) int {
	lines := strings.Split(text, "\n")
	steps := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if len(trimmed) > 2 && trimmed[0] >= '1' && trimmed[0] <= '9' && (trimmed[1] == '.' || trimmed[1] == ')') {
			steps++
			continue
		}
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			steps++
			continue
		}
	}
	if steps == 0 {
		sentences := strings.Count(text, ".") + strings.Count(text, "!") + strings.Count(text, "?")
		return max(1, sentences)
	}
	return max(1, steps)
}

func scoreCompleteness(text string) float64 {
	steps := countSteps(text)
	words := len(strings.Fields(text))

	hasConcl := false
	lower := strings.ToLower(text)
	for _, marker := range []string{"therefore", "in conclusion", "thus", "so the answer", "the result"} {
		if strings.Contains(lower, marker) {
			hasConcl = true
			break
		}
	}

	score := 50.0
	if steps >= 3 {
		score += 20
	} else if steps >= 2 {
		score += 10
	}
	if words >= 50 {
		score += 15
	} else if words >= 20 {
		score += 8
	}
	if hasConcl {
		score += 15
	}

	return clamp(score, 0, 100)
}

func scoreLogicalConsistency(text string) float64 {
	lower := strings.ToLower(text)
	transitions := 0
	for _, tw := range transitionWords {
		transitions += strings.Count(lower, tw)
	}

	contradictions := 0
	for _, marker := range []string{"however", "but", "on the other hand", "contradicts", "inconsistent"} {
		contradictions += strings.Count(lower, marker)
	}

	words := len(strings.Fields(text))
	if words == 0 {
		return 50
	}

	transitionDensity := float64(transitions) / float64(words) * 100
	score := 50.0
	if transitionDensity >= 2 {
		score += 30
	} else if transitionDensity >= 1 {
		score += 20
	} else if transitionDensity >= 0.5 {
		score += 10
	}

	if contradictions > 2 {
		score -= 20
	} else if contradictions > 0 {
		score -= 5
	}

	sentences := max(1, strings.Count(text, ".")+strings.Count(text, "!")+strings.Count(text, "?"))
	if sentences > 1 {
		score += 10
	}

	return clamp(score, 0, 100)
}

func scoreEvidenceGrounding(text string) float64 {
	lower := strings.ToLower(text)
	evidenceCount := 0
	for _, em := range evidenceMarkers {
		evidenceCount += strings.Count(lower, em)
	}

	sentences := max(1, strings.Count(text, ".")+strings.Count(text, "!")+strings.Count(text, "?"))
	ratio := float64(evidenceCount) / float64(sentences) * 100

	score := 40.0
	if ratio >= 30 {
		score += 40
	} else if ratio >= 15 {
		score += 25
	} else if ratio >= 5 {
		score += 15
	}

	if evidenceCount >= 3 {
		score += 20
	} else if evidenceCount >= 1 {
		score += 10
	}

	return clamp(score, 0, 100)
}

func gradeFromScore(score float64) string {
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

func Analyze(reasoningText string) ChainQualityReport {
	completeness := scoreCompleteness(reasoningText)
	consistency := scoreLogicalConsistency(reasoningText)
	grounding := scoreEvidenceGrounding(reasoningText)

	dims := []DimensionScore{
		{Name: "reasoning_completeness", Score: completeness, Grade: gradeFromScore(completeness)},
		{Name: "logical_consistency", Score: consistency, Grade: gradeFromScore(consistency)},
		{Name: "evidence_grounding", Score: grounding, Grade: gradeFromScore(grounding)},
	}

	overall := (completeness + consistency + grounding) / 3.0
	overall = math.Round(overall*100) / 100

	weakest := dims[0].Name
	weakestScore := dims[0].Score
	for _, d := range dims[1:] {
		if d.Score < weakestScore {
			weakestScore = d.Score
			weakest = d.Name
		}
	}

	return ChainQualityReport{
		OverallScore: overall,
		Grade:        gradeFromScore(overall),
		Dimensions:   dims,
		WeakestLink:  weakest,
		StepCount:    countSteps(reasoningText),
	}
}

func AnalyzeBatch(texts []string) BatchReport {
	br := BatchReport{}
	var total float64
	bestScore := 0.0
	worstScore := 100.0
	for _, t := range texts {
		r := Analyze(t)
		br.Results = append(br.Results, r)
		total += r.OverallScore
		if r.OverallScore > bestScore {
			bestScore = r.OverallScore
		}
		if r.OverallScore < worstScore {
			worstScore = r.OverallScore
		}
	}
	if len(texts) > 0 {
		br.MeanScore = total / float64(len(texts))
		br.BestGrade = gradeFromScore(bestScore)
		br.WorstGrade = gradeFromScore(worstScore)
	}
	return br
}

func FormatText(r ChainQualityReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Chain-of-Thought Quality Report\n")
	fmt.Fprintf(&b, "==============================\n\n")
	fmt.Fprintf(&b, "Overall Score: %.1f (%s)\n", r.OverallScore, r.Grade)
	fmt.Fprintf(&b, "Steps Found:   %d\n", r.StepCount)
	fmt.Fprintf(&b, "Weakest Link:  %s\n\n", r.WeakestLink)
	for _, d := range r.Dimensions {
		fmt.Fprintf(&b, "  %-25s  %.1f  %s\n", d.Name, d.Score, d.Grade)
	}
	return b.String()
}

func FormatJSON(r ChainQualityReport) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data)
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
