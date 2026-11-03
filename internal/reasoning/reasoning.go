package reasoning

import (
	"fmt"
	"math"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type Score struct {
	FindingID                string  `json:"finding_id"`
	ChainCompleteness        float64 `json:"chain_completeness"`
	Specificity              float64 `json:"specificity"`
	CausalClarity            float64 `json:"causal_clarity"`
	CounterargumentAwareness float64 `json:"counterargument_awareness"`
	Actionability            float64 `json:"actionability"`
	OverallQuality           float64 `json:"overall_quality"`
	Grade                    string  `json:"grade"`
}

type Report struct {
	Scores  []Score `json:"scores"`
	Summary Summary `json:"summary"`
}

type Summary struct {
	TotalFindings     int                `json:"total_findings"`
	GradeDistribution map[string]int     `json:"grade_distribution"`
	MeanQuality       float64            `json:"mean_quality"`
	MinQuality        float64            `json:"min_quality"`
	ByIndicator       map[string]float64 `json:"by_indicator"`
	PassRate          float64            `json:"pass_rate"`
}

const (
	weightChain   = 0.25
	weightSpec    = 0.25
	weightCausal  = 0.20
	weightCounter = 0.15
	weightAction  = 0.15
)

func Analyze(rr model.RunReport, findings []model.Finding, threshold float64) Report {
	if len(findings) == 0 {
		return Report{
			Summary: Summary{
				GradeDistribution: map[string]int{},
				ByIndicator:       map[string]float64{},
			},
		}
	}

	scores := make([]Score, len(findings))
	for i, f := range findings {
		chain := scoreChainCompleteness(f)
		spec := scoreSpecificity(f)
		causal := scoreCausalClarity(f)
		counter := scoreCounterargument(f)
		action := scoreActionability(f)

		overall := weightChain*chain +
			weightSpec*spec +
			weightCausal*causal +
			weightCounter*counter +
			weightAction*action
		overall = math.Round(overall*100) / 100

		scores[i] = Score{
			FindingID:                f.ID,
			ChainCompleteness:        chain,
			Specificity:              spec,
			CausalClarity:            causal,
			CounterargumentAwareness: counter,
			Actionability:            action,
			OverallQuality:           overall,
			Grade:                    gradeFromQuality(overall),
		}
	}

	var sum, minQ float64
	minQ = 2.0
	grades := map[string]int{}
	sumChain, sumSpec, sumCausal, sumCounter, sumAction := 0.0, 0.0, 0.0, 0.0, 0.0
	passing := 0

	for _, s := range scores {
		sum += s.OverallQuality
		if s.OverallQuality < minQ {
			minQ = s.OverallQuality
		}
		grades[s.Grade]++
		sumChain += s.ChainCompleteness
		sumSpec += s.Specificity
		sumCausal += s.CausalClarity
		sumCounter += s.CounterargumentAwareness
		sumAction += s.Actionability
		if s.OverallQuality >= threshold {
			passing++
		}
	}

	n := float64(len(scores))
	return Report{
		Scores: scores,
		Summary: Summary{
			TotalFindings:     len(scores),
			GradeDistribution: grades,
			MeanQuality:       math.Round(sum/n*100) / 100,
			MinQuality:        minQ,
			ByIndicator: map[string]float64{
				"chain_completeness":        math.Round(sumChain/n*100) / 100,
				"specificity":               math.Round(sumSpec/n*100) / 100,
				"causal_clarity":            math.Round(sumCausal/n*100) / 100,
				"counterargument_awareness": math.Round(sumCounter/n*100) / 100,
				"actionability":             math.Round(sumAction/n*100) / 100,
			},
			PassRate: math.Round(float64(passing)/n*100) / 100,
		},
	}
}

func scoreChainCompleteness(f model.Finding) float64 {
	kinds := map[string]bool{}
	for _, e := range f.Evidence {
		if e.Kind != "" {
			kinds[e.Kind] = true
		}
	}
	count := len(kinds)
	if count >= 3 {
		return 1.0
	}
	if count == 2 {
		return 0.7
	}
	if count == 1 {
		return 0.4
	}
	return 0.1
}

func scoreSpecificity(f model.Finding) float64 {
	concrete := 0
	if f.Location.Method != "" {
		concrete++
	}
	if f.Location.PathTemplate != "" {
		concrete++
	}
	if f.Location.Param != "" {
		concrete++
	}
	if f.Location.Component != "" {
		concrete++
	}
	for _, e := range f.Evidence {
		if e.StatusCode != 0 {
			concrete++
		}
		if e.Marker != "" {
			concrete++
		}
	}
	if concrete >= 4 {
		return 1.0
	}
	if concrete >= 2 {
		return 0.7
	}
	if concrete >= 1 {
		return 0.4
	}
	return 0.1
}

func scoreCausalClarity(f model.Finding) float64 {
	lower := strings.ToLower(f.Summary)
	causalTerms := []string{"because", "leads to", "allows", "results in", "enables", "causes", "permits", "exposes"}
	count := 0
	for _, term := range causalTerms {
		if strings.Contains(lower, term) {
			count++
		}
	}
	if count >= 2 {
		return 1.0
	}
	if count == 1 {
		return 0.5
	}
	return 0.1
}

func scoreCounterargument(f model.Finding) float64 {
	lower := strings.ToLower(f.Summary)
	hedges := []string{"however", "although", "may", "in some cases", "if ", "potentially", "depending", "unless"}
	for _, h := range hedges {
		if strings.Contains(lower, h) {
			return 0.8
		}
	}
	return 0.3
}

func scoreActionability(f model.Finding) float64 {
	lower := strings.ToLower(f.Summary)
	actionTerms := []string{"fix", "update", "remove", "add", "change", "ensure", "validate", "restrict", "sanitize", "escape", "encode", "patch", "upgrade", "disable"}
	count := 0
	for _, term := range actionTerms {
		if strings.Contains(lower, term) {
			count++
		}
	}
	if count >= 2 {
		return 1.0
	}
	if count == 1 {
		return 0.6
	}
	return 0.2
}

func gradeFromQuality(q float64) string {
	switch {
	case q >= 0.8:
		return "A"
	case q >= 0.65:
		return "B"
	case q >= 0.5:
		return "C"
	case q >= 0.35:
		return "D"
	default:
		return "F"
	}
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Reasoning Quality Analysis\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")

	s := r.Summary
	fmt.Fprintf(&b, "  Total findings:     %d\n", s.TotalFindings)
	fmt.Fprintf(&b, "  Mean quality:       %.2f\n", s.MeanQuality)
	fmt.Fprintf(&b, "  Min quality:        %.2f\n", s.MinQuality)
	fmt.Fprintf(&b, "  Pass rate:          %.1f%% (threshold %.2f)\n", s.PassRate*100, s.PassRate)
	b.WriteString("\n")

	b.WriteString("  Grade Distribution:\n")
	for _, g := range []string{"A", "B", "C", "D", "F"} {
		fmt.Fprintf(&b, "    %s: %d", g, s.GradeDistribution[g])
	}
	b.WriteString("\n\n")

	b.WriteString("  Indicator Averages:\n")
	for _, key := range []string{"chain_completeness", "specificity", "causal_clarity", "counterargument_awareness", "actionability"} {
		label := reasoningLabel(key)
		fmt.Fprintf(&b, "    %-30s%.2f\n", label+":", s.ByIndicator[key])
	}
	b.WriteString("\n")

	for _, sc := range r.Scores {
		fmt.Fprintf(&b, "  [%s] %s (quality: %.2f)\n", sc.Grade, sc.FindingID, sc.OverallQuality)
		fmt.Fprintf(&b, "    Chain: %.2f  Specific: %.2f  Causal: %.2f\n",
			sc.ChainCompleteness, sc.Specificity, sc.CausalClarity)
		fmt.Fprintf(&b, "    Counter: %.2f  Action: %.2f\n",
			sc.CounterargumentAwareness, sc.Actionability)
		b.WriteString("\n")
	}

	return b.String()
}

func reasoningLabel(key string) string {
	switch key {
	case "chain_completeness":
		return "Chain completeness"
	case "specificity":
		return "Specificity"
	case "causal_clarity":
		return "Causal clarity"
	case "counterargument_awareness":
		return "Counterargument awareness"
	case "actionability":
		return "Actionability"
	default:
		return key
	}
}
