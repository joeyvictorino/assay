package persona

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type PersonaProfile struct {
	Model      string  `json:"model"`
	TotalRuns  int     `json:"total_runs"`
	ClassDist  map[model.Class]int `json:"class_distribution"`
	SevDist    map[model.Severity]int `json:"severity_distribution"`
	AvgScore   float64 `json:"avg_severity_score"`
	RefusalPct float64 `json:"refusal_pct"`
	TopClasses []string `json:"top_classes"`
}

type Drift struct {
	ModelA     string  `json:"model_a"`
	ModelB     string  `json:"model_b"`
	ClassJaccard   float64 `json:"class_jaccard"`
	SevCorrelation float64 `json:"severity_correlation"`
	PersonaScore   float64 `json:"persona_score"`
	Divergences    []string `json:"divergences,omitempty"`
}

type Report struct {
	Profiles []PersonaProfile `json:"profiles"`
	Drifts   []Drift          `json:"drifts,omitempty"`
	OverallConsistency float64 `json:"overall_consistency"`
}

func Analyze(findings []model.Finding) Report {
	byModel := make(map[string][]model.Finding)
	for _, f := range findings {
		byModel[f.Model.Model] = append(byModel[f.Model.Model], f)
	}

	var profiles []PersonaProfile
	for m, fs := range byModel {
		p := buildProfile(m, fs)
		profiles = append(profiles, p)
	}
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Model < profiles[j].Model
	})

	var drifts []Drift
	for i := 0; i < len(profiles); i++ {
		for j := i + 1; j < len(profiles); j++ {
			d := comparePair(profiles[i], profiles[j])
			drifts = append(drifts, d)
		}
	}
	sort.Slice(drifts, func(i, j int) bool {
		return drifts[i].PersonaScore < drifts[j].PersonaScore
	})

	overall := 0.0
	if len(drifts) > 0 {
		sum := 0.0
		for _, d := range drifts {
			sum += d.PersonaScore
		}
		overall = sum / float64(len(drifts))
	} else if len(profiles) == 1 {
		overall = 1.0
	}

	return Report{
		Profiles:           profiles,
		Drifts:             drifts,
		OverallConsistency: round2(overall),
	}
}

func buildProfile(modelName string, findings []model.Finding) PersonaProfile {
	classDist := make(map[model.Class]int)
	sevDist := make(map[model.Severity]int)
	totalScore := 0.0
	refusals := 0

	for _, f := range findings {
		classDist[f.Class]++
		sevDist[f.Severity]++
		totalScore += sevScore(f.Severity)
		if f.State == model.StateDeclined {
			refusals++
		}
	}

	avgScore := 0.0
	if len(findings) > 0 {
		avgScore = totalScore / float64(len(findings))
	}

	refusalPct := 0.0
	if len(findings) > 0 {
		refusalPct = float64(refusals) / float64(len(findings)) * 100
	}

	type classCount struct {
		class model.Class
		count int
	}
	var ranked []classCount
	for c, n := range classDist {
		ranked = append(ranked, classCount{c, n})
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].count > ranked[j].count
	})
	topN := 5
	if len(ranked) < topN {
		topN = len(ranked)
	}
	topClasses := make([]string, topN)
	for i := 0; i < topN; i++ {
		topClasses[i] = string(ranked[i].class)
	}

	return PersonaProfile{
		Model:      modelName,
		TotalRuns:  len(findings),
		ClassDist:  classDist,
		SevDist:    sevDist,
		AvgScore:   round2(avgScore),
		RefusalPct: round2(refusalPct),
		TopClasses: topClasses,
	}
}

