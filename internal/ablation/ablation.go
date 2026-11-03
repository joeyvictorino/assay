package ablation

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type Component string

const (
	CompSystemPrompt Component = "system-prompt"
	CompExamples     Component = "examples"
	CompConstraints  Component = "constraints"
	CompFormatting   Component = "formatting"
	CompContext      Component = "context"
	CompFull         Component = "full"
)

var AllComponents = []Component{
	CompSystemPrompt, CompExamples, CompConstraints,
	CompFormatting, CompContext,
}

type SeverityDelta struct {
	Severity string `json:"severity"`
	Full     int    `json:"full"`
	Ablated  int    `json:"ablated"`
	Delta    int    `json:"delta"`
}

type ComponentResult struct {
	Component      Component       `json:"component"`
	FullFindings   int             `json:"full_findings"`
	AblatedFindings int            `json:"ablated_findings"`
	FindingDelta   int             `json:"finding_delta"`
	DeltaPercent   float64         `json:"delta_percent"`
	SeverityShift  []SeverityDelta `json:"severity_shift"`
	LostFindings   []string        `json:"lost_findings"`
	NewFindings    []string        `json:"new_findings"`
	ImpactScore    float64         `json:"impact_score"`
}

type Report struct {
	Components []ComponentResult `json:"components"`
	MostImpact Component         `json:"most_impact"`
	LeastImpact Component        `json:"least_impact"`
	TotalFull  int               `json:"total_full"`
}

func Analyze(full []model.Finding, ablated map[Component][]model.Finding) Report {
	fullByDedup := indexByDedup(full)
	fullSev := countBySeverity(full)

	var results []ComponentResult
	for _, comp := range AllComponents {
		af, ok := ablated[comp]
		if !ok {
			continue
		}
		ablatedByDedup := indexByDedup(af)
		ablatedSev := countBySeverity(af)

		var lost, gained []string
		for key := range fullByDedup {
			if _, exists := ablatedByDedup[key]; !exists {
				lost = append(lost, key)
			}
		}
		sort.Strings(lost)
		for key := range ablatedByDedup {
			if _, exists := fullByDedup[key]; !exists {
				gained = append(gained, key)
			}
		}
		sort.Strings(gained)

		delta := len(af) - len(full)
		var pct float64
		if len(full) > 0 {
			pct = float64(delta) / float64(len(full)) * 100
		}

		var shifts []SeverityDelta
		allSevs := mergeSevKeys(fullSev, ablatedSev)
		for _, sev := range allSevs {
			fc := fullSev[sev]
			ac := ablatedSev[sev]
			if fc != ac {
				shifts = append(shifts, SeverityDelta{
					Severity: sev, Full: fc, Ablated: ac, Delta: ac - fc,
				})
			}
		}

		impact := computeImpact(len(full), len(af), lost, gained)

		results = append(results, ComponentResult{
			Component:       comp,
			FullFindings:    len(full),
			AblatedFindings: len(af),
			FindingDelta:    delta,
			DeltaPercent:    pct,
			SeverityShift:   shifts,
			LostFindings:    lost,
			NewFindings:     gained,
			ImpactScore:     impact,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].ImpactScore > results[j].ImpactScore
	})

	var most, least Component
	if len(results) > 0 {
		most = results[0].Component
		least = results[len(results)-1].Component
	}

	return Report{
		Components:  results,
		MostImpact:  most,
		LeastImpact: least,
		TotalFull:   len(full),
	}
}

func computeImpact(fullCount, ablatedCount int, lost, gained []string) float64 {
	if fullCount == 0 && ablatedCount == 0 {
		return 0
	}
	lostWeight := float64(len(lost)) * 2.0
	gainedWeight := float64(len(gained)) * 1.0
	countDiff := abs(ablatedCount - fullCount)
	denom := float64(max(fullCount, 1))
	return (lostWeight + gainedWeight + float64(countDiff)) / denom * 100
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func indexByDedup(findings []model.Finding) map[string]model.Finding {
	m := make(map[string]model.Finding, len(findings))
	for _, f := range findings {
		key := f.DedupKey
		if key == "" {
			key = f.ID
		}
		m[key] = f
	}
	return m
}

func countBySeverity(findings []model.Finding) map[string]int {
	m := make(map[string]int)
	for _, f := range findings {
		m[string(f.Severity)]++
	}
	return m
}

func mergeSevKeys(a, b map[string]int) []string {
	seen := make(map[string]bool)
	for k := range a {
		seen[k] = true
	}
	for k := range b {
		seen[k] = true
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Prompt Ablation Analysis\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")
	fmt.Fprintf(&b, "  Full prompt findings: %d\n\n", r.TotalFull)

	if len(r.Components) == 0 {
		b.WriteString("  No ablation data provided.\n")
		return b.String()
	}

	for _, c := range r.Components {
		fmt.Fprintf(&b, "  Component: %s\n", c.Component)
		fmt.Fprintf(&b, "    Findings: %d -> %d (delta: %+d, %.1f%%)\n",
			c.FullFindings, c.AblatedFindings, c.FindingDelta, c.DeltaPercent)
		fmt.Fprintf(&b, "    Impact score: %.1f\n", c.ImpactScore)
		if len(c.LostFindings) > 0 {
			fmt.Fprintf(&b, "    Lost findings: %s\n", strings.Join(c.LostFindings, ", "))
		}
		if len(c.NewFindings) > 0 {
			fmt.Fprintf(&b, "    New findings: %s\n", strings.Join(c.NewFindings, ", "))
		}
		if len(c.SeverityShift) > 0 {
			b.WriteString("    Severity shifts:\n")
			for _, s := range c.SeverityShift {
				fmt.Fprintf(&b, "      %s: %d -> %d (%+d)\n", s.Severity, s.Full, s.Ablated, s.Delta)
			}
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "  Most impactful component:  %s\n", r.MostImpact)
	fmt.Fprintf(&b, "  Least impactful component: %s\n", r.LeastImpact)

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
