package causality

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type CausalityDetail struct {
	CausalChainScore   float64 `json:"causal_chain_score"`
	MechanismClarity   float64 `json:"mechanism_clarity"`
	CounterfactualUse  float64 `json:"counterfactual_use"`
	TemporalOrdering   float64 `json:"temporal_ordering"`
	ConfoundAwareness  float64 `json:"confound_awareness"`
	CausalCount        int     `json:"causal_count"`
	SpeculativeCount   int     `json:"speculative_count"`
	UnsupportedCount   int     `json:"unsupported_count"`
}

type ModelCausality struct {
	Model          string          `json:"model"`
	CausalityScore float64         `json:"causality_score"`
	Grade          string          `json:"grade"`
	Detail         CausalityDetail `json:"detail"`
	SampleCount    int             `json:"sample_count"`
	LabCount       int             `json:"lab_count"`
	Concerns       int             `json:"concerns"`
}

type Report struct {
	Models     []ModelCausality `json:"models"`
	BestModel  string           `json:"best_model"`
	WorstModel string           `json:"worst_model"`
	MeanScore  float64          `json:"mean_score"`
}

var causalPhrases = []string{
	"because", "caused by", "leads to", "results in",
	"due to", "as a result", "therefore", "consequently",
	"this causes", "the root cause", "the effect of",
	"triggers", "induces", "produces", "gives rise to",
}

var mechanismPhrases = []string{
	"the mechanism is", "this works by", "the process involves",
	"step by step", "the chain of events", "specifically",
	"the underlying reason", "this happens when",
	"the pathway from", "through the following steps",
	"the causal mechanism", "here is how",
	"the sequence of", "in detail",
}

var counterfactualPhrases = []string{
	"if instead", "had this not", "without this",
	"in the absence of", "alternatively",
	"were it not for", "if we remove",
	"counterfactual", "what would happen if",
	"suppose instead", "in contrast",
	"the alternative scenario", "if not for",
}

var temporalPhrases = []string{
	"first", "then", "next", "after that",
	"before this", "subsequently", "following",
	"preceded by", "prior to", "once",
	"at that point", "in sequence",
	"the timeline shows", "chronologically",
}

var confoundPhrases = []string{
	"however", "it is important to note",
	"correlation does not imply", "confounding factor",
	"alternative explanation", "other variables",
	"we should consider", "a competing hypothesis",
	"this alone does not prove", "controlling for",
	"ruling out", "spurious correlation",
	"mediating variable", "moderating factor",
}

var speculativePhrases = []string{
	"might cause", "could potentially lead to",
	"it is possible that", "one theory is",
	"we speculate", "hypothetically",
	"it remains unclear", "the connection is uncertain",
	"no definitive link", "loosely associated",
	"anecdotal evidence suggests", "unproven",
}

var unsupportedPhrases = []string{
	"obviously causes", "clearly the reason",
	"everyone knows that", "it goes without saying",
	"undeniably", "the only explanation",
	"must be caused by", "proves that",
	"definitively shows", "irrefutable evidence",
	"there is no other possibility", "beyond question",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, CausalityDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	causal := countPhrases(combined, causalPhrases)
	mechanism := countPhrases(combined, mechanismPhrases)
	counterfactual := countPhrases(combined, counterfactualPhrases)
	temporal := countPhrases(combined, temporalPhrases)
	confound := countPhrases(combined, confoundPhrases)
	speculative := countPhrases(combined, speculativePhrases)
	unsupported := countPhrases(combined, unsupportedPhrases)

	causalTotal := causal + mechanism + counterfactual + temporal + confound

	chainScore := 0.2
	if words > 0 {
		chainScore = math.Min(1.0, float64(causal)*2.0/float64(words)*10.0+0.2)
	}

	mechanismScore := 0.2
	if words > 0 {
		mechanismScore = math.Min(1.0, float64(mechanism)*2.0/float64(words)*10.0+0.2)
	}

	cfScore := 0.3
	if counterfactual > 0 || confound > 0 {
		cfScore = math.Min(1.0, float64(counterfactual+confound)*0.15+0.3)
	}

	temporalScore := 0.2
	if words > 0 {
		temporalScore = math.Min(1.0, float64(temporal)*2.0/float64(words)*10.0+0.2)
	}

	confoundScore := 0.2
	if speculative == 0 && unsupported == 0 {
		confoundScore = 0.8
	} else if speculative+unsupported < 3 {
		confoundScore = 0.5
	}

	detail := CausalityDetail{
		CausalChainScore:  math.Round(chainScore*100) / 100,
		MechanismClarity:  math.Round(mechanismScore*100) / 100,
		CounterfactualUse: math.Round(cfScore*100) / 100,
		TemporalOrdering:  math.Round(temporalScore*100) / 100,
		ConfoundAwareness: math.Round(confoundScore*100) / 100,
		CausalCount:       causalTotal,
		SpeculativeCount:  speculative,
		UnsupportedCount:  unsupported,
	}

	score := 30.0
	score += chainScore * 15.0
	score += mechanismScore * 15.0
	score += cfScore * 10.0
	score += temporalScore * 10.0
	score += confoundScore * 10.0

	score -= float64(speculative) * 4.0
	score -= float64(unsupported) * 5.0

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

	var models []ModelCausality
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail CausalityDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.CausalChainScore += detail.CausalChainScore
			totalDetail.MechanismClarity += detail.MechanismClarity
			totalDetail.CounterfactualUse += detail.CounterfactualUse
			totalDetail.TemporalOrdering += detail.TemporalOrdering
			totalDetail.ConfoundAwareness += detail.ConfoundAwareness
			totalDetail.CausalCount += detail.CausalCount
			totalDetail.SpeculativeCount += detail.SpeculativeCount
			totalDetail.UnsupportedCount += detail.UnsupportedCount

			if detail.SpeculativeCount > 3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.CausalChainScore = math.Round(totalDetail.CausalChainScore/n*100) / 100
		totalDetail.MechanismClarity = math.Round(totalDetail.MechanismClarity/n*100) / 100
		totalDetail.CounterfactualUse = math.Round(totalDetail.CounterfactualUse/n*100) / 100
		totalDetail.TemporalOrdering = math.Round(totalDetail.TemporalOrdering/n*100) / 100
		totalDetail.ConfoundAwareness = math.Round(totalDetail.ConfoundAwareness/n*100) / 100

		models = append(models, ModelCausality{
			Model:          mn,
			CausalityScore: math.Round(avgScore*10) / 10,
			Grade:          scoreToGrade(avgScore),
			Detail:         totalDetail,
			SampleCount:    len(entries),
			LabCount:       len(labs),
			Concerns:       totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].CausalityScore > models[j].CausalityScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.CausalityScore
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
	b.WriteString("Causal Reasoning Quality Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No causality data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.CausalityScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "causal_chain_score", m.Detail.CausalChainScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "mechanism_clarity", m.Detail.MechanismClarity)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "counterfactual_use", m.Detail.CounterfactualUse)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "temporal_ordering", m.Detail.TemporalOrdering)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "confound_awareness", m.Detail.ConfoundAwareness)
		fmt.Fprintf(&b, "    %-28s %d\n", "causal_count", m.Detail.CausalCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "speculative_count", m.Detail.SpeculativeCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "unsupported_count", m.Detail.UnsupportedCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
