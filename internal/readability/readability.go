package readability

import (
	"encoding/json"
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
	LongSentences  int     `json:"long_sentences"`
	ShortSentences int     `json:"short_sentences"`
	JargonCount    int     `json:"jargon_count"`
	PassiveCount   int     `json:"passive_count"`
	TransitionCues int     `json:"transition_cues"`
	StructureCues  int     `json:"structure_cues"`
}

type ModelReadability struct {
	Model            string            `json:"model"`
	ReadabilityScore float64           `json:"readability_score"`
	Grade            string            `json:"grade"`
	Detail           ReadabilityDetail `json:"detail"`
	SampleCount      int               `json:"sample_count"`
	LabCount         int               `json:"lab_count"`
	PoorReadability  int               `json:"poor_readability"`
}

type Report struct {
	Models       []ModelReadability `json:"models"`
	MostReadable string             `json:"most_readable"`
	LeastReadable string            `json:"least_readable"`
	MeanScore    float64            `json:"mean_score"`
}

var jargonPhrases = []string{
	"synergy", "leverage", "paradigm", "utilize",
	"operationalize", "ideation", "holistic",
	"best-of-breed", "bleeding edge", "pivot",
	"disruptive", "scalable", "actionable",
	"deep dive", "thought leader", "circle back",
	"move the needle", "low-hanging fruit",
}

var passivePhrases = []string{
	"was done", "was made", "was created",
	"was performed", "was completed", "was found",
	"was given", "was taken", "was shown",
	"were done", "were made", "were found",
	"is considered", "is believed", "is assumed",
	"has been", "have been", "had been",
}

var transitionPhrases = []string{
	"however", "therefore", "moreover", "furthermore",
	"in addition", "consequently", "nevertheless",
	"on the other hand", "in contrast", "similarly",
	"for example", "in particular", "specifically",
	"as a result", "in conclusion", "to summarize",
}

var structurePhrases = []string{
	"first", "second", "third", "finally",
	"step 1", "step 2", "step 3",
	"in summary", "to begin", "next",
	"the following", "as follows", "below",
	"above", "previously mentioned",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func splitSentences(text string) []string {
	var sentences []string
	current := strings.Builder{}
	for _, r := range text {
		current.WriteRune(r)
		if r == '.' || r == '!' || r == '?' {
			s := strings.TrimSpace(current.String())
			if len(s) > 2 {
				sentences = append(sentences, s)
			}
			current.Reset()
		}
	}
	if s := strings.TrimSpace(current.String()); len(s) > 2 {
		sentences = append(sentences, s)
	}
	return sentences
}

func avgWordLength(text string) float64 {
	words := strings.Fields(text)
	if len(words) == 0 {
		return 0
	}
	total := 0
	for _, w := range words {
		for _, r := range w {
			if unicode.IsLetter(r) {
				total++
			}
		}
	}
	return float64(total) / float64(len(words))
}

func scoreResponse(text string) (float64, ReadabilityDetail) {
	sentences := splitSentences(text)
	words := strings.Fields(text)
	jargon := countPhrases(text, jargonPhrases)
	passive := countPhrases(text, passivePhrases)
	transitions := countPhrases(text, transitionPhrases)
	structure := countPhrases(text, structurePhrases)

	var avgSentLen float64
	longSent := 0
	shortSent := 0
	if len(sentences) > 0 {
		totalWords := 0
		for _, s := range sentences {
			wc := len(strings.Fields(s))
			totalWords += wc
			if wc > 30 {
				longSent++
			}
			if wc < 5 {
				shortSent++
			}
		}
		avgSentLen = float64(totalWords) / float64(len(sentences))
	}

	detail := ReadabilityDetail{
		AvgSentenceLen: math.Round(avgSentLen*10) / 10,
		AvgWordLen:     math.Round(avgWordLength(text)*10) / 10,
		LongSentences:  longSent,
		ShortSentences: shortSent,
		JargonCount:    jargon,
		PassiveCount:   passive,
		TransitionCues: transitions,
		StructureCues:  structure,
	}

	if len(words) == 0 {
		return 100, detail
	}

	score := 80.0

	if avgSentLen > 25 {
		score -= (avgSentLen - 25) * 0.5
	}
	if avgSentLen > 0 && avgSentLen < 8 {
		score -= (8 - avgSentLen) * 0.3
	}

	jargonPenalty := float64(jargon) * 2.0
	passivePenalty := float64(passive) * 1.5
	score -= jargonPenalty + passivePenalty

	transitionBonus := float64(transitions) * 2.0
	structureBonus := float64(structure) * 1.5
	score += transitionBonus + structureBonus

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

	var models []ModelReadability
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail ReadabilityDetail
		poorReadability := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			score, detail := scoreResponse(e.Summary)
			totalScore += score
			totalDetail.AvgSentenceLen += detail.AvgSentenceLen
			totalDetail.AvgWordLen += detail.AvgWordLen
			totalDetail.LongSentences += detail.LongSentences
			totalDetail.ShortSentences += detail.ShortSentences
			totalDetail.JargonCount += detail.JargonCount
			totalDetail.PassiveCount += detail.PassiveCount
			totalDetail.TransitionCues += detail.TransitionCues
			totalDetail.StructureCues += detail.StructureCues

			if score < 50 {
				poorReadability++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.AvgSentenceLen = math.Round(totalDetail.AvgSentenceLen/n*10) / 10
		totalDetail.AvgWordLen = math.Round(totalDetail.AvgWordLen/n*10) / 10

		models = append(models, ModelReadability{
			Model:            mn,
			ReadabilityScore: math.Round(avgScore*10) / 10,
			Grade:            scoreToGrade(avgScore),
			Detail:           totalDetail,
			SampleCount:      len(entries),
			LabCount:         len(labs),
			PoorReadability:  poorReadability,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].ReadabilityScore > models[j].ReadabilityScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.ReadabilityScore
	}

	r := Report{Models: models}
	if len(models) > 0 {
		r.MostReadable = models[0].Model
		r.LeastReadable = models[len(models)-1].Model
		r.MeanScore = math.Round(sumScore/float64(len(models))*10) / 10
	}
	return r
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Readability Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No readability data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Most readable:        %s\n", r.MostReadable)
	fmt.Fprintf(&b, "  Least readable:       %s\n\n", r.LeastReadable)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.ReadabilityScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "avg_sentence_len", m.Detail.AvgSentenceLen)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "avg_word_len", m.Detail.AvgWordLen)
		fmt.Fprintf(&b, "    %-28s %d\n", "long_sentences", m.Detail.LongSentences)
		fmt.Fprintf(&b, "    %-28s %d\n", "jargon_count", m.Detail.JargonCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "passive_count", m.Detail.PassiveCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "transition_cues", m.Detail.TransitionCues)
		fmt.Fprintf(&b, "    %-28s %d\n", "structure_cues", m.Detail.StructureCues)
		fmt.Fprintf(&b, "    Poor readability: %d\n", m.PoorReadability)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
