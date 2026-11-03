package courtesy

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type CourtesyDetail struct {
	PoliteCount      int     `json:"polite_count"`
	RudeCount        int     `json:"rude_count"`
	AcknowledgeCount int     `json:"acknowledge_count"`
	DismissiveCount  int     `json:"dismissive_count"`
	EncourageCount   int     `json:"encourage_count"`
	WordCount        int     `json:"word_count"`
	CourtesyRatio    float64 `json:"courtesy_ratio"`
}

type ModelCourtesy struct {
	Model       string         `json:"model"`
	Score       float64        `json:"score"`
	Grade       string         `json:"grade"`
	Detail      CourtesyDetail `json:"detail"`
	SampleCount int            `json:"sample_count"`
	LabCount    int            `json:"lab_count"`
}

type Report struct {
	Models     []ModelCourtesy `json:"models"`
	BestModel  string          `json:"best_model"`
	WorstModel string          `json:"worst_model"`
	MeanScore  float64         `json:"mean_score"`
}

var politePhrases = []string{
	"please", "thank you", "thanks for", "i appreciate",
	"you're welcome", "glad to help", "happy to assist",
	"would you like", "if you prefer", "feel free to",
	"at your convenience", "when you're ready", "no rush",
	"i hope this helps", "let me know if",
}

var rudePhrases = []string{
	"that's wrong", "you should have", "you failed to",
	"that's obvious", "as i already said", "i already told you",
	"you clearly don't", "that makes no sense", "you're confused",
	"that's ridiculous", "you need to", "you must",
	"stop doing", "don't bother", "that's not my problem",
}

var acknowledgePhrases = []string{
	"great question", "good point", "i understand",
	"that's a fair point", "you raise a good", "i see what you mean",
	"that makes sense", "understandable", "valid concern",
	"i hear you", "absolutely", "you're right",
	"that's correct", "well observed", "good catch",
}

var dismissivePhrases = []string{
	"just do", "simply", "just use", "it's easy",
	"it's trivial", "it's straightforward", "anyone can",
	"you just need to", "all you have to do", "it's not that hard",
	"it's not complicated", "it's basic", "obviously just",
}

var encouragePhrases = []string{
	"you're on the right track", "good approach", "nice work",
	"well done", "that's a good start", "you're making progress",
	"keep going", "almost there", "getting closer",
	"that's promising", "solid foundation", "nice thinking",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func AnalyzeText(text string) (float64, CourtesyDetail) {
	words := strings.Fields(text)
	wordCount := len(words)

	if wordCount == 0 {
		return 50.0, CourtesyDetail{}
	}

	polite := countPhrases(text, politePhrases)
	rude := countPhrases(text, rudePhrases)
	acknowledge := countPhrases(text, acknowledgePhrases)
	dismissive := countPhrases(text, dismissivePhrases)
	encourage := countPhrases(text, encouragePhrases)

	positiveSignals := float64(polite + acknowledge + encourage)
	negativeSignals := float64(rude + dismissive)
	totalSignals := positiveSignals + negativeSignals

	courtesyRatio := 0.5
	if totalSignals > 0 {
		courtesyRatio = positiveSignals / totalSignals
	}

	score := 50.0
	score += float64(polite) * 3.0
	score += float64(acknowledge) * 4.0
	score += float64(encourage) * 3.5
	score -= float64(rude) * 5.0
	score -= float64(dismissive) * 4.0

	density := positiveSignals / float64(wordCount) * 100
	score += density * 2.0

	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	detail := CourtesyDetail{
		PoliteCount:      polite,
		RudeCount:        rude,
		AcknowledgeCount: acknowledge,
		DismissiveCount:  dismissive,
		EncourageCount:   encourage,
		WordCount:        wordCount,
		CourtesyRatio:    math.Round(courtesyRatio*100) / 100,
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

	var models []ModelCourtesy
	for mn, entries := range modelEntries {
		var totalScore float64
		var aggDetail CourtesyDetail
		labs := map[string]bool{}

		for _, e := range entries {
			labs[e.Lab] = true
			score, detail := AnalyzeText(e.Summary)
			totalScore += score
			aggDetail.PoliteCount += detail.PoliteCount
			aggDetail.RudeCount += detail.RudeCount
			aggDetail.AcknowledgeCount += detail.AcknowledgeCount
			aggDetail.DismissiveCount += detail.DismissiveCount
			aggDetail.EncourageCount += detail.EncourageCount
			aggDetail.WordCount += detail.WordCount
			aggDetail.CourtesyRatio += detail.CourtesyRatio
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		aggDetail.CourtesyRatio = math.Round(aggDetail.CourtesyRatio/n*100) / 100

		models = append(models, ModelCourtesy{
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
	b.WriteString("Response Courtesy Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No courtesy data available.\n")
		return b.String()
	}

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s\n", m.Model)
		fmt.Fprintf(&b, "    Score: %.1f  Grade: %s\n", m.Score, m.Grade)
		fmt.Fprintf(&b, "    Polite: %d  Rude: %d  Acknowledge: %d\n",
			m.Detail.PoliteCount, m.Detail.RudeCount, m.Detail.AcknowledgeCount)
		fmt.Fprintf(&b, "    Dismissive: %d  Encourage: %d  Ratio: %.2f\n",
			m.Detail.DismissiveCount, m.Detail.EncourageCount, m.Detail.CourtesyRatio)
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
