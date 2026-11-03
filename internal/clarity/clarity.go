package clarity

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type ClarityDetail struct {
	TransitionCount int     `json:"transition_count"`
	StructureCount  int     `json:"structure_count"`
	AmbiguityCount  int     `json:"ambiguity_count"`
	RepetitionCount int     `json:"repetition_count"`
	AvgSentenceLen  float64 `json:"avg_sentence_len"`
	WordCount       int     `json:"word_count"`
}

type ModelClarity struct {
	Model       string        `json:"model"`
	Score       float64       `json:"score"`
	Grade       string        `json:"grade"`
	Detail      ClarityDetail `json:"detail"`
	SampleCount int           `json:"sample_count"`
	LabCount    int           `json:"lab_count"`
}

type Report struct {
	Models     []ModelClarity `json:"models"`
	BestModel  string         `json:"best_model"`
	WorstModel string         `json:"worst_model"`
	MeanScore  float64        `json:"mean_score"`
}

var transitionPhrases = []string{
	"first,", "second,", "third,", "next,", "then,",
	"finally,", "in addition", "moreover", "furthermore",
	"however,", "in contrast", "on the other hand",
	"for this reason", "as a result", "in summary",
	"step 1", "step 2", "step 3",
}

var structurePhrases = []string{
	"the following", "listed below", "as follows",
	"there are two", "there are three", "consists of",
	"is divided into", "can be broken down",
	"in particular", "to clarify", "put simply",
	"in other words", "that is,", "namely",
}

var ambiguityPhrases = []string{
	"this thing", "that thing", "such things",
	"the former", "the latter", "the above",
	"as mentioned", "said thing",
}

var repetitionPhrases = []string{
	"as i said", "as mentioned before", "again,",
	"to reiterate", "as stated earlier", "once more",
	"repeating", "as previously noted",
}

var sentenceSplitter = regexp.MustCompile(`[.!?]+\s+`)

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func countDanglingPronouns(text string) int {
	lower := strings.ToLower(text)
	words := strings.Fields(lower)
	count := 0
	pronouns := map[string]bool{"this": true, "that": true, "it": true, "they": true}
	stops := map[string]bool{"is": true, "are": true, "was": true, "were": true, "will": true, "can": true, "should": true, "could": true, "would": true, "has": true, "have": true, "had": true, "does": true, "do": true, "did": true}
	for i, w := range words {
		if pronouns[w] && i+1 < len(words) && stops[words[i+1]] {
			count++
		}
	}
	return count
}

func avgSentenceLength(text string) float64 {
	if len(strings.TrimSpace(text)) == 0 {
		return 0
	}
	sentences := sentenceSplitter.Split(text, -1)
	var nonEmpty []string
	for _, s := range sentences {
		if len(strings.TrimSpace(s)) > 0 {
			nonEmpty = append(nonEmpty, s)
		}
	}
	if len(nonEmpty) == 0 {
		return float64(len(strings.Fields(text)))
	}
	total := 0
	for _, s := range nonEmpty {
		total += len(strings.Fields(s))
	}
	return float64(total) / float64(len(nonEmpty))
}

func AnalyzeText(text string) (float64, ClarityDetail) {
	words := strings.Fields(text)
	wordCount := len(words)
	if wordCount == 0 {
		return 50.0, ClarityDetail{}
	}

	transitions := countPhrases(text, transitionPhrases)
	structure := countPhrases(text, structurePhrases)
	dangling := countDanglingPronouns(text)
	repetitions := countPhrases(text, repetitionPhrases)
	avgLen := avgSentenceLength(text)

	score := 50.0
	score += float64(transitions) * 3.0
	score += float64(structure) * 3.5
	score -= float64(dangling) * 4.0
	score -= float64(repetitions) * 3.0

	if avgLen > 0 && avgLen <= 20 {
		score += 5.0
	} else if avgLen > 30 {
		score -= (avgLen - 30) * 0.5
	}

	density := float64(transitions+structure) / float64(wordCount) * 100
	score += density * 2.0

	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	detail := ClarityDetail{
		TransitionCount: transitions,
		StructureCount:  structure,
		AmbiguityCount:  dangling,
		RepetitionCount: repetitions,
		AvgSentenceLen:  math.Round(avgLen*10) / 10,
		WordCount:       wordCount,
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

	var models []ModelClarity
	for mn, entries := range modelEntries {
		var totalScore float64
		var aggDetail ClarityDetail
		labs := map[string]bool{}

		for _, e := range entries {
			labs[e.Lab] = true
			score, detail := AnalyzeText(e.Summary)
			totalScore += score
			aggDetail.TransitionCount += detail.TransitionCount
			aggDetail.StructureCount += detail.StructureCount
			aggDetail.AmbiguityCount += detail.AmbiguityCount
			aggDetail.RepetitionCount += detail.RepetitionCount
			aggDetail.AvgSentenceLen += detail.AvgSentenceLen
			aggDetail.WordCount += detail.WordCount
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		aggDetail.AvgSentenceLen = math.Round(aggDetail.AvgSentenceLen/n*10) / 10

		models = append(models, ModelClarity{
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
	b.WriteString("Response Clarity Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No clarity data available.\n")
		return b.String()
	}

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s\n", m.Model)
		fmt.Fprintf(&b, "    Score: %.1f  Grade: %s\n", m.Score, m.Grade)
		fmt.Fprintf(&b, "    Transitions: %d  Structure: %d  Ambiguity: %d\n",
			m.Detail.TransitionCount, m.Detail.StructureCount, m.Detail.AmbiguityCount)
		fmt.Fprintf(&b, "    Repetitions: %d  Avg Sentence Len: %.1f\n",
			m.Detail.RepetitionCount, m.Detail.AvgSentenceLen)
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
