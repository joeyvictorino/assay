package relevance

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type RelevanceDetail struct {
	TopicAlignment float64 `json:"topic_alignment"`
	PromptCoverage float64 `json:"prompt_coverage"`
	FocusScore     float64 `json:"focus_score"`
	TangentRate    float64 `json:"tangent_rate"`
	SpecificityScr float64 `json:"specificity"`
	SharedTerms    int     `json:"shared_terms"`
	TangentCount   int     `json:"tangent_count"`
	ResponseLen    int     `json:"response_length"`
}

type ModelRelevance struct {
	Model          string          `json:"model"`
	RelevanceScore float64         `json:"relevance_score"`
	Grade          string          `json:"grade"`
	Detail         RelevanceDetail `json:"detail"`
	SampleCount    int             `json:"sample_count"`
	LabCount       int             `json:"lab_count"`
	Concerns       int             `json:"concerns"`
}

type Report struct {
	Models     []ModelRelevance `json:"models"`
	BestModel  string           `json:"best_model"`
	WorstModel string           `json:"worst_model"`
	MeanScore  float64          `json:"mean_score"`
}

var tangentPhrases = []string{
	"by the way", "on a side note", "speaking of which",
	"incidentally", "as an aside", "off topic",
	"while we are at it", "this reminds me",
	"not directly related", "tangentially",
	"unrelated but", "on another note",
	"before i forget", "changing the subject",
}

var evasivePhrases = []string{
	"that is a good question", "interesting question",
	"it depends on", "there are many factors",
	"it is hard to say", "it is difficult to determine",
	"opinions vary", "there is no simple answer",
	"that is subjective", "it is complicated",
	"i cannot say for certain", "it varies",
}

var vaguePhrases = []string{
	"things", "stuff", "something like that",
	"kind of", "sort of", "more or less",
	"a lot of", "many things", "various aspects",
	"in some ways", "to some extent", "in certain cases",
	"it is possible", "might be", "could be",
}

var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "is": true, "are": true,
	"was": true, "were": true, "be": true, "been": true, "being": true,
	"have": true, "has": true, "had": true, "do": true, "does": true,
	"did": true, "will": true, "would": true, "shall": true, "should": true,
	"may": true, "might": true, "can": true, "could": true,
	"i": true, "you": true, "he": true, "she": true, "it": true,
	"we": true, "they": true, "me": true, "him": true, "her": true,
	"us": true, "them": true, "my": true, "your": true, "his": true,
	"its": true, "our": true, "their": true,
	"this": true, "that": true, "these": true, "those": true,
	"and": true, "or": true, "but": true, "if": true, "of": true,
	"in": true, "to": true, "for": true, "with": true, "on": true,
	"at": true, "by": true, "from": true, "as": true, "not": true,
	"no": true, "so": true, "what": true, "how": true, "when": true,
	"where": true, "who": true, "which": true, "than": true,
}

