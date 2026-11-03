package warmth

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type WarmthDetail struct {
	WarmPhraseCount  int     `json:"warm_phrase_count"`
	ColdPhraseCount  int     `json:"cold_phrase_count"`
	EncouragingCount int     `json:"encouraging_count"`
	DismissiveCount  int     `json:"dismissive_count"`
	WarmthRatio      float64 `json:"warmth_ratio"`
}

type ModelWarmth struct {
	Model       string       `json:"model"`
	WarmthScore float64      `json:"warmth_score"`
	Grade       string       `json:"grade"`
	Detail      WarmthDetail `json:"detail"`
	SampleCount int          `json:"sample_count"`
	LabCount    int          `json:"lab_count"`
}

type Report struct {
	Models     []ModelWarmth `json:"models"`
	BestModel  string        `json:"best_model"`
	WorstModel string        `json:"worst_model"`
	MeanScore  float64       `json:"mean_score"`
}

var warmPhrases = []string{
	"i understand", "that makes sense", "great question",
	"happy to help", "glad to assist", "of course",
	"absolutely", "no problem", "you're welcome",
	"i appreciate", "thank you for", "well done",
	"good point", "that's a great", "i'm here to help",
	"feel free to", "don't hesitate", "i'd be happy to",
}

var coldPhrases = []string{
	"as i said", "i already explained", "that's wrong",
	"you should know", "obviously", "clearly you",
	"that's incorrect", "you failed to", "as stated before",
	"i cannot help", "that is not possible", "you misunderstand",
}

var encouragingPhrases = []string{
	"you're on the right track", "keep going", "nice work",
	"that's a good approach", "you've got this", "well thought out",
	"excellent", "impressive", "that's clever",
	"good thinking", "smart approach", "you're making progress",
}

var dismissivePhrases = []string{
	"that's trivial", "anyone would know", "it's simple",
	"just do it", "this is basic", "that's obvious",
	"not my problem", "figure it out", "that doesn't matter",
	"irrelevant", "that's a waste", "not worth discussing",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(text string) (float64, WarmthDetail) {
	warm := countPhrases(text, warmPhrases)
	cold := countPhrases(text, coldPhrases)
	encouraging := countPhrases(text, encouragingPhrases)
	dismissive := countPhrases(text, dismissivePhrases)

	totalSignals := warm + cold + encouraging + dismissive
	warmthRatio := 0.0
	if totalSignals > 0 {
		warmthRatio = float64(warm+encouraging) / float64(totalSignals)
	}

	detail := WarmthDetail{
		WarmPhraseCount:  warm,
		ColdPhraseCount:  cold,
		EncouragingCount: encouraging,
		DismissiveCount:  dismissive,
		WarmthRatio:      math.Round(warmthRatio*100) / 100,
	}

	score := 50.0
	score += float64(warm) * 3.0
	score += float64(encouraging) * 4.0
	score -= float64(cold) * 5.0
	score -= float64(dismissive) * 6.0

	words := strings.Fields(text)
	if len(words) > 0 {
		density := float64(warm+encouraging) / float64(len(words)) * 100
		score += density * 2.0
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

	var models []ModelWarmth
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail WarmthDetail
		labs := map[string]bool{}

		for _, e := range entries {
			labs[e.Lab] = true
			score, detail := scoreResponse(e.Summary)
			totalScore += score
			totalDetail.WarmPhraseCount += detail.WarmPhraseCount
			totalDetail.ColdPhraseCount += detail.ColdPhraseCount
			totalDetail.EncouragingCount += detail.EncouragingCount
			totalDetail.DismissiveCount += detail.DismissiveCount
			totalDetail.WarmthRatio += detail.WarmthRatio
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.WarmthRatio = math.Round(totalDetail.WarmthRatio/n*100) / 100

		models = append(models, ModelWarmth{
			Model:       mn,
			WarmthScore: math.Round(avgScore*10) / 10,
			Grade:       scoreToGrade(avgScore),
			Detail:      totalDetail,
			SampleCount: len(entries),
			LabCount:    len(labs),
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].WarmthScore > models[j].WarmthScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.WarmthScore
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
	b.WriteString("Response Warmth Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No warmth data available.\n")
		return b.String()
	}

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s\n", m.Model)
		fmt.Fprintf(&b, "    Score: %.1f  Grade: %s\n", m.WarmthScore, m.Grade)
		fmt.Fprintf(&b, "    Warm: %d  Cold: %d  Encouraging: %d  Dismissive: %d\n",
			m.Detail.WarmPhraseCount, m.Detail.ColdPhraseCount,
			m.Detail.EncouragingCount, m.Detail.DismissiveCount)
		fmt.Fprintf(&b, "    Warmth Ratio: %.2f  Samples: %d  Labs: %d\n\n",
			m.Detail.WarmthRatio, m.SampleCount, m.LabCount)
	}

	fmt.Fprintf(&b, "  Best: %s  Worst: %s  Mean: %.1f\n",
		r.BestModel, r.WorstModel, r.MeanScore)

	return b.String()
}
