package promptcomplexity

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type ComplexityMetrics struct {
	TokenCount      int `json:"token_count"`
	SentenceCount   int `json:"sentence_count"`
	NestingDepth    int `json:"nesting_depth"`
	ConditionalCount int `json:"conditional_count"`
	ConstraintCount int `json:"constraint_count"`
	StepCount       int `json:"step_count"`
	QuestionCount   int `json:"question_count"`
	NegationCount   int `json:"negation_count"`
	ReferenceCount  int `json:"reference_count"`
}

type ComplexityReport struct {
	Prompt          string           `json:"prompt"`
	Metrics         ComplexityMetrics `json:"metrics"`
	Score           int              `json:"score"`
	Grade           string           `json:"grade"`
	Recommendations []string         `json:"recommendations"`
}

var (
	conditionalRe = regexp.MustCompile(`(?i)\b(if|when|unless|otherwise|else|elif|whether|provided that|in case)\b`)
	constraintRe  = regexp.MustCompile(`(?i)\b(must|shall|always|never|only|exactly|required|mandatory|prohibited|forbidden)\b`)
	stepRe        = regexp.MustCompile(`(?i)(?:^|\n)\s*(?:step\s+\d+|\d+[\.\)]\s)`)
	negationRe    = regexp.MustCompile(`(?i)\b(not|don't|doesn't|won't|can't|cannot|never|no|neither|nor|shouldn't|wouldn't)\b`)
	referenceRe   = regexp.MustCompile(`(?i)\b(above|below|previous|following|aforementioned|preceding|subsequent|earlier|later|respectively)\b`)
	sentenceEndRe = regexp.MustCompile(`[.!?]+`)
)

func Analyze(prompt string) ComplexityReport {
	if strings.TrimSpace(prompt) == "" {
		return ComplexityReport{
			Prompt: prompt,
			Score:  100,
			Grade:  "A",
		}
	}

	m := ComplexityMetrics{
		TokenCount:       len(strings.Fields(prompt)),
		SentenceCount:    countSentences(prompt),
		NestingDepth:     maxNesting(prompt),
		ConditionalCount: len(conditionalRe.FindAllString(prompt, -1)),
		ConstraintCount:  len(constraintRe.FindAllString(prompt, -1)),
		StepCount:        len(stepRe.FindAllString(prompt, -1)),
		QuestionCount:    strings.Count(prompt, "?"),
		NegationCount:    len(negationRe.FindAllString(prompt, -1)),
		ReferenceCount:   len(referenceRe.FindAllString(prompt, -1)),
	}

	score := computeScore(m)
	grade := scoreGrade(score)
	recs := recommendations(m)

	return ComplexityReport{
		Prompt:          prompt,
		Metrics:         m,
		Score:           score,
		Grade:           grade,
		Recommendations: recs,
	}
}

func countSentences(s string) int {
	parts := sentenceEndRe.Split(s, -1)
	count := 0
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			count++
		}
	}
	if count == 0 {
		count = 1
	}
	return count
}

func maxNesting(s string) int {
	max, cur := 0, 0
	for _, c := range s {
		switch c {
		case '(', '[', '{':
			cur++
			if cur > max {
				max = cur
			}
		case ')', ']', '}':
			if cur > 0 {
				cur--
			}
		}
	}
	return max
}

func computeScore(m ComplexityMetrics) int {
	raw := 0.0
	if m.TokenCount > 50 {
		raw += float64(m.TokenCount-50) * 0.1
	}
	raw += float64(m.NestingDepth) * 5
	raw += float64(m.ConditionalCount) * 4
	raw += float64(m.ConstraintCount) * 3
	raw += float64(m.StepCount) * 2
	raw += float64(m.QuestionCount) * 2
	raw += float64(m.NegationCount) * 3
	raw += float64(m.ReferenceCount) * 2

	score := int(raw)
	if score > 100 {
		score = 100
	}
	return score
}

func scoreGrade(score int) string {
	switch {
	case score < 20:
		return "A"
	case score < 40:
		return "B"
	case score < 60:
		return "C"
	case score < 80:
		return "D"
	default:
		return "F"
	}
}

func recommendations(m ComplexityMetrics) []string {
	var recs []string
	if m.TokenCount > 200 {
		recs = append(recs, "Consider breaking this prompt into smaller, focused prompts")
	}
	if m.NestingDepth > 3 {
		recs = append(recs, "Reduce nesting depth for clarity")
	}
	if m.ConditionalCount > 5 {
		recs = append(recs, "Too many conditionals; consider a decision table or structured format")
	}
	if m.NegationCount > 5 {
		recs = append(recs, "High negation count; rephrase constraints in positive terms")
	}
	if m.ConstraintCount > 8 {
		recs = append(recs, "Many constraints detected; group related constraints together")
	}
	return recs
}

func FormatText(r ComplexityReport) string {
	var sb strings.Builder
	sb.WriteString("Prompt Complexity Analysis\n")
	sb.WriteString(strings.Repeat("=", 50) + "\n\n")
	fmt.Fprintf(&sb, "  Tokens:        %d\n", r.Metrics.TokenCount)
	fmt.Fprintf(&sb, "  Sentences:     %d\n", r.Metrics.SentenceCount)
	fmt.Fprintf(&sb, "  Nesting depth: %d\n", r.Metrics.NestingDepth)
	fmt.Fprintf(&sb, "  Conditionals:  %d\n", r.Metrics.ConditionalCount)
	fmt.Fprintf(&sb, "  Constraints:   %d\n", r.Metrics.ConstraintCount)
	fmt.Fprintf(&sb, "  Steps:         %d\n", r.Metrics.StepCount)
	fmt.Fprintf(&sb, "  Questions:     %d\n", r.Metrics.QuestionCount)
	fmt.Fprintf(&sb, "  Negations:     %d\n", r.Metrics.NegationCount)
	fmt.Fprintf(&sb, "  References:    %d\n", r.Metrics.ReferenceCount)
	fmt.Fprintf(&sb, "\n  Score: %d/100 (%s)\n", r.Score, r.Grade)

	if len(r.Recommendations) > 0 {
		sb.WriteString("\nRecommendations:\n")
		for i, rec := range r.Recommendations {
			fmt.Fprintf(&sb, "  %d. %s\n", i+1, rec)
		}
	}

	return sb.String()
}

func FormatJSON(r ComplexityReport) string {
	b, _ := json.MarshalIndent(r, "", "  ")
	return string(b) + "\n"
}
