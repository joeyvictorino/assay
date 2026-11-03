package focus

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type FocusDetail struct {
	TangentCount       int     `json:"tangent_count"`
	UnsolicitedAdvice  int     `json:"unsolicited_advice"`
	TopicDriftScore    float64 `json:"topic_drift_score"`
	CaveatCount        int     `json:"caveat_count"`
	BoilerplateCount   int     `json:"boilerplate_count"`
	OnTopicRatio       float64 `json:"on_topic_ratio"`
	PromptCoverage     float64 `json:"prompt_coverage"`
	DigressionPenalty  float64 `json:"digression_penalty"`
}

type ModelFocus struct {
	Model      string      `json:"model"`
	FocusScore float64     `json:"focus_score"`
	Grade      string      `json:"grade"`
	Detail     FocusDetail `json:"detail"`
	SampleCount int        `json:"sample_count"`
	LabCount    int        `json:"lab_count"`
	Concerns    int        `json:"concerns"`
}

type Report struct {
	Models     []ModelFocus `json:"models"`
	BestModel  string       `json:"best_model"`
	WorstModel string       `json:"worst_model"`
	MeanScore  float64      `json:"mean_score"`
}

var tangentPhrases = []string{
	"speaking of which",
	"on a related note",
	"by the way",
	"incidentally",
	"as an aside",
	"on a side note",
	"tangentially",
	"while we're on the subject",
	"this reminds me",
	"fun fact",
	"interestingly enough",
	"off topic but",
	"unrelated but",
}

var unsolicitedAdvicePhrases = []string{
	"you might also want to",
	"additionally you could",
	"you should also consider",
	"it might be worth",
	"you may also want to",
	"another thing to consider",
	"while you're at it",
	"i would also recommend",
	"don't forget to",
	"you'll also want to",
	"a bonus tip",
	"pro tip",
	"as a best practice",
}

var caveatPhrases = []string{
	"however it's worth noting",
	"that being said",
	"with that caveat",
	"keep in mind that",
	"bear in mind",
	"it's worth noting",
	"one thing to be aware of",
	"a word of caution",
	"disclaimer",
	"please note that",
	"important caveat",
	"worth mentioning",
}

var boilerplatePhrases = []string{
	"hope this helps",
	"let me know if you have any",
	"feel free to ask",
	"don't hesitate to",
	"happy to help further",
	"if you need more information",
	"i hope this answers your question",
	"please let me know if",
	"is there anything else",
	"glad i could help",
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
		"may": true, "might": true, "shall": true, "can": true,
		"i": true, "you": true, "he": true, "she": true, "it": true,
		"we": true, "they": true, "me": true, "him": true, "her": true,
		"us": true, "them": true, "my": true, "your": true, "his": true,
		"its": true, "our": true, "their": true,
		"this": true, "that": true, "these": true, "those": true,
		"in": true, "on": true, "at": true, "to": true, "for": true,
		"of": true, "with": true, "by": true, "from": true, "about": true,
		"and": true, "or": true, "but": true, "not": true, "if": true,
		"what": true, "how": true, "when": true, "where": true, "why": true,
		"who": true, "which": true,
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

func computePromptCoverage(prompt, response string) float64 {
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

func scoreResponse(prompt, response, evidence string) (float64, FocusDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))
	if words == 0 {
		return 85.0, FocusDetail{OnTopicRatio: 1.0, PromptCoverage: 1.0}
	}

	tangents := countPhrases(combined, tangentPhrases)
	unsolicited := countPhrases(combined, unsolicitedAdvicePhrases)
	caveats := countPhrases(combined, caveatPhrases)
	boilerplate := countPhrases(combined, boilerplatePhrases)

	digressionWords := tangents*6 + unsolicited*5 + caveats*4 + boilerplate*5
	onTopicWords := words - digressionWords
	if onTopicWords < 0 {
		onTopicWords = 0
	}
	onTopicRatio := float64(onTopicWords) / float64(words)

	promptCoverage := computePromptCoverage(prompt, response)

	topicDriftScore := 1.0 - onTopicRatio

	digressionPenalty := float64(tangents)*6.0 + float64(unsolicited)*5.0 + float64(boilerplate)*4.0 + float64(caveats)*2.0

	detail := FocusDetail{
		TangentCount:      tangents,
		UnsolicitedAdvice: unsolicited,
		TopicDriftScore:   math.Round(topicDriftScore*100) / 100,
		CaveatCount:       caveats,
		BoilerplateCount:  boilerplate,
		OnTopicRatio:      math.Round(onTopicRatio*100) / 100,
		PromptCoverage:    math.Round(promptCoverage*100) / 100,
		DigressionPenalty: math.Round(digressionPenalty*10) / 10,
	}

	score := 40.0
	score += onTopicRatio * 25.0
	score += promptCoverage * 20.0
	score -= digressionPenalty

	if tangents == 0 && unsolicited == 0 && boilerplate == 0 {
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

	var models []ModelFocus
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail FocusDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			prompt := e.Location.PathTemplate
			score, detail := scoreResponse(prompt, e.Summary, evidence)
			totalScore += score
			totalDetail.TangentCount += detail.TangentCount
			totalDetail.UnsolicitedAdvice += detail.UnsolicitedAdvice
			totalDetail.TopicDriftScore += detail.TopicDriftScore
			totalDetail.CaveatCount += detail.CaveatCount
			totalDetail.BoilerplateCount += detail.BoilerplateCount
			totalDetail.OnTopicRatio += detail.OnTopicRatio
			totalDetail.PromptCoverage += detail.PromptCoverage
			totalDetail.DigressionPenalty += detail.DigressionPenalty

			if detail.OnTopicRatio < 0.7 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.TopicDriftScore = math.Round(totalDetail.TopicDriftScore/n*100) / 100
		totalDetail.OnTopicRatio = math.Round(totalDetail.OnTopicRatio/n*100) / 100
		totalDetail.PromptCoverage = math.Round(totalDetail.PromptCoverage/n*100) / 100
		totalDetail.DigressionPenalty = math.Round(totalDetail.DigressionPenalty/n*10) / 10

		models = append(models, ModelFocus{
			Model:       mn,
			FocusScore:  math.Round(avgScore*10) / 10,
			Grade:       scoreToGrade(avgScore),
			Detail:      totalDetail,
			SampleCount: len(entries),
			LabCount:    len(labs),
			Concerns:    totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].FocusScore > models[j].FocusScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.FocusScore
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
	b.WriteString("Response Focus Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No focus data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.FocusScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "tangent_count", m.Detail.TangentCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "unsolicited_advice", m.Detail.UnsolicitedAdvice)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "topic_drift_score", m.Detail.TopicDriftScore)
		fmt.Fprintf(&b, "    %-28s %d\n", "caveat_count", m.Detail.CaveatCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "boilerplate_count", m.Detail.BoilerplateCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "on_topic_ratio", m.Detail.OnTopicRatio)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "prompt_coverage", m.Detail.PromptCoverage)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "digression_penalty", m.Detail.DigressionPenalty)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
