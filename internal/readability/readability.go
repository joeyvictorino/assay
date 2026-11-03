package readability

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/joeyvictorino/assay/internal/model"
)

type ReadabilityDetail struct {
	AvgSentenceLen float64 `json:"avg_sentence_len"`
	AvgWordLen     float64 `json:"avg_word_len"`
	SyllableCount  int     `json:"syllable_count"`
	WordCount      int     `json:"word_count"`
	SentenceCount  int     `json:"sentence_count"`
	ComplexWords   int     `json:"complex_words"`
	FleschKincaid  float64 `json:"flesch_kincaid"`
	GunningFog     float64 `json:"gunning_fog"`
}

type ModelReadability struct {
	Model       string            `json:"model"`
	Score       float64           `json:"score"`
	Grade       string            `json:"grade"`
	Detail      ReadabilityDetail `json:"detail"`
	SampleCount int               `json:"sample_count"`
	LabCount    int               `json:"lab_count"`
}

type Report struct {
	Models     []ModelReadability `json:"models"`
	BestModel  string             `json:"best_model"`
	WorstModel string             `json:"worst_model"`
	MeanScore  float64            `json:"mean_score"`
}

func CountSyllables(word string) int {
	word = strings.ToLower(word)
	if len(word) <= 2 {
		return 1
	}

	vowels := "aeiouy"
	count := 0
	prevVowel := false

	for _, ch := range word {
		isVowel := strings.ContainsRune(vowels, ch)
		if isVowel && !prevVowel {
			count++
		}
		prevVowel = isVowel
	}

	if strings.HasSuffix(word, "e") && count > 1 {
		count--
	}
	if strings.HasSuffix(word, "le") && len(word) > 3 && !strings.ContainsRune(vowels, rune(word[len(word)-3])) {
		count++
	}

	if count == 0 {
		count = 1
	}
	return count
}

func ExtractWords(text string) []string {
	var words []string
	for _, w := range strings.Fields(text) {
		cleaned := strings.TrimFunc(w, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		})
		if cleaned != "" {
			words = append(words, cleaned)
		}
	}
	return words
}

func CountSentences(text string) int {
	count := 0
	for _, r := range text {
		if r == '.' || r == '!' || r == '?' {
			count++
		}
	}
	if count == 0 && len(strings.TrimSpace(text)) > 0 {
		count = 1
	}
	return count
}

func AnalyzeText(text string) (float64, ReadabilityDetail) {
	words := ExtractWords(text)
	wordCount := len(words)
	sentenceCount := CountSentences(text)

	if wordCount == 0 {
		return 50.0, ReadabilityDetail{}
	}
	if sentenceCount == 0 {
		sentenceCount = 1
	}

	totalSyllables := 0
	totalWordLen := 0
	complexWords := 0

	for _, w := range words {
		syl := CountSyllables(w)
		totalSyllables += syl
		totalWordLen += len(w)
		if syl >= 3 {
			complexWords++
		}
	}

	avgSentLen := float64(wordCount) / float64(sentenceCount)
	avgWordLen := float64(totalWordLen) / float64(wordCount)
	avgSyllPerWord := float64(totalSyllables) / float64(wordCount)

	fleschKincaid := 0.39*avgSentLen + 11.8*avgSyllPerWord - 15.59
	if fleschKincaid < 0 {
		fleschKincaid = 0
	}

	complexRatio := float64(complexWords) / float64(wordCount)
	gunningFog := 0.4 * (avgSentLen + 100*complexRatio)
	if gunningFog < 0 {
		gunningFog = 0
	}

	score := 100.0 - (fleschKincaid-4.0)*6.0
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	detail := ReadabilityDetail{
		AvgSentenceLen: math.Round(avgSentLen*10) / 10,
		AvgWordLen:     math.Round(avgWordLen*10) / 10,
		SyllableCount:  totalSyllables,
		WordCount:      wordCount,
		SentenceCount:  sentenceCount,
		ComplexWords:   complexWords,
		FleschKincaid:  math.Round(fleschKincaid*10) / 10,
		GunningFog:     math.Round(gunningFog*10) / 10,
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

	var models []ModelReadability
	for mn, entries := range modelEntries {
		var totalScore float64
		var aggDetail ReadabilityDetail
		labs := map[string]bool{}

		for _, e := range entries {
			labs[e.Lab] = true
			score, detail := AnalyzeText(e.Summary)
			totalScore += score
			aggDetail.SyllableCount += detail.SyllableCount
			aggDetail.WordCount += detail.WordCount
			aggDetail.SentenceCount += detail.SentenceCount
			aggDetail.ComplexWords += detail.ComplexWords
			aggDetail.FleschKincaid += detail.FleschKincaid
			aggDetail.GunningFog += detail.GunningFog
			aggDetail.AvgSentenceLen += detail.AvgSentenceLen
			aggDetail.AvgWordLen += detail.AvgWordLen
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		aggDetail.FleschKincaid = math.Round(aggDetail.FleschKincaid/n*10) / 10
		aggDetail.GunningFog = math.Round(aggDetail.GunningFog/n*10) / 10
		aggDetail.AvgSentenceLen = math.Round(aggDetail.AvgSentenceLen/n*10) / 10
		aggDetail.AvgWordLen = math.Round(aggDetail.AvgWordLen/n*10) / 10

		models = append(models, ModelReadability{
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
	b.WriteString("Response Readability Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No readability data available.\n")
		return b.String()
	}

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s\n", m.Model)
		fmt.Fprintf(&b, "    Score: %.1f  Grade: %s\n", m.Score, m.Grade)
		fmt.Fprintf(&b, "    Flesch-Kincaid: %.1f  Gunning Fog: %.1f\n",
			m.Detail.FleschKincaid, m.Detail.GunningFog)
		fmt.Fprintf(&b, "    Avg Sentence: %.1f words  Avg Word: %.1f chars\n",
			m.Detail.AvgSentenceLen, m.Detail.AvgWordLen)
		fmt.Fprintf(&b, "    Complex Words: %d  Samples: %d  Labs: %d\n\n",
			m.Detail.ComplexWords, m.SampleCount, m.LabCount)
	}

	fmt.Fprintf(&b, "  Best: %s  Worst: %s  Mean: %.1f\n",
		r.BestModel, r.WorstModel, r.MeanScore)

	return b.String()
}
