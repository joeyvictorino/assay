package clarity

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type ClarityDetail struct {
	AvgSentenceLen float64 `json:"avg_sentence_len"`
	LongSentences  int     `json:"long_sentences"`
	JargonCount    int     `json:"jargon_count"`
	AmbiguityCount int     `json:"ambiguity_count"`
	ReadingEase    float64 `json:"reading_ease"`
	StructureScore float64 `json:"structure_score"`
	TotalWords     int     `json:"total_words"`
	TotalSentences int     `json:"total_sentences"`
}

type ModelClarity struct {
	Model        string       `json:"model"`
	ClarityScore float64      `json:"clarity_score"`
	Grade        string       `json:"grade"`
	Detail       ClarityDetail `json:"detail"`
	SampleCount  int          `json:"sample_count"`
	LabCount     int          `json:"lab_count"`
	Concerns     int          `json:"concerns"`
}

type Report struct {
	Models     []ModelClarity `json:"models"`
	BestModel  string         `json:"best_model"`
	WorstModel string         `json:"worst_model"`
	MeanScore  float64        `json:"mean_score"`
}

var jargonPhrases = []string{
	"paradigm shift", "synergy", "leverage", "utilize", "operationalize",
	"methodological framework", "holistic approach", "best-in-class",
	"value proposition", "core competency", "thought leadership",
	"bandwidth", "scalable solution", "ecosystem", "disruptive innovation",
}

var ambiguityPhrases = []string{
	"it depends", "in some cases", "under certain circumstances",
	"there are various", "it can vary", "sometimes", "occasionally",
	"in general", "more or less", "to some extent",
	"roughly speaking", "approximately", "arguably",
}

func splitSentences(text string) []string {
	var sentences []string
	current := strings.Builder{}
	for _, ch := range text {
		current.WriteRune(ch)
		if ch == '.' || ch == '!' || ch == '?' {
			s := strings.TrimSpace(current.String())
			if len(s) > 0 {
				sentences = append(sentences, s)
			}
			current.Reset()
		}
	}
	s := strings.TrimSpace(current.String())
	if len(s) > 3 {
		sentences = append(sentences, s)
	}
	return sentences
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, ClarityDetail) {
	combined := response + " " + evidence
	words := strings.Fields(combined)
	wordCount := len(words)

	if wordCount == 0 {
		return 85.0, ClarityDetail{ReadingEase: 1.0, StructureScore: 1.0}
	}

	sentences := splitSentences(combined)
	sentCount := len(sentences)
	if sentCount == 0 {
		sentCount = 1
	}

	avgSentLen := float64(wordCount) / float64(sentCount)

	longSentences := 0
	for _, s := range sentences {
		if len(strings.Fields(s)) > 30 {
			longSentences++
		}
	}

	jargon := countPhrases(combined, jargonPhrases)
	ambiguity := countPhrases(combined, ambiguityPhrases)

	readingEase := 1.0
	if avgSentLen > 20 {
		readingEase -= (avgSentLen - 20) * 0.03
	}
	if readingEase < 0 {
		readingEase = 0
	}

	structureScore := 1.0
	if sentCount > 1 {
		structureScore = math.Min(1.0, float64(sentCount)*0.1+0.3)
	}
	if longSentences > 0 {
		structureScore -= float64(longSentences) * 0.1
	}
	if structureScore < 0 {
		structureScore = 0
	}

	detail := ClarityDetail{
		AvgSentenceLen: math.Round(avgSentLen*10) / 10,
		LongSentences:  longSentences,
		JargonCount:    jargon,
		AmbiguityCount: ambiguity,
		ReadingEase:    math.Round(readingEase*100) / 100,
		StructureScore: math.Round(structureScore*100) / 100,
		TotalWords:     wordCount,
		TotalSentences: sentCount,
	}

	score := 40.0
	score += readingEase * 20.0
	score += structureScore * 15.0
	score -= float64(jargon) * 4.0
	score -= float64(ambiguity) * 3.0
	score -= float64(longSentences) * 3.0

	if jargon == 0 && ambiguity == 0 && longSentences == 0 {
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

	var models []ModelClarity
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail ClarityDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.AvgSentenceLen += detail.AvgSentenceLen
			totalDetail.LongSentences += detail.LongSentences
			totalDetail.JargonCount += detail.JargonCount
			totalDetail.AmbiguityCount += detail.AmbiguityCount
			totalDetail.ReadingEase += detail.ReadingEase
			totalDetail.StructureScore += detail.StructureScore
			totalDetail.TotalWords += detail.TotalWords
			totalDetail.TotalSentences += detail.TotalSentences

			if detail.ReadingEase < 0.5 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.AvgSentenceLen = math.Round(totalDetail.AvgSentenceLen/n*10) / 10
		totalDetail.ReadingEase = math.Round(totalDetail.ReadingEase/n*100) / 100
		totalDetail.StructureScore = math.Round(totalDetail.StructureScore/n*100) / 100

		models = append(models, ModelClarity{
			Model:        mn,
			ClarityScore: math.Round(avgScore*10) / 10,
			Grade:        scoreToGrade(avgScore),
			Detail:       totalDetail,
			SampleCount:  len(entries),
			LabCount:     len(labs),
			Concerns:     totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].ClarityScore > models[j].ClarityScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.ClarityScore
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
	b.WriteString("Response Clarity Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No clarity data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.ClarityScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "avg_sentence_len", m.Detail.AvgSentenceLen)
		fmt.Fprintf(&b, "    %-28s %d\n", "long_sentences", m.Detail.LongSentences)
		fmt.Fprintf(&b, "    %-28s %d\n", "jargon_count", m.Detail.JargonCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "ambiguity_count", m.Detail.AmbiguityCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "reading_ease", m.Detail.ReadingEase)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "structure_score", m.Detail.StructureScore)
		fmt.Fprintf(&b, "    %-28s %d\n", "total_words", m.Detail.TotalWords)
		fmt.Fprintf(&b, "    %-28s %d\n", "total_sentences", m.Detail.TotalSentences)
		fmt.Fprintf(&b, "    Concerns: %d\n\n", m.Concerns)
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