func comparePair(a, b PersonaProfile) Drift {
	classJ := jaccard(classKeys(a.ClassDist), classKeys(b.ClassDist))

	sevCorr := sevCorrelation(a.SevDist, b.SevDist)

	persona := (classJ + sevCorr) / 2.0

	var divs []string
	if classJ < 0.3 {
		divs = append(divs, fmt.Sprintf("low class overlap (jaccard=%.2f)", classJ))
	}
	if sevCorr < 0.3 {
		divs = append(divs, fmt.Sprintf("low severity correlation (%.2f)", sevCorr))
	}
	if math.Abs(a.RefusalPct-b.RefusalPct) > 20 {
		divs = append(divs, fmt.Sprintf("refusal rate gap (%.1f%% vs %.1f%%)", a.RefusalPct, b.RefusalPct))
	}

	return Drift{
		ModelA:         a.Model,
		ModelB:         b.Model,
		ClassJaccard:   round2(classJ),
		SevCorrelation: round2(sevCorr),
		PersonaScore:   round2(persona),
		Divergences:    divs,
	}
}

func classKeys(m map[model.Class]int) map[string]bool {
	out := make(map[string]bool, len(m))
	for k := range m {
		out[string(k)] = true
	}
	return out
}

func jaccard(a, b map[string]bool) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	inter := 0
	union := make(map[string]bool)
	for k := range a {
		union[k] = true
		if b[k] {
			inter++
		}
	}
	for k := range b {
		union[k] = true
	}
	if len(union) == 0 {
		return 1.0
	}
	return float64(inter) / float64(len(union))
}

func sevCorrelation(a, b map[model.Severity]int) float64 {
	sevs := []model.Severity{model.SevInfo, model.SevLow, model.SevMedium, model.SevHigh, model.SevCritical}
	va := make([]float64, len(sevs))
	vb := make([]float64, len(sevs))
	for i, s := range sevs {
		va[i] = float64(a[s])
		vb[i] = float64(b[s])
	}
	return cosine(va, vb)
}

func cosine(a, b []float64) float64 {
	dot := 0.0
	na := 0.0
	nb := 0.0
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func sevScore(s model.Severity) float64 {
	switch s {
	case model.SevCritical:
		return 1.0
	case model.SevHigh:
		return 0.8
	case model.SevMedium:
		return 0.6
	case model.SevLow:
		return 0.4
	default:
		return 0.2
	}
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Model Persona Consistency Analysis\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")

	if len(r.Profiles) == 0 {
		b.WriteString("  No model profiles to analyze.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models analyzed:          %d\n", len(r.Profiles))
	fmt.Fprintf(&b, "  Pairwise comparisons:     %d\n", len(r.Drifts))
	fmt.Fprintf(&b, "  Overall consistency:      %.0f%%\n\n", r.OverallConsistency*100)

	b.WriteString("Profiles\n")
	b.WriteString(strings.Repeat("-", 62) + "\n")
	for _, p := range r.Profiles {
		fmt.Fprintf(&b, "  %s\n", p.Model)
		fmt.Fprintf(&b, "    Findings: %d  Avg severity: %.2f  Refusal: %.1f%%\n",
			p.TotalRuns, p.AvgScore, p.RefusalPct)
		if len(p.TopClasses) > 0 {
			fmt.Fprintf(&b, "    Top classes: %s\n", strings.Join(p.TopClasses, ", "))
		}
		b.WriteString("\n")
	}

	if len(r.Drifts) > 0 {
		b.WriteString("Pairwise Drift\n")
		b.WriteString(strings.Repeat("-", 62) + "\n")
		for _, d := range r.Drifts {
			fmt.Fprintf(&b, "  %s vs %s\n", d.ModelA, d.ModelB)
			fmt.Fprintf(&b, "    Class overlap: %.0f%%  Severity corr: %.0f%%  Persona: %.0f%%\n",
				d.ClassJaccard*100, d.SevCorrelation*100, d.PersonaScore*100)
			if len(d.Divergences) > 0 {
				for _, dv := range d.Divergences {
					fmt.Fprintf(&b, "    ! %s\n", dv)
				}
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}

func FormatJSON(r Report) string {
	b, _ := json.MarshalIndent(r, "", "  ")
	return string(b) + "\n"
}
