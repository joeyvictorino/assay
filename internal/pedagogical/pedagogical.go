package pedagogical

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type PedagogicalDetail struct {
	ConceptClarity    float64 `json:"concept_clarity"`
	ExampleUsage      float64 `json:"example_usage"`
	ProgressionFlow   float64 `json:"progression_flow"`
	ScaffoldingDepth  float64 `json:"scaffolding_depth"`
	RetentionSupport  float64 `json:"retention_support"`
	ExplanationCount  int     `json:"explanation_count"`
	OversimplifyCount int     `json:"oversimplify_count"`
	AnalogyCount      int     `json:"analogy_count"`
}

type ModelPedagogical struct {
	Model            string            `json:"model"`
	PedagogicalScore float64           `json:"pedagogical_score"`
	Grade            string            `json:"grade"`
	Detail           PedagogicalDetail `json:"detail"`
	SampleCount      int               `json:"sample_count"`
	LabCount         int               `json:"lab_count"`
	Concerns         int               `json:"concerns"`
}

type Report struct {
	Models     []ModelPedagogical `json:"models"`
	BestModel  string             `json:"best_model"`
	WorstModel string             `json:"worst_model"`
	MeanScore  float64            `json:"mean_score"`
}

var clarityPhrases = []string{
	"in other words", "put simply", "to clarify",
	"this means", "essentially", "the key idea",
	"the main point", "to break this down", "let me explain",
	"what this means is", "the concept is",
	"to understand this", "the idea behind",
	"think of it as", "at its core",
	"the fundamental principle",
}

var examplePhrases = []string{
	"for example", "for instance", "such as",
	"consider the case", "imagine that", "suppose",
	"to illustrate", "a good example", "take for example",
	"here is an example", "as an illustration",
	"picture this", "like when", "say you have",
	"a concrete case", "in practice",
}

var progressionPhrases = []string{
	"first", "second", "third",
	"next", "then", "after that",
	"building on this", "now that we understand",
	"with this foundation", "the next step",
	"moving on to", "once you grasp",
	"before we proceed", "let us start with",
	"from here", "finally",
}

var scaffoldPhrases = []string{
	"let us start with the basics", "the prerequisite",
	"you already know", "recall that", "as mentioned",
	"building on the previous", "this is similar to",
	"an analogy would be", "think of it like",
	"compare this to", "just as",
	"in the same way", "this relates to",
	"the connection between", "this extends",
	"like a", "similar to how",
}

var retentionPhrases = []string{
	"to summarize", "in summary", "the key takeaway",
	"remember that", "the important thing",
	"to recap", "keep in mind",
	"the bottom line", "in short",
	"the essential point", "do not forget",
	"worth noting", "critically",
	"the main lesson", "above all",
	"most importantly",
}

var oversimplifyPhrases = []string{
	"it is just", "simply put it is nothing more",
	"trivially", "any idiot", "obviously just",
	"nothing complicated", "dead simple",
	"no need to think", "brainless",
	"it is only", "barely worth explaining",
	"child could do it",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, PedagogicalDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	clarity := countPhrases(combined, clarityPhrases)
	examples := countPhrases(combined, examplePhrases)
	progression := countPhrases(combined, progressionPhrases)
	scaffold := countPhrases(combined, scaffoldPhrases)
	retention := countPhrases(combined, retentionPhrases)
	oversimplify := countPhrases(combined, oversimplifyPhrases)

	clarityScore := 0.2
	if words > 0 {
		clarityScore = math.Min(1.0, float64(clarity)*2.0/float64(words)*10.0+0.2)
	}

	exampleScore := 0.2
	if words > 0 {
		exampleScore = math.Min(1.0, float64(examples)*2.0/float64(words)*10.0+0.2)
	}

	progressionScore := 0.2
	if words > 0 {
		progressionScore = math.Min(1.0, float64(progression)*2.0/float64(words)*10.0+0.2)
	}

	scaffoldScore := 0.3
	if scaffold > 0 {
		scaffoldScore = math.Min(1.0, float64(scaffold)*0.2+0.3)
	}

	retentionScore := 0.2
	if retention > 0 {
		retentionScore = math.Min(1.0, float64(retention)*0.25+0.2)
	}

	detail := PedagogicalDetail{
		ConceptClarity:    math.Round(clarityScore*100) / 100,
		ExampleUsage:      math.Round(exampleScore*100) / 100,
		ProgressionFlow:   math.Round(progressionScore*100) / 100,
		ScaffoldingDepth:  math.Round(scaffoldScore*100) / 100,
		RetentionSupport:  math.Round(retentionScore*100) / 100,
		ExplanationCount:  clarity + examples,
		OversimplifyCount: oversimplify,
		AnalogyCount:      scaffold,
	}

	score := 30.0
	score += clarityScore * 15.0
	score += exampleScore * 15.0
	score += progressionScore * 15.0
	score += scaffoldScore * 10.0
	score += retentionScore * 10.0

	score -= float64(oversimplify) * 4.0

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

	var models []ModelPedagogical
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail PedagogicalDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.ConceptClarity += detail.ConceptClarity
			totalDetail.ExampleUsage += detail.ExampleUsage
			totalDetail.ProgressionFlow += detail.ProgressionFlow
			totalDetail.ScaffoldingDepth += detail.ScaffoldingDepth
			totalDetail.RetentionSupport += detail.RetentionSupport
			totalDetail.ExplanationCount += detail.ExplanationCount
			totalDetail.OversimplifyCount += detail.OversimplifyCount
			totalDetail.AnalogyCount += detail.AnalogyCount

			if detail.OversimplifyCount > 2 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.ConceptClarity = math.Round(totalDetail.ConceptClarity/n*100) / 100
		totalDetail.ExampleUsage = math.Round(totalDetail.ExampleUsage/n*100) / 100
		totalDetail.ProgressionFlow = math.Round(totalDetail.ProgressionFlow/n*100) / 100
		totalDetail.ScaffoldingDepth = math.Round(totalDetail.ScaffoldingDepth/n*100) / 100
		totalDetail.RetentionSupport = math.Round(totalDetail.RetentionSupport/n*100) / 100

		models = append(models, ModelPedagogical{
			Model:            mn,
			PedagogicalScore: math.Round(avgScore*10) / 10,
			Grade:            scoreToGrade(avgScore),
			Detail:           totalDetail,
			SampleCount:      len(entries),
			LabCount:         len(labs),
			Concerns:         totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].PedagogicalScore > models[j].PedagogicalScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.PedagogicalScore
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
	b.WriteString("Response Pedagogical Quality Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No pedagogical data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.PedagogicalScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "concept_clarity", m.Detail.ConceptClarity)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "example_usage", m.Detail.ExampleUsage)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "progression_flow", m.Detail.ProgressionFlow)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "scaffolding_depth", m.Detail.ScaffoldingDepth)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "retention_support", m.Detail.RetentionSupport)
		fmt.Fprintf(&b, "    %-28s %d\n", "explanation_count", m.Detail.ExplanationCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "oversimplify_count", m.Detail.OversimplifyCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "analogy_count", m.Detail.AnalogyCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
