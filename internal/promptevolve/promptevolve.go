package promptevolve

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type PromptVersion struct {
	Name         string  `json:"name"`
	Version      int     `json:"version"`
	Hash         string  `json:"hash"`
	FindingCount int     `json:"finding_count"`
	AvgSeverity  float64 `json:"avg_severity"`
}

type Evolution struct {
	Name            string          `json:"name"`
	Versions        []PromptVersion `json:"versions"`
	ImprovementRate float64         `json:"improvement_rate"`
	TrendDirection  string          `json:"trend_direction"`
}

type EvolutionSummary struct {
	TotalPrompts  int     `json:"total_prompts"`
	TotalVersions int     `json:"total_versions"`
	ImprovedCount int     `json:"improved_count"`
	DegradedCount int     `json:"degraded_count"`
	StableCount   int     `json:"stable_count"`
	AvgImprovement float64 `json:"avg_improvement_rate"`
}

type EvolutionReport struct {
	Evolutions []Evolution      `json:"evolutions"`
	Summary    EvolutionSummary `json:"summary"`
}

func severityScore(sev model.Severity) float64 {
	switch sev {
	case model.SevCritical:
		return 5.0
	case model.SevHigh:
		return 4.0
	case model.SevMedium:
		return 3.0
	case model.SevLow:
		return 2.0
	case model.SevInfo:
		return 1.0
	default:
		return 0.0
	}
}

func hashSummary(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:8])
}

func Analyze(findings []model.Finding) EvolutionReport {
	type bucket struct {
		count    int
		sevTotal float64
	}
	byLab := make(map[string]map[string]*bucket)

	for _, f := range findings {
		lab := f.Lab
		if lab == "" {
			lab = "default"
		}
		runID := f.RunID
		if runID == "" {
			runID = hashSummary(f.Summary)
		}
		if byLab[lab] == nil {
			byLab[lab] = make(map[string]*bucket)
		}
		b, ok := byLab[lab][runID]
		if !ok {
			b = &bucket{}
			byLab[lab][runID] = b
		}
		b.count++
		b.sevTotal += severityScore(f.Severity)
	}

	var evolutions []Evolution
	totalVersions := 0

	for lab, runs := range byLab {
		var versions []PromptVersion
		idx := 0
		for runID, b := range runs {
			avg := 0.0
			if b.count > 0 {
				avg = b.sevTotal / float64(b.count)
			}
			versions = append(versions, PromptVersion{
				Name:         lab,
				Version:      idx + 1,
				Hash:         runID,
				FindingCount: b.count,
				AvgSeverity:  avg,
			})
			idx++
		}

		sort.Slice(versions, func(i, j int) bool {
			return versions[i].Version < versions[j].Version
		})

		improvementRate := 0.0
		direction := "stable"
		if len(versions) >= 2 {
			first := versions[0]
			last := versions[len(versions)-1]
			if first.FindingCount > 0 {
				improvementRate = float64(last.FindingCount-first.FindingCount) / float64(first.FindingCount)
			}
			if improvementRate > 0.1 {
				direction = "improved"
			} else if improvementRate < -0.1 {
				direction = "degraded"
			}
		}

		evolutions = append(evolutions, Evolution{
			Name:            lab,
			Versions:        versions,
			ImprovementRate: improvementRate,
			TrendDirection:  direction,
		})
		totalVersions += len(versions)
	}

	sort.Slice(evolutions, func(i, j int) bool {
		return evolutions[i].ImprovementRate > evolutions[j].ImprovementRate
	})

	improved, degraded, stable := 0, 0, 0
	totalImprovement := 0.0
	for _, e := range evolutions {
		switch e.TrendDirection {
		case "improved":
			improved++
		case "degraded":
			degraded++
		default:
			stable++
		}
		totalImprovement += e.ImprovementRate
	}

	avgImprovement := 0.0
	if len(evolutions) > 0 {
		avgImprovement = totalImprovement / float64(len(evolutions))
	}

	return EvolutionReport{
		Evolutions: evolutions,
		Summary: EvolutionSummary{
			TotalPrompts:   len(evolutions),
			TotalVersions:  totalVersions,
			ImprovedCount:  improved,
			DegradedCount:  degraded,
			StableCount:    stable,
			AvgImprovement: avgImprovement,
		},
	}
}

func FormatText(r EvolutionReport) string {
	var sb strings.Builder
	sb.WriteString("Prompt Evolution Tracker\n")
	sb.WriteString(strings.Repeat("=", 50) + "\n\n")
	fmt.Fprintf(&sb, "  Prompts:     %d\n", r.Summary.TotalPrompts)
	fmt.Fprintf(&sb, "  Versions:    %d\n", r.Summary.TotalVersions)
	fmt.Fprintf(&sb, "  Improved:    %d\n", r.Summary.ImprovedCount)
	fmt.Fprintf(&sb, "  Degraded:    %d\n", r.Summary.DegradedCount)
	fmt.Fprintf(&sb, "  Stable:      %d\n\n", r.Summary.StableCount)

	for _, e := range r.Evolutions {
		icon := "="
		switch e.TrendDirection {
		case "improved":
			icon = "+"
		case "degraded":
			icon = "-"
		}
		fmt.Fprintf(&sb, "  [%s] %s (%.1f%% change)\n", icon, e.Name, e.ImprovementRate*100)
		for _, v := range e.Versions {
			fmt.Fprintf(&sb, "    v%d: %d findings, avg severity %.1f\n", v.Version, v.FindingCount, v.AvgSeverity)
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func FormatJSON(r EvolutionReport) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
