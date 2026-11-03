package nuance

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type NuanceDetail struct {
	EdgeCaseHandling   float64 `json:"edge_case_handling"`
	CaveatPresence     float64 `json:"caveat_presence"`
	BalancedPerspective float64 `json:"balanced_perspective"`
	ContextSensitivity float64 `json:"context_sensitivity"`
	GrayAreaAwareness  float64 `json:"gray_area_awareness"`
	QualifierCount     int     `json:"qualifier_count"`
	AbsoluteCount      int     `json:"absolute_count"`
	HedgeCount         int     `json:"hedge_count"`
}

type ModelNuance struct {
	Model       string       `json:"model"`
	NuanceScore float64      `json:"nuance_score"`
	Grade       string       `json:"grade"`
	Detail      NuanceDetail `json:"detail"`
	SampleCount int          `json:"sample_count"`
	LabCount    int          `json:"lab_count"`
	Concerns    int          `json:"concerns"`
}

type Report struct {
	Models     []ModelNuance `json:"models"`
	BestModel  string        `json:"best_model"`
	WorstModel string        `json:"worst_model"`
	MeanScore  float64       `json:"mean_score"`
}

var edgeCasePhrases = []string{
	"edge case", "corner case", "special case",
	"exception", "in rare cases", "under certain conditions",
	"in some scenarios", "depending on the context",
	"not always", "may vary", "it depends",
	"in specific situations", "when applicable",
	"particular circumstance", "unusual scenario",
}

var caveatPhrases = []string{
	"however", "although", "on the other hand",
	"that said", "keep in mind", "it's worth noting",
	"bear in mind", "one caveat", "important to note",
	"be aware that", "with the caveat", "note that",
	"disclaimer", "limitation", "with some exceptions",
}

var balancePhrases = []string{
	"on one hand", "on the other hand", "both sides",
	"pros and cons", "advantages and disadvantages",
	"trade-off", "tradeoff", "trade off",
	"balanced view", "multiple perspectives",
	"from another angle", "alternatively",
	"counterargument", "counter-argument",
	"different viewpoint", "opposing view",
}

var contextPhrases = []string{
	"in this context", "given your situation",
	"for your use case", "considering your",
	"specific to", "in your case",
	"depending on your needs", "based on your requirements",
	"tailored to", "context-dependent",
	"situational", "case-by-case",
}

var grayAreaPhrases = []string{
	"not black and white", "gray area", "grey area",
	"nuanced", "complex issue", "multifaceted",
	"no simple answer", "it's complicated",
	"ambiguous", "debatable", "subjective",
	"open to interpretation", "matter of opinion",
}

var qualifierPhrases = []string{
	"generally", "typically", "usually", "often",
	"in most cases", "tends to", "likely",
	"probably", "arguably", "somewhat",
	"to some extent", "partially", "relatively",
	"approximately", "roughly",
}

var absolutePhrases = []string{
	"always", "never", "absolutely",
	"definitely", "without exception", "guaranteed",
	"impossible", "certainly", "undoubtedly",
	"invariably", "unquestionably", "indisputably",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, NuanceDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	edgeCases := countPhrases(combined, edgeCasePhrases)
	caveats := countPhrases(combined, caveatPhrases)
	balance := countPhrases(combined, balancePhrases)
	context := countPhrases(combined, contextPhrases)
	grayArea := countPhrases(combined, grayAreaPhrases)
	qualifiers := countPhrases(combined, qualifierPhrases)
	absolutes := countPhrases(combined, absolutePhrases)

	edgeScore := 0.2
	if words > 0 {
		edgeScore = math.Min(1.0, float64(edgeCases)*2.0/float64(words)*10.0+0.2)
	}

	caveatScore := 0.2
	if words > 0 {
		caveatScore = math.Min(1.0, float64(caveats)*2.0/float64(words)*10.0+0.2)
	}

	balanceScore := 0.2
	if words > 0 {
		balanceScore = math.Min(1.0, float64(balance)*2.5/float64(words)*10.0+0.2)
	}

	contextScore := 0.3
	if context > 0 {
		contextScore = math.Min(1.0, float64(context)*0.25+0.3)
	}

	grayScore := 0.2
	if grayArea > 0 {
		grayScore = math.Min(1.0, float64(grayArea)*0.3+0.2)
	}

	hedges := qualifiers
	detail := NuanceDetail{
		EdgeCaseHandling:    math.Round(edgeScore*100) / 100,
		CaveatPresence:      math.Round(caveatScore*100) / 100,
		BalancedPerspective: math.Round(balanceScore*100) / 100,
		ContextSensitivity:  math.Round(contextScore*100) / 100,
		GrayAreaAwareness:   math.Round(grayScore*100) / 100,
		QualifierCount:      qualifiers,
		AbsoluteCount:       absolutes,
		HedgeCount:          hedges,
	}

	score := 30.0
	score += edgeScore * 15.0
	score += caveatScore * 15.0
	score += balanceScore * 15.0
	score += contextScore * 10.0
	score += grayScore * 10.0

	score -= float64(absolutes) * 3.0
	score += float64(qualifiers) * 1.0

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

	var models []ModelNuance
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail NuanceDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.EdgeCaseHandling += detail.EdgeCaseHandling
			totalDetail.CaveatPresence += detail.CaveatPresence
			totalDetail.BalancedPerspective += detail.BalancedPerspective
			totalDetail.ContextSensitivity += detail.ContextSensitivity
			totalDetail.GrayAreaAwareness += detail.GrayAreaAwareness
			totalDetail.QualifierCount += detail.QualifierCount
			totalDetail.AbsoluteCount += detail.AbsoluteCount
			totalDetail.HedgeCount += detail.HedgeCount

			if detail.AbsoluteCount > 3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.EdgeCaseHandling = math.Round(totalDetail.EdgeCaseHandling/n*100) / 100
		totalDetail.CaveatPresence = math.Round(totalDetail.CaveatPresence/n*100) / 100
		totalDetail.BalancedPerspective = math.Round(totalDetail.BalancedPerspective/n*100) / 100
		totalDetail.ContextSensitivity = math.Round(totalDetail.ContextSensitivity/n*100) / 100
		totalDetail.GrayAreaAwareness = math.Round(totalDetail.GrayAreaAwareness/n*100) / 100

		models = append(models, ModelNuance{
			Model:       mn,
			NuanceScore: math.Round(avgScore*10) / 10,
			Grade:       scoreToGrade(avgScore),
			Detail:      totalDetail,
			SampleCount: len(entries),
			LabCount:    len(labs),
			Concerns:    totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].NuanceScore > models[j].NuanceScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.NuanceScore
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
	b.WriteString("Response Nuance Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No nuance data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.NuanceScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "edge_case_handling", m.Detail.EdgeCaseHandling)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "caveat_presence", m.Detail.CaveatPresence)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "balanced_perspective", m.Detail.BalancedPerspective)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "context_sensitivity", m.Detail.ContextSensitivity)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "gray_area_awareness", m.Detail.GrayAreaAwareness)
		fmt.Fprintf(&b, "    %-28s %d\n", "qualifier_count", m.Detail.QualifierCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "absolute_count", m.Detail.AbsoluteCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "hedge_count", m.Detail.HedgeCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
