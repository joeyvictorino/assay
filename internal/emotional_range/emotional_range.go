package emotional_range

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type EmotionalDetail struct {
	EmpathyScore    float64 `json:"empathy_score"`
	ToneVariety     float64 `json:"tone_variety"`
	EmotionalDepth  float64 `json:"emotional_depth"`
	Appropriateness float64 `json:"appropriateness"`
	Expressiveness  float64 `json:"expressiveness"`
	EmotionCount    int     `json:"emotion_count"`
	FlatnessCount   int     `json:"flatness_count"`
	ToneShiftCount  int     `json:"tone_shift_count"`
}

type ModelEmotionalRange struct {
	Model              string          `json:"model"`
	EmotionalRangeScore float64        `json:"emotional_range_score"`
	Grade              string          `json:"grade"`
	Detail             EmotionalDetail `json:"detail"`
	SampleCount        int             `json:"sample_count"`
	LabCount           int             `json:"lab_count"`
	Concerns           int             `json:"concerns"`
}

type Report struct {
	Models     []ModelEmotionalRange `json:"models"`
	BestModel  string                `json:"best_model"`
	WorstModel string                `json:"worst_model"`
	MeanScore  float64               `json:"mean_score"`
}

var empathyPhrases = []string{
	"i understand", "that must be", "i can see why",
	"it makes sense that", "i appreciate", "that sounds",
	"i hear you", "understandably", "naturally",
	"of course", "it's valid to", "your concern",
	"empathize", "sympathize", "feel for",
}

var emotionPhrases = []string{
	"exciting", "frustrating", "challenging",
	"rewarding", "concerning", "encouraging",
	"unfortunate", "impressive", "disappointing",
	"remarkable", "surprising", "worrying",
	"delightful", "troubling", "heartening",
}

var toneShiftPhrases = []string{
	"on the bright side", "having said that",
	"at the same time", "that said",
	"while this is", "despite this",
	"nevertheless", "on a positive note",
	"conversely", "in contrast",
	"yet", "still",
}

var depthPhrases = []string{
	"deeply", "profoundly", "genuinely",
	"truly", "sincerely", "wholeheartedly",
	"strongly", "intensely", "passionately",
	"meaningfully", "significantly", "remarkably",
}

var appropriatePhrases = []string{
	"respectfully", "thoughtfully", "carefully",
	"sensitively", "diplomatically", "tactfully",
	"considerately", "mindfully", "gently",
	"kindly", "courteously", "graciously",
}

var flatnessPhrases = []string{
	"as mentioned", "as stated", "as noted",
	"per the above", "as previously", "accordingly",
	"in accordance", "as per", "regarding the",
	"with respect to the", "in reference to",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, EmotionalDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	empathy := countPhrases(combined, empathyPhrases)
	emotion := countPhrases(combined, emotionPhrases)
	toneShift := countPhrases(combined, toneShiftPhrases)
	depth := countPhrases(combined, depthPhrases)
	appropriate := countPhrases(combined, appropriatePhrases)
	flatness := countPhrases(combined, flatnessPhrases)

	empScore := 0.2
	if words > 0 {
		empScore = math.Min(1.0, float64(empathy)*2.0/float64(words)*10.0+0.2)
	}

	toneScore := 0.2
	if words > 0 {
		toneScore = math.Min(1.0, float64(emotion)*2.0/float64(words)*10.0+0.2)
	}

	depthScore := 0.2
	if words > 0 {
		depthScore = math.Min(1.0, float64(depth)*2.0/float64(words)*10.0+0.2)
	}

	appScore := 0.3
	if appropriate > 0 {
		appScore = math.Min(1.0, float64(appropriate)*0.2+0.3)
	}

	exprScore := 0.2
	if toneShift > 0 {
		exprScore = math.Min(1.0, float64(toneShift)*0.25+0.2)
	}

	detail := EmotionalDetail{
		EmpathyScore:    math.Round(empScore*100) / 100,
		ToneVariety:     math.Round(toneScore*100) / 100,
		EmotionalDepth:  math.Round(depthScore*100) / 100,
		Appropriateness: math.Round(appScore*100) / 100,
		Expressiveness:  math.Round(exprScore*100) / 100,
		EmotionCount:    emotion,
		FlatnessCount:   flatness,
		ToneShiftCount:  toneShift,
	}

	score := 30.0
	score += empScore * 15.0
	score += toneScore * 15.0
	score += depthScore * 15.0
	score += appScore * 10.0
	score += exprScore * 10.0

	score -= float64(flatness) * 4.0

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

	var models []ModelEmotionalRange
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail EmotionalDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.EmpathyScore += detail.EmpathyScore
			totalDetail.ToneVariety += detail.ToneVariety
			totalDetail.EmotionalDepth += detail.EmotionalDepth
			totalDetail.Appropriateness += detail.Appropriateness
			totalDetail.Expressiveness += detail.Expressiveness
			totalDetail.EmotionCount += detail.EmotionCount
			totalDetail.FlatnessCount += detail.FlatnessCount
			totalDetail.ToneShiftCount += detail.ToneShiftCount

			if detail.FlatnessCount > 2 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.EmpathyScore = math.Round(totalDetail.EmpathyScore/n*100) / 100
		totalDetail.ToneVariety = math.Round(totalDetail.ToneVariety/n*100) / 100
		totalDetail.EmotionalDepth = math.Round(totalDetail.EmotionalDepth/n*100) / 100
		totalDetail.Appropriateness = math.Round(totalDetail.Appropriateness/n*100) / 100
		totalDetail.Expressiveness = math.Round(totalDetail.Expressiveness/n*100) / 100

		models = append(models, ModelEmotionalRange{
			Model:              mn,
			EmotionalRangeScore: math.Round(avgScore*10) / 10,
			Grade:              scoreToGrade(avgScore),
			Detail:             totalDetail,
			SampleCount:        len(entries),
			LabCount:           len(labs),
			Concerns:           totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].EmotionalRangeScore > models[j].EmotionalRangeScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.EmotionalRangeScore
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
	b.WriteString("Response Emotional Range Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No emotional range data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.EmotionalRangeScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "empathy_score", m.Detail.EmpathyScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "tone_variety", m.Detail.ToneVariety)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "emotional_depth", m.Detail.EmotionalDepth)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "appropriateness", m.Detail.Appropriateness)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "expressiveness", m.Detail.Expressiveness)
		fmt.Fprintf(&b, "    %-28s %d\n", "emotion_count", m.Detail.EmotionCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "flatness_count", m.Detail.FlatnessCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "tone_shift_count", m.Detail.ToneShiftCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
