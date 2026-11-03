package answercompleteness

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type CompletenessDetail struct {
	SubQuestionsCovered int     `json:"sub_questions_covered"`
	SubQuestionsTotal   int     `json:"sub_questions_total"`
	CoverageRatio       float64 `json:"coverage_ratio"`
	MissingAspects      int     `json:"missing_aspects"`
	PartialAnswers      int     `json:"partial_answers"`
	FullAnswers         int     `json:"full_answers"`
}

type ModelCompleteness struct {
	Model            string             `json:"model"`
	CompletenessScore float64           `json:"completeness_score"`
	Grade            string             `json:"grade"`
	Detail           CompletenessDetail `json:"detail"`
	SampleCount      int                `json:"sample_count"`
	LabCount         int                `json:"lab_count"`
	FullyComplete    int                `json:"fully_complete"`
	PartialCount     int                `json:"partial_count"`
	MissingCount     int                `json:"missing_count"`
}

type Report struct {
	Models         []ModelCompleteness `json:"models"`
	MostComplete   string              `json:"most_complete"`
	LeastComplete  string              `json:"least_complete"`
	MeanScore      float64             `json:"mean_score"`
}

var questionIndicators = []string{"?", "how", "what", "why", "when", "where", "which", "who",
	"explain", "describe", "compare", "contrast", "list", "enumerate", "define",
	"analyze", "evaluate", "discuss", "summarize", "outline"}

var conjunctions = []string{" and ", " also ", " additionally ", " furthermore ",
	" moreover ", " plus ", " as well as "}

func countSubQuestions(prompt string) int {
	lower := strings.ToLower(prompt)
	count := 0

	qmarks := strings.Count(lower, "?")
	if qmarks > 1 {
		return qmarks
	}

	for _, conj := range conjunctions {
		count += strings.Count(lower, conj)
	}

	numbered := 0
	for _, line := range strings.Split(lower, "\n") {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 2 && trimmed[0] >= '1' && trimmed[0] <= '9' && (trimmed[1] == '.' || trimmed[1] == ')') {
			numbered++
		}
	}
	if numbered > count {
		count = numbered
	}

	if count == 0 {
		indicators := 0
		for _, ind := range questionIndicators {
			if strings.Contains(lower, ind) {
				indicators++
			}
		}
		if indicators >= 3 {
			count = 2
		} else {
			count = 1
		}
	} else {
		count++
	}

	return count
}

func extractKeyAspects(prompt string) []string {
	lower := strings.ToLower(prompt)
	var aspects []string

	for _, line := range strings.Split(lower, "\n") {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 2 && trimmed[0] >= '1' && trimmed[0] <= '9' && (trimmed[1] == '.' || trimmed[1] == ')') {
			aspects = append(aspects, strings.TrimSpace(trimmed[2:]))
		}
	}

	if len(aspects) == 0 {
		parts := strings.Split(lower, "?")
		for _, p := range parts {
			t := strings.TrimSpace(p)
			if len(t) > 3 {
				aspects = append(aspects, t)
			}
		}
	}

	if len(aspects) == 0 {
		for _, conj := range conjunctions {
			if strings.Contains(lower, conj) {
				parts := strings.SplitN(lower, conj, 2)
				for _, p := range parts {
					t := strings.TrimSpace(p)
					if len(t) > 3 {
						aspects = append(aspects, t)
					}
				}
				break
			}
		}
	}

	if len(aspects) == 0 {
		aspects = append(aspects, strings.TrimSpace(lower))
	}

	return aspects
}

func extractKeywords(text string) map[string]bool {
	stopWords := map[string]bool{
		"a": true, "an": true, "the": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "to": true,
		"of": true, "in": true, "for": true, "on": true, "with": true,
		"at": true, "by": true, "from": true, "and": true, "or": true,
		"it": true, "this": true, "that": true, "i": true, "you": true,
		"we": true, "they": true, "do": true, "does": true, "did": true,
		"not": true, "but": true, "so": true, "if": true, "as": true,
		"what": true, "how": true, "why": true, "when": true, "where": true,
		"which": true, "who": true, "would": true, "could": true, "should": true,
		"can": true, "will": true, "has": true, "had": true, "have": true,
	}
	words := strings.Fields(strings.ToLower(text))
	kw := map[string]bool{}
	for _, w := range words {
		clean := strings.Trim(w, ".,;:!?\"'()[]{}")
		if len(clean) > 2 && !stopWords[clean] {
			kw[clean] = true
		}
	}
	return kw
}

