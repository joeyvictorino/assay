package analogy

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type AnalogyDetail struct {
	AnalogyCount    int     `json:"analogy_count"`
	Relevance       float64 `json:"relevance"`
	Clarity         float64 `json:"clarity"`
	Novelty         float64 `json:"novelty"`
	Accuracy        float64 `json:"accuracy"`
	BadAnalogyCount int     `json:"bad_analogy_count"`
}

type ModelAnalogy struct {
	Model        string        `json:"model"`
	AnalogyScore float64       `json:"analogy_score"`
	Grade        string        `json:"grade"`
	Detail       AnalogyDetail `json:"detail"`
	SampleCount  int           `json:"sample_count"`
	LabCount     int           `json:"lab_count"`
	Concerns     int           `json:"concerns"`
}

type Report struct {
	Models     []ModelAnalogy `json:"models"`
	BestModel  string         `json:"best_model"`
	WorstModel string         `json:"worst_model"`
	MeanScore  float64        `json:"mean_score"`
}

var analogyPhrases = []string{
	"think of it like", "similar to", "just like",
	"analogous to", "in the same way", "much like",
	"compare this to", "imagine a", "picture a",
	"it is like", "as if", "the same way that",
	"resembles", "parallels", "akin to",
	"metaphorically", "figuratively speaking",
	"to draw a parallel", "by analogy",
}

var relevancePhrases = []string{
	"this analogy helps because", "the similarity is",
	"this comparison works because", "the parallel here",
	"the connection is", "what makes this relevant",
	"this maps to", "the correspondence",
	"the relationship between", "this mirrors",
	"likewise", "correspondingly",
}

var clarityPhrases = []string{
	"to put it simply", "in simpler terms",
	"a familiar example", "something you know",
	"everyday experience", "common example",
	"most people know", "a well known",
	"relatable", "intuitive",
	"easy to understand", "straightforward comparison",
}

var noveltyPhrases = []string{
	"an unusual comparison", "a fresh perspective",
	"a creative analogy", "unconventional",
	"a unique way to think", "a new angle",
	"surprisingly similar", "unexpected parallel",
	"a novel comparison", "an original metaphor",
}

var badAnalogyPhrases = []string{
	"the analogy breaks down", "this comparison fails",
	"not a perfect analogy", "the metaphor falls apart",
	"oversimplified comparison", "misleading analogy",
	"false equivalence", "apples and oranges",
	"this is a stretch", "forced comparison",
	"weak analogy", "flawed comparison",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, AnalogyDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	analogies := countPhrases(combined, analogyPhrases)
	relevance := countPhrases(combined, relevancePhrases)
	clarity := countPhrases(combined, clarityPhrases)
	novelty := countPhrases(combined, noveltyPhrases)
	bad := countPhrases(combined, badAnalogyPhrases)

	analogyPresence := 0.1
	if words > 0 {
		analogyPresence = math.Min(1.0, float64(analogies)*2.0/float64(words)*10.0+0.1)
	}

	relevanceScore := 0.2
	if relevance > 0 {
		relevanceScore = math.Min(1.0, float64(relevance)*0.25+0.2)
	}

	clarityScore := 0.2
	if clarity > 0 {
		clarityScore = math.Min(1.0, float64(clarity)*0.25+0.2)
	}

	noveltyScore := 0.1
	if novelty > 0 {
		noveltyScore = math.Min(1.0, float64(novelty)*0.3+0.1)
	}

	accuracyScore := 1.0
	if bad > 0 {
		accuracyScore = math.Max(0.0, 1.0-float64(bad)*0.25)
	}

	detail := AnalogyDetail{
		AnalogyCount:    analogies,
		Relevance:       math.Round(relevanceScore*100) / 100,
		Clarity:         math.Round(clarityScore*100) / 100,
		Novelty:         math.Round(noveltyScore*100) / 100,
		Accuracy:        math.Round(accuracyScore*100) / 100,
		BadAnalogyCount: bad,
	}

	score := 25.0
	score += analogyPresence * 20.0
	score += relevanceScore * 15.0
	score += clarityScore * 15.0
	score += noveltyScore * 10.0
	score += accuracyScore * 15.0

	score -= float64(bad) * 5.0

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

	var models []ModelAnalogy
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail AnalogyDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.AnalogyCount += detail.AnalogyCount
			totalDetail.Relevance += detail.Relevance
			totalDetail.Clarity += detail.Clarity
			totalDetail.Novelty += detail.Novelty
			totalDetail.Accuracy += detail.Accuracy
			totalDetail.BadAnalogyCount += detail.BadAnalogyCount

			if detail.BadAnalogyCount > 2 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.Relevance = math.Round(totalDetail.Relevance/n*100) / 100
		totalDetail.Clarity = math.Round(totalDetail.Clarity/n*100) / 100
		totalDetail.Novelty = math.Round(totalDetail.Novelty/n*100) / 100
		totalDetail.Accuracy = math.Round(totalDetail.Accuracy/n*100) / 100

		models = append(models, ModelAnalogy{
			Model:        mn,
			AnalogyScore: math.Round(avgScore*10) / 10,
			Grade:        scoreToGrade(avgScore),
			Detail:       totalDetail,
			SampleCount:  len(entries),
			LabCount:     len(labs),
			Concerns:     totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].AnalogyScore > models[j].AnalogyScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.AnalogyScore
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
	b.WriteString("Analogy Quality Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No analogy data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.AnalogyScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "analogy_count", m.Detail.AnalogyCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "relevance", m.Detail.Relevance)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "clarity", m.Detail.Clarity)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "novelty", m.Detail.Novelty)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "accuracy", m.Detail.Accuracy)
		fmt.Fprintf(&b, "    %-28s %d\n", "bad_analogy_count", m.Detail.BadAnalogyCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
