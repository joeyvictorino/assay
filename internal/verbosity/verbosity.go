package verbosity

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type VerbosityDetail struct {
	WordCount       int     `json:"word_count"`
	SentenceCount   int     `json:"sentence_count"`
	AvgSentenceLen  float64 `json:"avg_sentence_len"`
	RepetitionScore float64 `json:"repetition_score"`
	FillerCount     int     `json:"filler_count"`
}

type ModelVerbosity struct {
	Model       string          `json:"model"`
	Score       float64         `json:"score"`
	Grade       string          `json:"grade"`
	Detail      VerbosityDetail `json:"detail"`
	SampleCount int             `json:"sample_count"`
	LabCount    int             `json:"lab_count"`
}

type Report struct {
	Models     []ModelVerbosity `json:"models"`
	BestModel  string           `json:"best_model"`
	WorstModel string           `json:"worst_model"`
	MeanScore  float64          `json:"mean_score"`
}

var fillerPhrases = []string{
	"as i mentioned", "it's important to note", "as you can see",
	"in other words", "to be more specific", "let me explain",
	"basically", "essentially",
}

func countSentences(text string) int {
	count := 0
	for _, r := range text {
		if r == '.' || r == '!' || r == '?' {
			count++
		}
	}
	if count == 0 && len(strings.TrimSpace(text)) > 0 {
		return 1
	}
	return count
}

func countFillers(text string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range fillerPhrases {
		count += strings.Count(lower, p)
	}
	return count
}

func countRepetition(text string) float64 {
	words := strings.Fields(strings.ToLower(text))
	if len(words) < 3 {
		return 0
	}
	trigrams := map[string]int{}
	for i := 0; i <= len(words)-3; i++ {
		tri := words[i] + " " + words[i+1] + " " + words[i+2]
		trigrams[tri]++
	}
	repeated := 0
	for _, c := range trigrams {
		if c > 1 {
			repeated += c - 1
		}
	}
	total := len(words) - 2
	if total == 0 {
		return 0
	}
	return float64(repeated) / float64(total) * 100
}

func AnalyzeText(prompt, response string) (float64, VerbosityDetail) {
	respWords := strings.Fields(response)
	promptWords := strings.Fields(prompt)
	wordCount := len(respWords)
	promptLen := len(promptWords)

	if wordCount == 0 {
		return 50.0, VerbosityDetail{}
	}

	sentenceCount := countSentences(response)
	avgSentenceLen := float64(wordCount) / float64(sentenceCount)
	fillerCount := countFillers(response)
	repetitionScore := countRepetition(response)

	ratio := 1.0
	if promptLen > 0 {
		ratio = float64(wordCount) / float64(promptLen)
	}

	score := 50.0
	if ratio > 5 {
		score += (ratio - 5) * 3.0
	} else if ratio < 1 {
		score -= (1 - ratio) * 10.0
	}

	score += float64(fillerCount) * 5.0
	score += repetitionScore * 2.0

	if avgSentenceLen > 30 {
		score += (avgSentenceLen - 30) * 1.0
	}

	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	detail := VerbosityDetail{
		WordCount:       wordCount,
		SentenceCount:   sentenceCount,
		AvgSentenceLen:  math.Round(avgSentenceLen*10) / 10,
		RepetitionScore: math.Round(repetitionScore*10) / 10,
		FillerCount:     fillerCount,
	}

	return math.Round(score*10) / 10, detail
}

func scoreToGrade(score float64) string {
	diff := math.Abs(score - 50)
	switch {
	case diff <= 5:
		return "A"
	case diff <= 10:
		return "B"
	case diff <= 20:
		return "C"
	case diff <= 30:
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

	var models []ModelVerbosity
	for mn, entries := range modelEntries {
		var totalScore float64
		var aggDetail VerbosityDetail
		labs := map[string]bool{}

		for _, e := range entries {
			labs[e.Lab] = true
			score, detail := AnalyzeText(e.Lab, e.Summary)
			totalScore += score
			aggDetail.WordCount += detail.WordCount
			aggDetail.SentenceCount += detail.SentenceCount
			aggDetail.FillerCount += detail.FillerCount
			aggDetail.RepetitionScore += detail.RepetitionScore
			aggDetail.AvgSentenceLen += detail.AvgSentenceLen
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		aggDetail.AvgSentenceLen = math.Round(aggDetail.AvgSentenceLen/n*10) / 10
		aggDetail.RepetitionScore = math.Round(aggDetail.RepetitionScore/n*10) / 10

		models = append(models, ModelVerbosity{
			Model:       mn,
			Score:       math.Round(avgScore*10) / 10,
			Grade:       scoreToGrade(avgScore),
			Detail:      aggDetail,
			SampleCount: len(entries),
			LabCount:    len(labs),
		})
	}

	var bestScore float64 = 100
	var worstScore float64 = 0
	var bestModel, worstModel string
	var sumScore float64

	for _, m := range models {
		sumScore += m.Score
		diff := math.Abs(m.Score - 50)
		if diff < math.Abs(bestScore-50) {
			bestScore = m.Score
			bestModel = m.Model
		}
		if diff > math.Abs(worstScore-50) {
			worstScore = m.Score
			worstModel = m.Model
		}
	}

	return Report{
		Models:     models,
		BestModel:  bestModel,
		WorstModel: worstModel,
		MeanScore:  math.Round(sumScore/float64(len(models))*10) / 10,
	}
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Response Verbosity Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No verbosity data available.\n")
		return b.String()
	}

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s\n", m.Model)
		fmt.Fprintf(&b, "    Score: %.1f  Grade: %s\n", m.Score, m.Grade)
		fmt.Fprintf(&b, "    Words: %d  Sentences: %d  Avg Len: %.1f\n",
			m.Detail.WordCount, m.Detail.SentenceCount, m.Detail.AvgSentenceLen)
		fmt.Fprintf(&b, "    Fillers: %d  Repetition: %.1f%%\n",
			m.Detail.FillerCount, m.Detail.RepetitionScore)
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