func aspectCoverage(aspect string, response string) float64 {
	aspectKW := extractKeywords(aspect)
	responseKW := extractKeywords(response)
	if len(aspectKW) == 0 {
		return 100
	}
	covered := 0
	for kw := range aspectKW {
		if responseKW[kw] {
			covered++
		}
	}
	return float64(covered) / float64(len(aspectKW)) * 100
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

	type entry struct {
		lab     string
		summary string
		prompt  string
	}
	modelEntries := map[string][]entry{}
	for _, f := range findings {
		mn := f.Model.Model
		if mn == "" {
			mn = "unknown"
		}
		prompt := ""
		if f.Location.PathTemplate != "" {
			prompt = f.Location.PathTemplate
		}
		modelEntries[mn] = append(modelEntries[mn], entry{
			lab:     f.Lab,
			summary: f.Summary,
			prompt:  prompt,
		})
	}

	var models []ModelCompleteness
	for mn, entries := range modelEntries {
		totalSubQ := 0
		coveredSubQ := 0
		missing := 0
		partial := 0
		full := 0
		fullyComplete := 0

		byLab := map[string][]entry{}
		for _, e := range entries {
			byLab[e.lab] = append(byLab[e.lab], e)
		}

		for _, labEntries := range byLab {
			for _, e := range labEntries {
				prompt := e.prompt
				if prompt == "" {
					prompt = e.lab
				}
				subQ := countSubQuestions(prompt)
				aspects := extractKeyAspects(prompt)
				totalSubQ += subQ

				aspectsCovered := 0
				aspectsPartial := 0
				for _, asp := range aspects {
					cov := aspectCoverage(asp, e.summary)
					if cov >= 60 {
						aspectsCovered++
					} else if cov >= 25 {
						aspectsPartial++
					}
				}

				ratio := float64(aspectsCovered) / float64(max(len(aspects), 1))
				if ratio >= 0.9 {
					fullyComplete++
					full++
					coveredSubQ += subQ
				} else if ratio >= 0.5 || aspectsPartial > 0 {
					partial++
					coveredSubQ += int(float64(subQ) * ratio)
				} else {
					missing++
				}
			}
		}

		totalEntries := len(entries)
		var coverageRatio float64
		if totalSubQ > 0 {
			coverageRatio = float64(coveredSubQ) / float64(totalSubQ)
		}

		score := 0.0
		if totalEntries > 0 {
			fullRatio := float64(full) / float64(totalEntries)
			partialRatio := float64(partial) / float64(totalEntries)
			score = (fullRatio*100*0.6 + partialRatio*50*0.2 + coverageRatio*100*0.2)
		}

		models = append(models, ModelCompleteness{
			Model:             mn,
			CompletenessScore: math.Round(score*10) / 10,
			Grade:             scoreToGrade(score),
			Detail: CompletenessDetail{
				SubQuestionsCovered: coveredSubQ,
				SubQuestionsTotal:   totalSubQ,
				CoverageRatio:       math.Round(coverageRatio*1000) / 1000,
				MissingAspects:      missing,
				PartialAnswers:      partial,
				FullAnswers:         full,
			},
			SampleCount:   totalEntries,
			LabCount:       len(byLab),
			FullyComplete: fullyComplete,
			PartialCount:  partial,
			MissingCount:  missing,
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Answer Completeness Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No completeness data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Most complete:        %s\n", r.MostComplete)
	fmt.Fprintf(&b, "  Least complete:       %s\n\n", r.LeastComplete)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.CompletenessScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %d / %d\n", "sub_questions_covered",
			m.Detail.SubQuestionsCovered, m.Detail.SubQuestionsTotal)
		fmt.Fprintf(&b, "    %-28s %.3f\n", "coverage_ratio", m.Detail.CoverageRatio)
		fmt.Fprintf(&b, "    %-28s %d\n", "full_answers", m.Detail.FullAnswers)
		fmt.Fprintf(&b, "    %-28s %d\n", "partial_answers", m.Detail.PartialAnswers)
		fmt.Fprintf(&b, "    %-28s %d\n", "missing_aspects", m.Detail.MissingAspects)
		fmt.Fprintf(&b, "    Fully complete: %d  Partial: %d  Missing: %d\n",
			m.FullyComplete, m.PartialCount, m.MissingCount)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
