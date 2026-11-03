package contextrelevance

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type RelevanceDetail struct {
	TopicAlignment   float64 `json:"topic_alignment"`
	ContextUse       float64 `json:"context_use"`
	FocusScore       float64 `json:"focus_score"`
	SignalNoiseRatio float64 `json:"signal_noise_ratio"`
	DistractionScore float64 `json:"distraction_score"`
	OnTopicCount     int     `json:"on_topic_count"`
	OffTopicCount    int     `json:"off_topic_count"`
	TangentCount     int     `json:"tangent_count"`
}

type ModelRelevance struct {
	Model          string          `json:"model"`
	RelevanceScore float64         `json:"relevance_score"`
	Grade          string          `json:"grade"`
	Detail         RelevanceDetail `json:"detail"`
	SampleCount    int             `json:"sample_count"`
	LabCount       int             `json:"lab_count"`
	Concerns       int             `json:"concerns"`
}

type Report struct {
	Models     []ModelRelevance `json:"models"`
	BestModel  string           `json:"best_model"`
	WorstModel string           `json:"worst_model"`
	MeanScore  float64          `json:"mean_score"`
}

var alignmentPhrases = []string{
	"as you asked", "to answer your question",
	"regarding your question", "in response to",
	"you asked about", "your query about",
	"the question was", "to address this",
	"directly answering", "the specific question",
	"relevant to your", "pertaining to",
}

var contextUsePhrases = []string{
	"based on the context", "given the information",
	"from the provided", "as mentioned",
	"referring to", "according to the input",
	"the context shows", "given that",
	"considering the", "in light of",
	"taking into account", "with reference to",
	"as stated", "the data indicates",
}

var focusPhrases = []string{
	"the key point is", "the main issue",
	"the core question", "fundamentally",
	"the crux", "most importantly",
	"the primary concern", "the central",
	"at its core", "the essential",
	"the critical factor", "the decisive",
}

var onTopicPhrases = []string{
	"specifically", "precisely",
	"exactly", "to be exact",
	"this directly", "this relates to",
	"the answer is", "the solution is",
	"the relevant", "applicable here",
	"in this case", "for this scenario",
}

var offTopicPhrases = []string{
	"on a related note", "by the way",
	"speaking of which", "incidentally",
	"as an aside", "tangentially",
	"while we're at it", "unrelated but",
	"off topic but", "side note",
	"not directly related", "loosely related",
}

var tangentPhrases = []string{
	"let me explain something else",
	"before i answer", "first let me",
	"some background", "to understand this",
	"a brief history", "going back to",
	"in general terms", "broadly speaking",
	"to give you context", "stepping back",
	"zooming out", "the bigger picture",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, RelevanceDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	alignment := countPhrases(combined, alignmentPhrases)
	ctxUse := countPhrases(combined, contextUsePhrases)
	focus := countPhrases(combined, focusPhrases)
	onTopic := countPhrases(combined, onTopicPhrases)
	offTopic := countPhrases(combined, offTopicPhrases)
	tangent := countPhrases(combined, tangentPhrases)

	relevantCount := alignment + ctxUse + focus + onTopic

	alignScore := 0.2
	if words > 0 {
		alignScore = math.Min(1.0, float64(alignment)*2.0/float64(words)*10.0+0.2)
	}

	ctxScore := 0.2
	if words > 0 {
		ctxScore = math.Min(1.0, float64(ctxUse)*2.0/float64(words)*10.0+0.2)
	}

	focusScore := 0.3
	if focus > 0 || onTopic > 0 {
		focusScore = math.Min(1.0, float64(focus+onTopic)*0.15+0.3)
	}

	snrScore := 0.2
	if words > 0 {
		snrScore = math.Min(1.0, float64(relevantCount)*2.0/float64(words)*10.0+0.2)
	}

	distractScore := 0.2
	if offTopic == 0 && tangent == 0 {
		distractScore = 0.8
	} else if offTopic+tangent < 3 {
		distractScore = 0.5
	}

	detail := RelevanceDetail{
		TopicAlignment:   math.Round(alignScore*100) / 100,
		ContextUse:       math.Round(ctxScore*100) / 100,
		FocusScore:       math.Round(focusScore*100) / 100,
		SignalNoiseRatio: math.Round(snrScore*100) / 100,
		DistractionScore: math.Round(distractScore*100) / 100,
		OnTopicCount:     relevantCount,
		OffTopicCount:    offTopic,
		TangentCount:     tangent,
	}

	score := 30.0
	score += alignScore * 15.0
	score += ctxScore * 15.0
	score += focusScore * 10.0
	score += snrScore * 10.0
	score += distractScore * 10.0

	score -= float64(offTopic) * 4.0
	score -= float64(tangent) * 5.0

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

	var models []ModelRelevance
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail RelevanceDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.TopicAlignment += detail.TopicAlignment
			totalDetail.ContextUse += detail.ContextUse
			totalDetail.FocusScore += detail.FocusScore
			totalDetail.SignalNoiseRatio += detail.SignalNoiseRatio
			totalDetail.DistractionScore += detail.DistractionScore
			totalDetail.OnTopicCount += detail.OnTopicCount
			totalDetail.OffTopicCount += detail.OffTopicCount
			totalDetail.TangentCount += detail.TangentCount

			if detail.OffTopicCount > 3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.TopicAlignment = math.Round(totalDetail.TopicAlignment/n*100) / 100
		totalDetail.ContextUse = math.Round(totalDetail.ContextUse/n*100) / 100
		totalDetail.FocusScore = math.Round(totalDetail.FocusScore/n*100) / 100
		totalDetail.SignalNoiseRatio = math.Round(totalDetail.SignalNoiseRatio/n*100) / 100
		totalDetail.DistractionScore = math.Round(totalDetail.DistractionScore/n*100) / 100

		models = append(models, ModelRelevance{
			Model:          mn,
			RelevanceScore: math.Round(avgScore*10) / 10,
			Grade:          scoreToGrade(avgScore),
			Detail:         totalDetail,
			SampleCount:    len(entries),
			LabCount:       len(labs),
			Concerns:       totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].RelevanceScore > models[j].RelevanceScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.RelevanceScore
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
	b.WriteString("Context Relevance Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No context relevance data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.RelevanceScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "topic_alignment", m.Detail.TopicAlignment)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "context_use", m.Detail.ContextUse)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "focus_score", m.Detail.FocusScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "signal_noise_ratio", m.Detail.SignalNoiseRatio)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "distraction_score", m.Detail.DistractionScore)
		fmt.Fprintf(&b, "    %-28s %d\n", "on_topic_count", m.Detail.OnTopicCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "off_topic_count", m.Detail.OffTopicCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "tangent_count", m.Detail.TangentCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
