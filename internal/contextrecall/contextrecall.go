package contextrecall

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

var recallSignals = []string{
	"as mentioned", "you said", "you asked", "your earlier",
	"referring to", "in reference to", "previously stated",
	"in your message", "your original", "you noted",
	"earlier in", "from above", "as discussed",
	"your prior", "recalling your", "based on what you said",
}

var contextRetention = []string{
	"context", "conversation", "thread", "discussion",
	"earlier turn", "previous message", "above question",
	"initial request", "original prompt", "first message",
	"prior exchange", "history", "earlier interaction",
}

var accurateCitation = []string{
	"you specifically", "your exact", "to quote",
	"verbatim", "word for word", "precisely",
	"accurately", "correctly recalled", "faithfully",
	"as you put it", "in your words", "your term",
}

var inferenceSignals = []string{
	"implies", "suggests", "inferred from",
	"based on context", "contextually", "reading between",
	"extrapolating", "given that you", "it follows",
	"deduced from", "understood from", "gathered from",
}

var confusionSignals = []string{
	"i apologize for the confusion", "let me correct",
	"i misunderstood", "i confused", "that was incorrect",
	"wrong context", "mixed up", "conflated",
	"misremembered", "incorrectly recalled", "my mistake",
	"i was wrong about", "correction needed",
}

var hallRecallSignals = []string{
	"you mentioned that", "as you said earlier",
	"you previously asked about", "in your last message you",
	"your earlier point about", "recalling what you said about",
}

type RecallDetail struct {
	DirectRecall       float64 `json:"direct_recall"`
	ContextRetention   float64 `json:"context_retention"`
	CitationAccuracy   float64 `json:"citation_accuracy"`
	InferenceQuality   float64 `json:"inference_quality"`
	ConfusionRate      float64 `json:"confusion_rate"`
	RecallSignalCount  int     `json:"recall_signal_count"`
	ConfusionCount     int     `json:"confusion_count"`
	HallucinatedRecall int     `json:"hallucinated_recall"`
}

type ModelRecall struct {
	Model       string       `json:"model"`
	RecallScore float64      `json:"recall_score"`
	Grade       string       `json:"grade"`
	Detail      RecallDetail `json:"detail"`
	SampleCount int          `json:"sample_count"`
	LabCount    int          `json:"lab_count"`
	Concerns    int          `json:"concerns"`
}

type Report struct {
	Models     []ModelRecall `json:"models"`
	BestModel  string        `json:"best_model"`
	WorstModel string        `json:"worst_model"`
	MeanScore  float64       `json:"mean_score"`
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, RecallDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	recallHits := countPhrases(combined, recallSignals)
	retentionHits := countPhrases(combined, contextRetention)
	citationHits := countPhrases(combined, accurateCitation)
	inferenceHits := countPhrases(combined, inferenceSignals)
	confusionHits := countPhrases(combined, confusionSignals)
	hallHits := countPhrases(combined, hallRecallSignals)

	recallScore := 0.2
	if words > 0 {
		recallScore = math.Min(1.0, float64(recallHits)*2.0/float64(words)*10.0+0.2)
	}

	retentionScore := 0.2
	if words > 0 {
		retentionScore = math.Min(1.0, float64(retentionHits)*2.0/float64(words)*10.0+0.2)
	}

	citationScore := 0.2
	if citationHits > 0 {
		citationScore = math.Min(1.0, float64(citationHits)*0.3+0.2)
	}

	inferenceScore := 0.3
	if inferenceHits > 0 {
		inferenceScore = math.Min(1.0, float64(inferenceHits)*0.25+0.3)
	}

	confusionRate := 0.0
	if words > 0 {
		confusionRate = math.Min(1.0, float64(confusionHits)*3.0/float64(words)*10.0)
	}

	detail := RecallDetail{
		DirectRecall:       math.Round(recallScore*100) / 100,
		ContextRetention:   math.Round(retentionScore*100) / 100,
		CitationAccuracy:   math.Round(citationScore*100) / 100,
		InferenceQuality:   math.Round(inferenceScore*100) / 100,
		ConfusionRate:      math.Round(confusionRate*100) / 100,
		RecallSignalCount:  recallHits + retentionHits,
		ConfusionCount:     confusionHits,
		HallucinatedRecall: hallHits,
	}

	score := 30.0
	score += recallScore * 18.0
	score += retentionScore * 15.0
	score += citationScore * 12.0
	score += inferenceScore * 10.0

	score -= confusionRate * 20.0
	score -= float64(hallHits) * 3.0

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

	var models []ModelRecall
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail RecallDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.DirectRecall += detail.DirectRecall
			totalDetail.ContextRetention += detail.ContextRetention
			totalDetail.CitationAccuracy += detail.CitationAccuracy
			totalDetail.InferenceQuality += detail.InferenceQuality
			totalDetail.ConfusionRate += detail.ConfusionRate
			totalDetail.RecallSignalCount += detail.RecallSignalCount
			totalDetail.ConfusionCount += detail.ConfusionCount
			totalDetail.HallucinatedRecall += detail.HallucinatedRecall

			if detail.ConfusionCount > 2 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.DirectRecall = math.Round(totalDetail.DirectRecall/n*100) / 100
		totalDetail.ContextRetention = math.Round(totalDetail.ContextRetention/n*100) / 100
		totalDetail.CitationAccuracy = math.Round(totalDetail.CitationAccuracy/n*100) / 100
		totalDetail.InferenceQuality = math.Round(totalDetail.InferenceQuality/n*100) / 100
		totalDetail.ConfusionRate = math.Round(totalDetail.ConfusionRate/n*100) / 100

		models = append(models, ModelRecall{
			Model:       mn,
			RecallScore: math.Round(avgScore*10) / 10,
			Grade:       scoreToGrade(avgScore),
			Detail:      totalDetail,
			SampleCount: len(entries),
			LabCount:    len(labs),
			Concerns:    totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].RecallScore > models[j].RecallScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.RecallScore
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
	b.WriteString("Context Recall Accuracy Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No context recall data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.RecallScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "direct_recall", m.Detail.DirectRecall)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "context_retention", m.Detail.ContextRetention)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "citation_accuracy", m.Detail.CitationAccuracy)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "inference_quality", m.Detail.InferenceQuality)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "confusion_rate", m.Detail.ConfusionRate)
		fmt.Fprintf(&b, "    %-28s %d\n", "recall_signals", m.Detail.RecallSignalCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "confusion_count", m.Detail.ConfusionCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "hallucinated_recall", m.Detail.HallucinatedRecall)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
