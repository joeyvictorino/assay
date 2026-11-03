package engagement

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type EngagementDetail struct {
	QuestionUse   float64 `json:"question_use"`
	Interactivity float64 `json:"interactivity"`
	Personalize   float64 `json:"personalization"`
	Encouragement float64 `json:"encouragement"`
	Clarity       float64 `json:"clarity"`
	QuestionCount int     `json:"question_count"`
	PromptCount   int     `json:"prompt_count"`
	BoringCount   int     `json:"boring_count"`
}

type ModelEngagement struct {
	Model           string           `json:"model"`
	EngagementScore float64          `json:"engagement_score"`
	Grade           string           `json:"grade"`
	Detail          EngagementDetail `json:"detail"`
	SampleCount     int              `json:"sample_count"`
	LabCount        int              `json:"lab_count"`
	Concerns        int              `json:"concerns"`
}

type Report struct {
	Models     []ModelEngagement `json:"models"`
	BestModel  string            `json:"best_model"`
	WorstModel string            `json:"worst_model"`
	MeanScore  float64           `json:"mean_score"`
}

var questionPhrases = []string{
	"would you like", "shall i", "do you want",
	"does that help", "any questions",
	"what do you think", "how does that sound",
	"would that work", "shall we",
	"interested in", "curious about",
}

var interactivePhrases = []string{
	"let's try", "let's explore", "let's look at",
	"follow along", "try this", "give it a shot",
	"experiment with", "play around",
	"hands-on", "step by step",
	"walk through", "work through",
}

var personalizePhrases = []string{
	"in your case", "for your situation",
	"based on what you", "given your",
	"tailored to", "specific to your",
	"your particular", "customized for",
	"considering your", "as you mentioned",
}

var encouragePhrases = []string{
	"great question", "good thinking",
	"that's a smart", "excellent point",
	"well done", "nice work",
	"you're on the right track", "that's a great start",
	"keep going", "you've got this",
	"impressive", "brilliant",
}

var boringPhrases = []string{
	"as per the documentation", "please refer to",
	"the standard approach", "the typical method",
	"commonly used", "the usual way",
	"the default behavior", "as expected",
	"per the specification", "according to standard",
}

var clarityPhrases = []string{
	"in summary", "to summarize", "the key takeaway",
	"put simply", "in plain terms", "bottom line",
	"the main point", "most importantly",
	"the crucial thing", "what matters most",
	"tl;dr", "in a nutshell",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, EngagementDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	questions := countPhrases(combined, questionPhrases)
	interact := countPhrases(combined, interactivePhrases)
	personal := countPhrases(combined, personalizePhrases)
	encourage := countPhrases(combined, encouragePhrases)
	boring := countPhrases(combined, boringPhrases)
	clarity := countPhrases(combined, clarityPhrases)

	qScore := 0.4
	if words > 0 {
		qScore = math.Min(1.0, float64(questions)*2.0/float64(words)*10.0+0.4)
	}

	interactScore := 0.4
	if words > 0 {
		interactScore = math.Min(1.0, float64(interact)*2.0/float64(words)*10.0+0.4)
	}

	personalScore := 0.3
	if personal > 0 {
		personalScore = math.Min(1.0, float64(personal)*0.25+0.3)
	}

	encourageScore := 0.4
	if encourage > 0 {
		encourageScore = math.Min(1.0, float64(encourage)*0.2+0.4)
	}

	clarityScore := 0.4
	if clarity > 0 {
		clarityScore = math.Min(1.0, float64(clarity)*0.2+0.4)
	}

	promptCount := questions + interact

	detail := EngagementDetail{
		QuestionUse:   math.Round(qScore*100) / 100,
		Interactivity: math.Round(interactScore*100) / 100,
		Personalize:   math.Round(personalScore*100) / 100,
		Encouragement: math.Round(encourageScore*100) / 100,
		Clarity:       math.Round(clarityScore*100) / 100,
		QuestionCount: questions,
		PromptCount:   promptCount,
		BoringCount:   boring,
	}

	score := 38.0
	score += qScore * 12.0
	score += interactScore * 10.0
	score += personalScore * 12.0
	score += encourageScore * 10.0
	score += clarityScore * 10.0

	score -= float64(boring) * 3.0

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

	var models []ModelEngagement
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail EngagementDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.QuestionUse += detail.QuestionUse
			totalDetail.Interactivity += detail.Interactivity
			totalDetail.Personalize += detail.Personalize
			totalDetail.Encouragement += detail.Encouragement
			totalDetail.Clarity += detail.Clarity
			totalDetail.QuestionCount += detail.QuestionCount
			totalDetail.PromptCount += detail.PromptCount
			totalDetail.BoringCount += detail.BoringCount

			if detail.BoringCount > 2 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.QuestionUse = math.Round(totalDetail.QuestionUse/n*100) / 100
		totalDetail.Interactivity = math.Round(totalDetail.Interactivity/n*100) / 100
		totalDetail.Personalize = math.Round(totalDetail.Personalize/n*100) / 100
		totalDetail.Encouragement = math.Round(totalDetail.Encouragement/n*100) / 100
		totalDetail.Clarity = math.Round(totalDetail.Clarity/n*100) / 100

		models = append(models, ModelEngagement{
			Model:           mn,
			EngagementScore: math.Round(avgScore*10) / 10,
			Grade:           scoreToGrade(avgScore),
			Detail:          totalDetail,
			SampleCount:     len(entries),
			LabCount:        len(labs),
			Concerns:        totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].EngagementScore > models[j].EngagementScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.EngagementScore
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
	b.WriteString("Response Engagement Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No engagement data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.EngagementScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "question_use", m.Detail.QuestionUse)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "interactivity", m.Detail.Interactivity)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "personalization", m.Detail.Personalize)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "encouragement", m.Detail.Encouragement)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "clarity", m.Detail.Clarity)
		fmt.Fprintf(&b, "    %-28s %d\n", "question_count", m.Detail.QuestionCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "prompt_count", m.Detail.PromptCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "boring_count", m.Detail.BoringCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