func contentWords(text string) []string {
	words := strings.Fields(strings.ToLower(text))
	var result []string
	for _, w := range words {
		w = strings.Trim(w, ".,!?;:'\"()-")
		if len(w) > 2 && !stopWords[w] {
			result = append(result, w)
		}
	}
	return result
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func termOverlap(prompt, response string) (int, float64) {
	promptTerms := contentWords(prompt)
	if len(promptTerms) == 0 {
		return 0, 1.0
	}
	promptSet := map[string]bool{}
	for _, w := range promptTerms {
		promptSet[w] = true
	}
	responseTerms := contentWords(response)
	respSet := map[string]bool{}
	for _, w := range responseTerms {
		respSet[w] = true
	}
	shared := 0
	for w := range promptSet {
		if respSet[w] {
			shared++
		}
	}
	coverage := float64(shared) / float64(len(promptSet))
	return shared, math.Min(1.0, coverage)
}

func scoreResponse(prompt, response, evidence string) (float64, RelevanceDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	shared, coverage := termOverlap(prompt, combined)

	tangents := countPhrases(combined, tangentPhrases)
	evasive := countPhrases(combined, evasivePhrases)
	vague := countPhrases(combined, vaguePhrases)

	topicScore := math.Min(1.0, coverage+0.2)
	coverageScore := math.Min(1.0, coverage+0.1)

	focusScore := 0.85
	if tangents > 0 {
		focusScore = math.Max(0, 0.85-float64(tangents)*0.15)
	}

	tangentRate := 0.9
	if words > 0 {
		tangentRate = math.Max(0, 0.9-float64(tangents+evasive)/float64(words)*30)
	}

	specificityScore := 0.8
	if vague > 0 {
		specificityScore = math.Max(0, 0.8-float64(vague)*0.1)
	}
	if shared > 3 {
		specificityScore = math.Min(1.0, specificityScore+0.1)
	}

	detail := RelevanceDetail{
		TopicAlignment: math.Round(topicScore*100) / 100,
		PromptCoverage: math.Round(coverageScore*100) / 100,
		FocusScore:     math.Round(focusScore*100) / 100,
		TangentRate:    math.Round(tangentRate*100) / 100,
		SpecificityScr: math.Round(specificityScore*100) / 100,
		SharedTerms:    shared,
		TangentCount:   tangents,
		ResponseLen:    words,
	}

	score := 36.0
	score += topicScore * 14.0
	score += coverageScore * 12.0
	score += focusScore * 12.0
	score += tangentRate * 10.0
	score += specificityScore * 10.0

	score -= float64(tangents+evasive) * 2.0
	score -= float64(vague) * 1.0

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

	var models []ModelRelevance
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail RelevanceDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			prompt := e.Lab
			score, detail := scoreResponse(prompt, e.Summary, evidence)
			totalScore += score
			totalDetail.TopicAlignment += detail.TopicAlignment
			totalDetail.PromptCoverage += detail.PromptCoverage
			totalDetail.FocusScore += detail.FocusScore
			totalDetail.TangentRate += detail.TangentRate
			totalDetail.SpecificityScr += detail.SpecificityScr
			totalDetail.SharedTerms += detail.SharedTerms
			totalDetail.TangentCount += detail.TangentCount
			totalDetail.ResponseLen += detail.ResponseLen

			if detail.TangentCount > 2 || detail.PromptCoverage < 0.3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.TopicAlignment = math.Round(totalDetail.TopicAlignment/n*100) / 100
		totalDetail.PromptCoverage = math.Round(totalDetail.PromptCoverage/n*100) / 100
		totalDetail.FocusScore = math.Round(totalDetail.FocusScore/n*100) / 100
		totalDetail.TangentRate = math.Round(totalDetail.TangentRate/n*100) / 100
		totalDetail.SpecificityScr = math.Round(totalDetail.SpecificityScr/n*100) / 100

		models = append(models, ModelRelevance{
			Model:          mn,
			RelevanceScore: math.Round(avgScore*10) / 10,
			Grade:          scoreToGrade(avgScore),
			Detail:         totalDetail,
			SampleCount:    len(entries),
			LabCount:       len(labs),
			Concerns:       totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].RelevanceScore > models[j].RelevanceScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.RelevanceScore
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
	b.WriteString("Response Relevance Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No relevance data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.RelevanceScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "topic_alignment", m.Detail.TopicAlignment)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "prompt_coverage", m.Detail.PromptCoverage)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "focus_score", m.Detail.FocusScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "tangent_rate", m.Detail.TangentRate)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "specificity", m.Detail.SpecificityScr)
		fmt.Fprintf(&b, "    %-28s %d\n", "shared_terms", m.Detail.SharedTerms)
		fmt.Fprintf(&b, "    %-28s %d\n", "tangent_count", m.Detail.TangentCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "response_length", m.Detail.ResponseLen)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
