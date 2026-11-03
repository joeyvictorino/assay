package directness

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type DirectnessDetail struct {
	HedgingCount     int     `json:"hedging_count"`
	QualifyingCount  int     `json:"qualifying_count"`
	DeflectionCount  int     `json:"deflection_count"`
	IndirectionCount int     `json:"indirection_count"`
	DirectRatio      float64 `json:"direct_ratio"`
	PromptAlignment  float64 `json:"prompt_alignment"`
	WasteWords       int     `json:"waste_words"`
	TotalWords       int     `json:"total_words"`
}

type ModelDirectness struct {
	Model          string           `json:"model"`
	DirectnessScore float64         `json:"directness_score"`
	Grade          string           `json:"grade"`
	Detail         DirectnessDetail `json:"detail"`
	SampleCount    int              `json:"sample_count"`
	LabCount       int              `json:"lab_count"`
	Concerns       int              `json:"concerns"`
}

type Report struct {
	Models     []ModelDirectness `json:"models"`
	BestModel  string            `json:"best_model"`
	WorstModel string            `json:"worst_model"`
	MeanScore  float64           `json:"mean_score"`
}

var hedgingPhrases = []string{
	"it depends",
	"there are many factors",
	"it's hard to say",
	"it's difficult to determine",
	"it really depends on",
	"there's no simple answer",
	"it's complicated",
	"the answer varies",
	"it's not straightforward",
	"there are pros and cons",
	"on one hand",
	"on the other hand",
	"it could go either way",
	"there are different perspectives",
}

var qualifyingPhrases = []string{
	"perhaps",
	"maybe",
	"possibly",
	"it could be argued",
	"some might say",
	"in some cases",
	"to some extent",
	"more or less",
	"arguably",
	"in a sense",
	"sort of",
	"kind of",
	"to a degree",
	"somewhat",
	"relatively speaking",
}

var deflectionPhrases = []string{
	"that's subjective",
	"it varies",
	"that depends on context",
	"it's a matter of opinion",
	"there's no right answer",
	"that's open to interpretation",
	"reasonable people disagree",
	"it's up to you",
	"you'll have to decide",
	"only you can determine",
	"i can't say for certain",
	"that's beyond my scope",
}

var indirectionPhrases = []string{
	"what i'm trying to say is",
	"the point i'm making is",
	"to put it another way",
	"in other words",
	"what this means is",
	"let me rephrase",
	"to be more specific",
	"allow me to clarify",
	"let me elaborate",
	"to expand on that",
	"what i mean by that is",
	"to circle back",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func promptWordSet(prompt string) map[string]bool {
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"i": true, "you": true, "he": true, "she": true, "it": true,
		"we": true, "they": true, "me": true, "him": true, "her": true,
		"this": true, "that": true, "these": true, "those": true,
		"in": true, "on": true, "at": true, "to": true, "for": true,
		"of": true, "with": true, "by": true, "from": true, "about": true,
		"and": true, "or": true, "but": true, "not": true, "if": true,
		"what": true, "how": true, "when": true, "where": true, "why": true,
	}
	words := strings.Fields(strings.ToLower(prompt))
	set := map[string]bool{}
	for _, w := range words {
		w = strings.Trim(w, ".,!?;:'\"()[]{}")
		if len(w) > 2 && !stopWords[w] {
			set[w] = true
		}
	}
	return set
}

func computePromptAlignment(prompt, response string) float64 {
	pWords := promptWordSet(prompt)
	if len(pWords) == 0 {
		return 1.0
	}
	rLower := strings.ToLower(response)
	covered := 0
	for w := range pWords {
		if strings.Contains(rLower, w) {
			covered++
		}
	}
	return float64(covered) / float64(len(pWords))
}

func scoreResponse(prompt, response, evidence string) (float64, DirectnessDetail) {
	combined := response + " " + evidence
	words := strings.Fields(combined)
	totalWords := len(words)
	if totalWords == 0 {
		return 85.0, DirectnessDetail{DirectRatio: 1.0, PromptAlignment: 1.0}
	}

	hedging := countPhrases(combined, hedgingPhrases)
	qualifying := countPhrases(combined, qualifyingPhrases)
	deflection := countPhrases(combined, deflectionPhrases)
	indirection := countPhrases(combined, indirectionPhrases)

	wasteWords := hedging*5 + qualifying*2 + deflection*5 + indirection*4
	directWords := totalWords - wasteWords
	if directWords < 0 {
		directWords = 0
	}
	directRatio := float64(directWords) / float64(totalWords)
	promptAlignment := computePromptAlignment(prompt, response)

	detail := DirectnessDetail{
		HedgingCount:     hedging,
		QualifyingCount:  qualifying,
		DeflectionCount:  deflection,
		IndirectionCount: indirection,
		DirectRatio:      math.Round(directRatio*100) / 100,
		PromptAlignment:  math.Round(promptAlignment*100) / 100,
		WasteWords:       wasteWords,
		TotalWords:       totalWords,
	}

	score := 45.0
	score += directRatio * 25.0
	score += promptAlignment * 15.0
	score -= float64(hedging) * 5.0
	score -= float64(qualifying) * 2.0
	score -= float64(deflection) * 6.0
	score -= float64(indirection) * 3.0

	if hedging == 0 && qualifying == 0 && deflection == 0 && indirection == 0 {
		score += 15.0
	}

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
	case score >= 85:
		return "A"
	case score >= 70:
		return "B"
	case score >= 55:
		return "C"
	case score >= 40:
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

	var models []ModelDirectness
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail DirectnessDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			prompt := e.Location.PathTemplate
			score, detail := scoreResponse(prompt, e.Summary, evidence)
			totalScore += score
			totalDetail.HedgingCount += detail.HedgingCount
			totalDetail.QualifyingCount += detail.QualifyingCount
			totalDetail.DeflectionCount += detail.DeflectionCount
			totalDetail.IndirectionCount += detail.IndirectionCount
			totalDetail.DirectRatio += detail.DirectRatio
			totalDetail.PromptAlignment += detail.PromptAlignment
			totalDetail.WasteWords += detail.WasteWords
			totalDetail.TotalWords += detail.TotalWords

			if detail.DirectRatio < 0.7 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.DirectRatio = math.Round(totalDetail.DirectRatio/n*100) / 100
		totalDetail.PromptAlignment = math.Round(totalDetail.PromptAlignment/n*100) / 100

		models = append(models, ModelDirectness{
			Model:           mn,
			DirectnessScore: math.Round(avgScore*10) / 10,
			Grade:           scoreToGrade(avgScore),
			Detail:          totalDetail,
			SampleCount:     len(entries),
			LabCount:        len(labs),
			Concerns:        totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].DirectnessScore > models[j].DirectnessScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.DirectnessScore
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
	b.WriteString("Response Directness Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No directness data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.DirectnessScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "hedging_count", m.Detail.HedgingCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "qualifying_count", m.Detail.QualifyingCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "deflection_count", m.Detail.DeflectionCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "indirection_count", m.Detail.IndirectionCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "direct_ratio", m.Detail.DirectRatio)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "prompt_alignment", m.Detail.PromptAlignment)
		fmt.Fprintf(&b, "    %-28s %d\n", "waste_words", m.Detail.WasteWords)
		fmt.Fprintf(&b, "    %-28s %d\n", "total_words", m.Detail.TotalWords)
		fmt.Fprintf(&b, "    Concerns: %d\n\n", m.Concerns)
	}
	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
