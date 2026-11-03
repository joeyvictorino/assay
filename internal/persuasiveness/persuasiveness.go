package persuasiveness

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type PersuasivenessDetail struct {
	ArgumentStrength  float64 `json:"argument_strength"`
	EvidenceUse       float64 `json:"evidence_use"`
	LogicalFlow       float64 `json:"logical_flow"`
	RhetoricalSkill   float64 `json:"rhetorical_skill"`
	CounterHandling   float64 `json:"counter_handling"`
	PersuasiveCount   int     `json:"persuasive_count"`
	WeakCount         int     `json:"weak_count"`
	HedgingCount      int     `json:"hedging_count"`
}

type ModelPersuasiveness struct {
	Model              string               `json:"model"`
	PersuasivenessScore float64             `json:"persuasiveness_score"`
	Grade              string               `json:"grade"`
	Detail             PersuasivenessDetail `json:"detail"`
	SampleCount        int                  `json:"sample_count"`
	LabCount           int                  `json:"lab_count"`
	Concerns           int                  `json:"concerns"`
}

type Report struct {
	Models     []ModelPersuasiveness `json:"models"`
	BestModel  string                `json:"best_model"`
	WorstModel string                `json:"worst_model"`
	MeanScore  float64               `json:"mean_score"`
}

var argumentPhrases = []string{
	"because", "therefore", "consequently",
	"as a result", "this demonstrates", "this proves",
	"the evidence shows", "research indicates", "studies show",
	"it follows that", "this means", "clearly",
	"the data suggests", "this implies", "given that",
}

var evidencePhrases = []string{
	"according to", "based on", "as shown by",
	"demonstrated by", "supported by", "confirmed by",
	"evidenced by", "as documented", "per the findings",
	"the results indicate", "data from", "analysis reveals",
	"experiments show", "metrics demonstrate", "benchmarks confirm",
}

var logicPhrases = []string{
	"first", "second", "third",
	"in addition", "furthermore", "moreover",
	"on one hand", "on the other hand", "in contrast",
	"building on this", "it follows", "to summarize",
	"in conclusion", "the key point", "most importantly",
}

var rhetoricalPhrases = []string{
	"consider the impact", "imagine if", "what would happen",
	"the question is", "let us examine", "it is worth noting",
	"significantly", "crucially", "fundamentally",
	"the compelling reason", "a strong case", "undeniably",
	"the critical factor", "the decisive evidence", "beyond doubt",
}

var counterPhrases = []string{
	"although some argue", "despite the objection",
	"while it may seem", "critics might say",
	"one could argue", "the counterargument",
	"addressing the concern", "in response to",
	"this objection overlooks", "however this fails to account",
	"while valid", "acknowledging this limitation",
}

var weakPhrases = []string{
	"i think", "i believe", "i guess",
	"it might be", "it could be", "perhaps",
	"not sure but", "this is just my opinion",
	"take this with a grain", "i could be wrong",
	"this is debatable", "results may vary",
}

var hedgingPhrases = []string{
	"sort of", "kind of", "more or less",
	"to some extent", "in a way", "arguably",
	"seemingly", "apparently", "ostensibly",
	"it appears", "it seems", "one might think",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, PersuasivenessDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	arguments := countPhrases(combined, argumentPhrases)
	evidenceUse := countPhrases(combined, evidencePhrases)
	logic := countPhrases(combined, logicPhrases)
	rhetorical := countPhrases(combined, rhetoricalPhrases)
	counter := countPhrases(combined, counterPhrases)
	weak := countPhrases(combined, weakPhrases)
	hedging := countPhrases(combined, hedgingPhrases)

	persuasive := arguments + evidenceUse + logic + rhetorical + counter

	argScore := 0.2
	if words > 0 {
		argScore = math.Min(1.0, float64(arguments)*2.0/float64(words)*10.0+0.2)
	}

	evScore := 0.2
	if words > 0 {
		evScore = math.Min(1.0, float64(evidenceUse)*2.0/float64(words)*10.0+0.2)
	}

	logicScore := 0.3
	if logic > 0 || rhetorical > 0 {
		logicScore = math.Min(1.0, float64(logic+rhetorical)*0.15+0.3)
	}

	rhetScore := 0.2
	if words > 0 {
		rhetScore = math.Min(1.0, float64(rhetorical)*2.0/float64(words)*10.0+0.2)
	}

	counterScore := 0.2
	if counter > 0 {
		counterScore = math.Min(1.0, float64(counter)*0.2+0.3)
	}

	detail := PersuasivenessDetail{
		ArgumentStrength: math.Round(argScore*100) / 100,
		EvidenceUse:      math.Round(evScore*100) / 100,
		LogicalFlow:      math.Round(logicScore*100) / 100,
		RhetoricalSkill:  math.Round(rhetScore*100) / 100,
		CounterHandling:  math.Round(counterScore*100) / 100,
		PersuasiveCount:  persuasive,
		WeakCount:        weak,
		HedgingCount:     hedging,
	}

	score := 30.0
	score += argScore * 15.0
	score += evScore * 15.0
	score += logicScore * 10.0
	score += rhetScore * 10.0
	score += counterScore * 10.0

	score -= float64(weak) * 4.0
	score -= float64(hedging) * 3.0

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

	var models []ModelPersuasiveness
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail PersuasivenessDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			ev := evidenceText(e)
			score, detail := scoreResponse(e.Summary, ev)
			totalScore += score
			totalDetail.ArgumentStrength += detail.ArgumentStrength
			totalDetail.EvidenceUse += detail.EvidenceUse
			totalDetail.LogicalFlow += detail.LogicalFlow
			totalDetail.RhetoricalSkill += detail.RhetoricalSkill
			totalDetail.CounterHandling += detail.CounterHandling
			totalDetail.PersuasiveCount += detail.PersuasiveCount
			totalDetail.WeakCount += detail.WeakCount
			totalDetail.HedgingCount += detail.HedgingCount

			if detail.WeakCount > 3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.ArgumentStrength = math.Round(totalDetail.ArgumentStrength/n*100) / 100
		totalDetail.EvidenceUse = math.Round(totalDetail.EvidenceUse/n*100) / 100
		totalDetail.LogicalFlow = math.Round(totalDetail.LogicalFlow/n*100) / 100
		totalDetail.RhetoricalSkill = math.Round(totalDetail.RhetoricalSkill/n*100) / 100
		totalDetail.CounterHandling = math.Round(totalDetail.CounterHandling/n*100) / 100

		models = append(models, ModelPersuasiveness{
			Model:               mn,
			PersuasivenessScore: math.Round(avgScore*10) / 10,
			Grade:               scoreToGrade(avgScore),
			Detail:              totalDetail,
			SampleCount:         len(entries),
			LabCount:            len(labs),
			Concerns:            totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].PersuasivenessScore > models[j].PersuasivenessScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.PersuasivenessScore
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
	b.WriteString("Response Persuasiveness Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No persuasiveness data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.PersuasivenessScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "argument_strength", m.Detail.ArgumentStrength)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "evidence_use", m.Detail.EvidenceUse)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "logical_flow", m.Detail.LogicalFlow)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "rhetorical_skill", m.Detail.RhetoricalSkill)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "counter_handling", m.Detail.CounterHandling)
		fmt.Fprintf(&b, "    %-28s %d\n", "persuasive_count", m.Detail.PersuasiveCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "weak_count", m.Detail.WeakCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "hedging_count", m.Detail.HedgingCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
