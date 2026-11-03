package brevity

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type BrevityDetail struct {
	FillerRatio      float64 `json:"filler_ratio"`
	RedundancyRatio  float64 `json:"redundancy_ratio"`
	PreambleCount    int     `json:"preamble_count"`
	QualifierCount   int     `json:"qualifier_count"`
	RepetitionScore  float64 `json:"repetition_score"`
	WordEfficiency   float64 `json:"word_efficiency"`
	SubstanceWords   int     `json:"substance_words"`
	TotalWords       int     `json:"total_words"`
}

type ModelBrevity struct {
	Model        string        `json:"model"`
	BrevityScore float64       `json:"brevity_score"`
	Grade        string        `json:"grade"`
	Detail       BrevityDetail `json:"detail"`
	SampleCount  int           `json:"sample_count"`
	LabCount     int           `json:"lab_count"`
	Concerns     int           `json:"concerns"`
}

type Report struct {
	Models     []ModelBrevity `json:"models"`
	BestModel  string         `json:"best_model"`
	WorstModel string         `json:"worst_model"`
	MeanScore  float64        `json:"mean_score"`
}

var fillerPhrases = []string{
	"it is important to note that",
	"it should be noted that",
	"as a matter of fact",
	"in order to",
	"for the purpose of",
	"due to the fact that",
	"in light of the fact that",
	"it is worth mentioning that",
	"at the end of the day",
	"needless to say",
	"as previously mentioned",
	"it goes without saying",
	"when all is said and done",
	"in the event that",
	"with respect to",
	"in regard to",
	"in terms of",
	"on the other hand",
	"having said that",
	"be that as it may",
}

var redundantQualifiers = []string{
	"very unique",
	"absolutely essential",
	"completely unanimous",
	"totally complete",
	"entirely whole",
	"extremely vital",
	"really important",
	"quite significant",
	"very critical",
	"particularly notable",
	"especially important",
	"highly recommend",
	"strongly suggest",
	"definitely should",
	"absolutely must",
}

var preamblePhrases = []string{
	"i'd be happy to help",
	"great question",
	"that's an interesting point",
	"that's a great question",
	"thank you for asking",
	"i appreciate the question",
	"let me explain",
	"let me break this down",
	"i'll do my best to",
	"sure, i can help with that",
	"of course",
	"certainly",
	"absolutely",
	"here's what you need to know",
	"to answer your question",
}

var repetitivePhrases = []string{
	"as i mentioned",
	"as stated above",
	"as noted earlier",
	"to reiterate",
	"once again",
	"as i said",
	"to repeat",
	"in summary",
	"to summarize",
	"in conclusion",
	"to conclude",
	"all in all",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, BrevityDetail) {
	combined := response + " " + evidence
	words := strings.Fields(combined)
	totalWords := len(words)
	if totalWords == 0 {
		return 85.0, BrevityDetail{WordEfficiency: 1.0}
	}

	fillers := countPhrases(combined, fillerPhrases)
	qualifiers := countPhrases(combined, redundantQualifiers)
	preambles := countPhrases(combined, preamblePhrases)
	repetitions := countPhrases(combined, repetitivePhrases)

	fillerWordCount := fillers * 5
	qualifierWordCount := qualifiers * 2
	preambleWordCount := preambles * 4
	repetitionWordCount := repetitions * 3

	wasteWords := fillerWordCount + qualifierWordCount + preambleWordCount + repetitionWordCount
	substanceWords := totalWords - wasteWords
	if substanceWords < 0 {
		substanceWords = 0
	}

	fillerRatio := float64(fillerWordCount) / float64(totalWords)
	redundancyRatio := float64(qualifierWordCount+repetitionWordCount) / float64(totalWords)
	wordEfficiency := float64(substanceWords) / float64(totalWords)

	uniqueWords := map[string]bool{}
	for _, w := range words {
		uniqueWords[strings.ToLower(w)] = true
	}
	repetitionScore := 1.0
	if totalWords > 10 {
		repetitionScore = float64(len(uniqueWords)) / float64(totalWords)
	}

	detail := BrevityDetail{
		FillerRatio:     math.Round(fillerRatio*1000) / 1000,
		RedundancyRatio: math.Round(redundancyRatio*1000) / 1000,
		PreambleCount:   preambles,
		QualifierCount:  qualifiers,
		RepetitionScore: math.Round(repetitionScore*100) / 100,
		WordEfficiency:  math.Round(wordEfficiency*100) / 100,
		SubstanceWords:  substanceWords,
		TotalWords:      totalWords,
	}

	score := 50.0
	score += wordEfficiency * 20.0
	score += repetitionScore * 15.0

	score -= fillerRatio * 80.0
	score -= redundancyRatio * 60.0
	score -= float64(preambles) * 5.0

	if fillers == 0 && qualifiers == 0 && preambles == 0 {
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

	var models []ModelBrevity
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail BrevityDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.FillerRatio += detail.FillerRatio
			totalDetail.RedundancyRatio += detail.RedundancyRatio
			totalDetail.PreambleCount += detail.PreambleCount
			totalDetail.QualifierCount += detail.QualifierCount
			totalDetail.RepetitionScore += detail.RepetitionScore
			totalDetail.WordEfficiency += detail.WordEfficiency
			totalDetail.SubstanceWords += detail.SubstanceWords
			totalDetail.TotalWords += detail.TotalWords

			if detail.FillerRatio > 0.1 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.FillerRatio = math.Round(totalDetail.FillerRatio/n*1000) / 1000
		totalDetail.RedundancyRatio = math.Round(totalDetail.RedundancyRatio/n*1000) / 1000
		totalDetail.RepetitionScore = math.Round(totalDetail.RepetitionScore/n*100) / 100
		totalDetail.WordEfficiency = math.Round(totalDetail.WordEfficiency/n*100) / 100

		models = append(models, ModelBrevity{
			Model:        mn,
			BrevityScore: math.Round(avgScore*10) / 10,
			Grade:        scoreToGrade(avgScore),
			Detail:       totalDetail,
			SampleCount:  len(entries),
			LabCount:     len(labs),
			Concerns:     totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].BrevityScore > models[j].BrevityScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.BrevityScore
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
	b.WriteString("Response Brevity Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No brevity data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.BrevityScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.3f\n", "filler_ratio", m.Detail.FillerRatio)
		fmt.Fprintf(&b, "    %-28s %.3f\n", "redundancy_ratio", m.Detail.RedundancyRatio)
		fmt.Fprintf(&b, "    %-28s %d\n", "preamble_count", m.Detail.PreambleCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "qualifier_count", m.Detail.QualifierCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "repetition_score", m.Detail.RepetitionScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "word_efficiency", m.Detail.WordEfficiency)
		fmt.Fprintf(&b, "    %-28s %d\n", "substance_words", m.Detail.SubstanceWords)
		fmt.Fprintf(&b, "    %-28s %d\n", "total_words", m.Detail.TotalWords)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
