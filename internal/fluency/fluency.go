package fluency

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type FluencyDetail struct {
	Naturalness  float64 `json:"naturalness"`
	GrammarScore float64 `json:"grammar_score"`
	FlowScore    float64 `json:"flow_score"`
	VocabRange   float64 `json:"vocab_range"`
	Readability  float64 `json:"readability"`
	FluentCount  int     `json:"fluent_count"`
	AwkwardCount int     `json:"awkward_count"`
	FillerCount  int     `json:"filler_count"`
}

type ModelFluency struct {
	Model        string        `json:"model"`
	FluencyScore float64       `json:"fluency_score"`
	Grade        string        `json:"grade"`
	Detail       FluencyDetail `json:"detail"`
	SampleCount  int           `json:"sample_count"`
	LabCount     int           `json:"lab_count"`
	Issues       int           `json:"issues"`
}

type Report struct {
	Models     []ModelFluency `json:"models"`
	BestModel  string         `json:"best_model"`
	WorstModel string         `json:"worst_model"`
	MeanScore  float64        `json:"mean_score"`
}

var naturalPhrases = []string{
	"for example", "in other words", "that is",
	"specifically", "to put it simply", "essentially",
	"in short", "to clarify", "to be more precise",
	"think of it as", "imagine", "consider",
}

var grammarPhrases = []string{
	"which means", "because", "since", "although",
	"therefore", "however", "moreover", "furthermore",
	"consequently", "accordingly", "thus",
	"in contrast", "on the other hand",
}

var flowPhrases = []string{
	"first", "second", "next", "then", "finally",
	"to begin", "moving on", "building on that",
	"with that in mind", "as a result",
	"following from", "this leads to",
}

var vocabPhrases = []string{
	"nuanced", "sophisticated", "comprehensive",
	"multifaceted", "intricate", "paradigm",
	"framework", "methodology", "substantive",
	"fundamental", "underlying", "contextual",
}

var readabilityPhrases = []string{
	"simply put", "in plain terms", "basically",
	"straightforward", "easy to understand",
	"clear", "accessible", "plain language",
	"uncomplicated", "intuitive",
}

var awkwardPhrases = []string{
	"as an ai", "as a language model",
	"i don't have personal", "i cannot feel",
	"as per my training", "based on my training data",
	"my knowledge cutoff", "i was trained",
}

var fillerPhrases = []string{
	"it is important to note that",
	"it should be noted that",
	"it is worth mentioning",
	"it goes without saying",
	"needless to say",
	"as you may know",
	"as we all know",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, FluencyDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	nat := countPhrases(combined, naturalPhrases)
	gram := countPhrases(combined, grammarPhrases)
	flow := countPhrases(combined, flowPhrases)
	vocab := countPhrases(combined, vocabPhrases)
	read := countPhrases(combined, readabilityPhrases)
	awk := countPhrases(combined, awkwardPhrases)
	fill := countPhrases(combined, fillerPhrases)

	natScore := 0.4
	if words > 0 {
		natScore = math.Min(1.0, float64(nat)*2.0/float64(words)*10.0+0.4)
	}
	gramScore := 0.4
	if gram > 0 {
		gramScore = math.Min(1.0, float64(gram)*0.12+0.4)
	}
	flowScore := 0.3
	if flow > 0 {
		flowScore = math.Min(1.0, float64(flow)*0.12+0.3)
	}
	vocabScore := 0.3
	if vocab > 0 {
		vocabScore = math.Min(1.0, float64(vocab)*0.15+0.3)
	}
	readScore := 0.4
	if read > 0 {
		readScore = math.Min(1.0, float64(read)*0.2+0.4)
	}

	fluentCount := nat + gram + flow
	detail := FluencyDetail{
		Naturalness:  math.Round(natScore*100) / 100,
		GrammarScore: math.Round(gramScore*100) / 100,
		FlowScore:    math.Round(flowScore*100) / 100,
		VocabRange:   math.Round(vocabScore*100) / 100,
		Readability:  math.Round(readScore*100) / 100,
		FluentCount:  fluentCount,
		AwkwardCount: awk,
		FillerCount:  fill,
	}

	score := 40.0
	score += natScore * 12.0
	score += gramScore * 10.0
	score += flowScore * 10.0
	score += vocabScore * 8.0
	score += readScore * 10.0
	score -= float64(awk) * 3.0
	score -= float64(fill) * 2.0

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
	var models []ModelFluency
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail FluencyDetail
		totalIssues := 0
		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.Naturalness += detail.Naturalness
			totalDetail.GrammarScore += detail.GrammarScore
			totalDetail.FlowScore += detail.FlowScore
			totalDetail.VocabRange += detail.VocabRange
			totalDetail.Readability += detail.Readability
			totalDetail.FluentCount += detail.FluentCount
			totalDetail.AwkwardCount += detail.AwkwardCount
			totalDetail.FillerCount += detail.FillerCount
			if detail.AwkwardCount > 0 || detail.FillerCount > 0 {
				totalIssues++
			}
		}
		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.Naturalness = math.Round(totalDetail.Naturalness/n*100) / 100
		totalDetail.GrammarScore = math.Round(totalDetail.GrammarScore/n*100) / 100
		totalDetail.FlowScore = math.Round(totalDetail.FlowScore/n*100) / 100
		totalDetail.VocabRange = math.Round(totalDetail.VocabRange/n*100) / 100
		totalDetail.Readability = math.Round(totalDetail.Readability/n*100) / 100
		models = append(models, ModelFluency{
			Model:        mn,
			FluencyScore: math.Round(avgScore*10) / 10,
			Grade:        scoreToGrade(avgScore),
			Detail:       totalDetail,
			SampleCount:  len(entries),
			LabCount:     len(labs),
			Issues:       totalIssues,
		})
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].FluencyScore > models[j].FluencyScore
	})
	var sumScore float64
	for _, m := range models {
		sumScore += m.FluencyScore
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
	b.WriteString("Language Fluency and Naturalness Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")
	if len(r.Models) == 0 {
		b.WriteString("  No fluency data available.\n")
		return b.String()
	}
	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)
	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.FluencyScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "naturalness", m.Detail.Naturalness)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "grammar_score", m.Detail.GrammarScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "flow_score", m.Detail.FlowScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "vocab_range", m.Detail.VocabRange)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "readability", m.Detail.Readability)
		fmt.Fprintf(&b, "    %-28s %d\n", "fluent_count", m.Detail.FluentCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "awkward_count", m.Detail.AwkwardCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "filler_count", m.Detail.FillerCount)
		fmt.Fprintf(&b, "    Issues: %d\n", m.Issues)
		b.WriteString("\n")
	}
	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
