package verbosity

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type VerbosityDetail struct {
	WordCount          int     `json:"word_count"`
	SentenceCount      int     `json:"sentence_count"`
	FillerPhrases      int     `json:"filler_phrases"`
	RedundantPhrases   int     `json:"redundant_phrases"`
	OverQualifications int     `json:"over_qualifications"`
	InfoDensity        float64 `json:"info_density"`
}

type ModelVerbosity struct {
	Model          string          `json:"model"`
	VerbosityScore float64         `json:"verbosity_score"`
	Grade          string          `json:"grade"`
	Detail         VerbosityDetail `json:"detail"`
	SampleCount    int             `json:"sample_count"`
	LabCount       int             `json:"lab_count"`
	TooVerbose     int             `json:"too_verbose"`
}

type Report struct {
	Models       []ModelVerbosity `json:"models"`
	MostConcise  string           `json:"most_concise"`
	MostVerbose  string           `json:"most_verbose"`
	MeanScore    float64          `json:"mean_score"`
}

var fillerPhrases = []string{
	"it is important to note that",
	"it should be noted that",
	"it is worth mentioning that",
	"as a matter of fact",
	"in order to",
	"for the purpose of",
	"due to the fact that",
	"in the event that",
	"at this point in time",
	"as previously mentioned",
	"it goes without saying",
	"needless to say",
	"in terms of",
	"with regard to",
	"with respect to",
	"on the other hand",
	"having said that",
	"that being said",
	"it is also worth noting",
	"basically",
	"essentially",
	"actually",
}

var redundantPhrases = []string{
	"past history",
	"future plans",
	"end result",
	"free gift",
	"added bonus",
	"close proximity",
	"each and every",
	"first and foremost",
	"various different",
	"completely eliminate",
	"absolutely essential",
	"true fact",
	"basic fundamentals",
	"final outcome",
	"general consensus",
	"new innovation",
	"completely finished",
	"repeat again",
}

var overQualPhrases = []string{
	"i think that perhaps",
	"it might be possible that",
	"it could potentially be",
	"there is a possibility that",
	"it is somewhat likely",
	"it may or may not be",
	"to some extent",
	"in some cases",
	"more or less",
	"sort of",
	"kind of",
	"arguably",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func splitSentences(text string) int {
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

func infoDensity(text string) float64 {
	words := strings.Fields(text)
	if len(words) == 0 {
		return 1.0
	}
	unique := map[string]bool{}
	for _, w := range words {
		unique[strings.ToLower(w)] = true
	}
	return float64(len(unique)) / float64(len(words))
}

func scoreResponse(text string) (float64, VerbosityDetail) {
	words := strings.Fields(text)
	wordCount := len(words)
	sentCount := splitSentences(text)
	fillers := countPhrases(text, fillerPhrases)
	redundant := countPhrases(text, redundantPhrases)
	overQual := countPhrases(text, overQualPhrases)
	density := infoDensity(text)

	detail := VerbosityDetail{
		WordCount:          wordCount,
		SentenceCount:      sentCount,
		FillerPhrases:      fillers,
		RedundantPhrases:   redundant,
		OverQualifications: overQual,
		InfoDensity:        math.Round(density*100) / 100,
	}

	if wordCount == 0 {
		return 100, detail
	}

	score := 85.0

	fillerPenalty := float64(fillers) * 3.0
	redundantPenalty := float64(redundant) * 4.0
	overQualPenalty := float64(overQual) * 2.5
	score -= fillerPenalty + redundantPenalty + overQualPenalty

	if density < 0.3 {
		score -= (0.3 - density) * 40
	}

	if wordCount > 500 {
		score -= float64(wordCount-500) * 0.02
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

	var models []ModelVerbosity
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail VerbosityDetail
		tooVerbose := 0
		labs := map[string]bool{}

		for _, e := range entries {
			labs[e.Lab] = true
			score, detail := scoreResponse(e.Summary)
			totalScore += score
			totalDetail.WordCount += detail.WordCount
			totalDetail.SentenceCount += detail.SentenceCount
			totalDetail.FillerPhrases += detail.FillerPhrases
			totalDetail.RedundantPhrases += detail.RedundantPhrases
			totalDetail.OverQualifications += detail.OverQualifications
			totalDetail.InfoDensity += detail.InfoDensity
			if score < 50 {
				tooVerbose++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.InfoDensity = math.Round(totalDetail.InfoDensity/n*100) / 100

		models = append(models, ModelVerbosity{
			Model:          mn,
			VerbosityScore: math.Round(avgScore*10) / 10,
			Grade:          scoreToGrade(avgScore),
			Detail:         totalDetail,
			SampleCount:    len(entries),
			LabCount:       len(labs),
			TooVerbose:     tooVerbose,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].VerbosityScore > models[j].VerbosityScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.VerbosityScore
	}

	r := Report{Models: models}
	if len(models) > 0 {
		r.MostConcise = models[0].Model
		r.MostVerbose = models[len(models)-1].Model
		r.MeanScore = math.Round(sumScore/float64(len(models))*10) / 10
	}
	return r
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Verbosity Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No verbosity data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Most concise:         %s\n", r.MostConcise)
	fmt.Fprintf(&b, "  Most verbose:         %s\n\n", r.MostVerbose)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.VerbosityScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "word_count", m.Detail.WordCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "sentence_count", m.Detail.SentenceCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "filler_phrases", m.Detail.FillerPhrases)
		fmt.Fprintf(&b, "    %-28s %d\n", "redundant_phrases", m.Detail.RedundantPhrases)
		fmt.Fprintf(&b, "    %-28s %d\n", "over_qualifications", m.Detail.OverQualifications)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "info_density", m.Detail.InfoDensity)
		fmt.Fprintf(&b, "    Too verbose: %d\n", m.TooVerbose)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
