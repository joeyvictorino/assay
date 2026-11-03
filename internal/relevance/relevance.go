package relevance

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type RelevanceDetail struct {
	OnTopicCount    int     `json:"on_topic_count"`
	OffTopicCount   int     `json:"off_topic_count"`
	DigressionCount int     `json:"digression_count"`
	FocusCount      int     `json:"focus_count"`
	FillerCount     int     `json:"filler_count"`
	WordCount       int     `json:"word_count"`
	FocusRatio      float64 `json:"focus_ratio"`
}

type ModelRelevance struct {
	Model       string          `json:"model"`
	Score       float64         `json:"score"`
	Grade       string          `json:"grade"`
	Detail      RelevanceDetail `json:"detail"`
	SampleCount int             `json:"sample_count"`
	LabCount    int             `json:"lab_count"`
}

type Report struct {
	Models     []ModelRelevance `json:"models"`
	BestModel  string           `json:"best_model"`
	WorstModel string           `json:"worst_model"`
	MeanScore  float64          `json:"mean_score"`
}

var onTopicPhrases = []string{
	"to answer your question", "regarding your query", "in response to",
	"the issue is", "the root cause", "the solution is",
	"to fix this", "the problem is", "this resolves",
	"the answer is", "specifically", "directly addresses",
	"relevant to", "pertaining to", "with respect to",
}

var offTopicPhrases = []string{
	"by the way", "on a side note", "unrelated but",
	"speaking of which", "incidentally", "as an aside",
	"tangentially", "off topic but", "while we're at it",
	"not directly related", "on another note", "changing the subject",
}

var digressionPhrases = []string{
	"let me tell you about", "fun fact", "did you know",
	"interesting trivia", "historically speaking", "as a matter of fact",
	"i should mention", "it reminds me of", "this is similar to",
	"going back to", "before i forget", "while i'm at it",
}

var focusPhrases = []string{
	"to summarize", "in short", "the key point is",
	"the main issue", "in conclusion", "bottom line",
	"to be precise", "the core problem", "fundamentally",
	"the critical thing", "most importantly", "the crux is",
}

var fillerPhrases = []string{
	"you know", "like i said", "as i mentioned",
	"needless to say", "it goes without saying", "obviously",
	"of course", "as you can see", "clearly",
	"that being said", "having said that", "at the end of the day",
	"long story short", "to make a long story short",
}

var questionRefPattern = regexp.MustCompile(`(?i)\b(your question|you asked|you mentioned|your request|you wanted)\b`)

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func AnalyzeText(text string) (float64, RelevanceDetail) {
	words := strings.Fields(text)
	wordCount := len(words)

	if wordCount == 0 {
		return 50.0, RelevanceDetail{}
	}

	onTopic := countPhrases(text, onTopicPhrases)
	offTopic := countPhrases(text, offTopicPhrases)
	digressions := countPhrases(text, digressionPhrases)
	focus := countPhrases(text, focusPhrases)
	filler := countPhrases(text, fillerPhrases)
	questionRefs := len(questionRefPattern.FindAllString(text, -1))

	onTopic += questionRefs

	positiveSignals := float64(onTopic + focus)
	negativeSignals := float64(offTopic + digressions + filler)
	totalSignals := positiveSignals + negativeSignals

	focusRatio := 0.5
	if totalSignals > 0 {
		focusRatio = positiveSignals / totalSignals
	}

	score := 50.0
	score += float64(onTopic) * 4.0
	score += float64(focus) * 3.5
	score += float64(questionRefs) * 2.0
	score -= float64(offTopic) * 5.0
	score -= float64(digressions) * 4.0
	score -= float64(filler) * 3.0

	density := positiveSignals / float64(wordCount) * 100
	score += density * 2.0

	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	detail := RelevanceDetail{
		OnTopicCount:    onTopic,
		OffTopicCount:   offTopic,
		DigressionCount: digressions,
		FocusCount:      focus,
		FillerCount:     filler,
		WordCount:       wordCount,
		FocusRatio:      math.Round(focusRatio*100) / 100,
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
		var aggDetail RelevanceDetail
		labs := map[string]bool{}

		for _, e := range entries {
			labs[e.Lab] = true
			score, detail := AnalyzeText(e.Summary)
			totalScore += score
			aggDetail.OnTopicCount += detail.OnTopicCount
			aggDetail.OffTopicCount += detail.OffTopicCount
			aggDetail.DigressionCount += detail.DigressionCount
			aggDetail.FocusCount += detail.FocusCount
			aggDetail.FillerCount += detail.FillerCount
			aggDetail.WordCount += detail.WordCount
			aggDetail.FocusRatio += detail.FocusRatio
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		aggDetail.FocusRatio = math.Round(aggDetail.FocusRatio/n*100) / 100

		models = append(models, ModelRelevance{
			Model:       mn,
			Score:       math.Round(avgScore*10) / 10,
			Grade:       scoreToGrade(avgScore),
			Detail:      aggDetail,
			SampleCount: len(entries),
			LabCount:    len(labs),
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].Score > models[j].Score
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.Score
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

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s\n", m.Model)
		fmt.Fprintf(&b, "    Score: %.1f  Grade: %s\n", m.Score, m.Grade)
		fmt.Fprintf(&b, "    On-topic: %d  Off-topic: %d  Digressions: %d\n",
			m.Detail.OnTopicCount, m.Detail.OffTopicCount, m.Detail.DigressionCount)
		fmt.Fprintf(&b, "    Focus: %d  Filler: %d  Ratio: %.2f\n",
			m.Detail.FocusCount, m.Detail.FillerCount, m.Detail.FocusRatio)
		fmt.Fprintf(&b, "    Samples: %d  Labs: %d\n\n",
			m.SampleCount, m.LabCount)
	}

	fmt.Fprintf(&b, "  Best: %s  Worst: %s  Mean: %.1f\n",
		r.BestModel, r.WorstModel, r.MeanScore)

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data)
}
